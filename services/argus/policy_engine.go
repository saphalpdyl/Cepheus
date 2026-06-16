package argus

import (
	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/log"
	"cepheus/services/argus/types"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Employs the policy engine for reporting events
// Gathers different metrics (instant findings from detectors) and
// reports/alarms based on leaky bucket
// https://en.wikipedia.org/wiki/Leaky_bucket

type PolicyStateStatus string

const (
	PolicyStateStatusClean      = "clean"
	PolicyStateStatusWatching   = "watching"
	PolicyStateStatusFiring     = "firing"
	PolicyStateStatusRecovering = "recovering"
)

type PendingFinding struct {
	Id      string
	Finding *types.Finding
}

type PolicyState struct {
	Status          PolicyStateStatus
	StatusUpdatedAt time.Time

	BucketState     BucketState
	OpenEventId     *uuid.UUID
	PendingFindings []*PendingFinding
}

type PolicyEngine struct {
	logger *slog.Logger
	query  *argus_db.Queries
	mu     sync.Mutex

	bucket              *LeakyBucket
	states              map[types.SeriesKey]*PolicyState
	bucketSweepInterval time.Duration

	confirmWindow time.Duration
	quietPeriod   time.Duration

	transactionGenerator func(ctx context.Context) (pgx.Tx, error)
}

type PolicyEngineConfig struct {
	Logger *slog.Logger
	Query  *argus_db.Queries

	LeakyBucketConfiguration LeakyBucketConfiguration
	LeakyBucketSweepInterval time.Duration
	QuietPeriod              time.Duration // wait time for recovering -> clean
	ConfirmWindow            time.Duration // wait time for waiting -> clean

	TransactionGenerator func(ctx context.Context) (pgx.Tx, error)
}

func NewPolicyEngine(cfg PolicyEngineConfig) (*PolicyEngine, error) {
	if cfg.Query == nil {
		return nil, errors.New("configuration missing *argus_db.Queries")
	}

	return &PolicyEngine{
		logger:               cfg.Logger,
		query:                cfg.Query,
		bucket:               NewLeakyBucket(cfg.LeakyBucketConfiguration),
		states:               map[types.SeriesKey]*PolicyState{},
		bucketSweepInterval:  cfg.LeakyBucketSweepInterval,
		confirmWindow:        cfg.ConfirmWindow,
		quietPeriod:          cfg.QuietPeriod,
		transactionGenerator: cfg.TransactionGenerator,
	}, nil
}

func (p *PolicyEngine) Start(ctx context.Context) error {
	err := p.Sweep(ctx)
	if err != nil {
		p.logger.ErrorContext(ctx, "error sweeping policies during start", log.Err(err))
		return err
	}

	go func() {
		ticker := time.NewTicker(p.bucketSweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := p.Sweep(ctx)
				if err != nil {
					p.logger.DebugContext(ctx, "error sweeping policies", log.Err(err))
					continue
				}
			}
		}
	}()

	return nil
}

// Sweep loops through all buckets
func (p *PolicyEngine) Sweep(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	policyStateRaw, err := p.query.ListNonCleanPolicyStates(ctx)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to find non clean policy states", log.Err(err))
		return err
	}

	for _, pRaw := range policyStateRaw {
		seriesKey := types.SeriesKey{
			SerialId: pRaw.SerialID,
			Target:   pRaw.Target,
			Port:     pRaw.Port,
			Metric:   pRaw.Metric,
			Detector: types.DetectorType(pRaw.Detector),
		}

		_, ok := p.states[seriesKey]
		if ok {
			continue
		}

		var openEventId *uuid.UUID
		if pRaw.OpenEventID.Valid {
			u, err := uuid.Parse(pRaw.OpenEventID.String())
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to find open event id", log.Err(err))
				continue
			}

			openEventId = &u
		}

		var pendingFindings []*PendingFinding
		for _, fd := range pRaw.PendingFindings {
			pendingFindings = append(pendingFindings, &PendingFinding{
				Id:      fd.String(),
				Finding: nil,
			})
		}

		// Hydrate states
		p.states[seriesKey] = &PolicyState{
			Status:          PolicyStateStatus(pRaw.Status),
			StatusUpdatedAt: pRaw.EnteredStatusAt.Time,
			BucketState: BucketState{
				Score:          pRaw.Score,
				ScoreUpdatedAt: pRaw.ScoreUpdatedAt.Time,
			},
			OpenEventId:     openEventId,
			PendingFindings: pendingFindings,
		}
	}

	now := time.Now()
	for sk, state := range p.states {
		updateState := p.bucket.Decay(state.BucketState, now)
		state.BucketState = updateState.NewState

		switch state.Status {
		case PolicyStateStatusWatching:
			if now.Sub(state.StatusUpdatedAt) > p.confirmWindow {
				// If it has been longer than the allowed window before reverting back to clean
				state.Status = PolicyStateStatusClean
				state.PendingFindings = make([]*PendingFinding, 0)
				state.StatusUpdatedAt = now
			}
		case PolicyStateStatusFiring:
			if updateState.CrossedClose {
				state.Status = PolicyStateStatusRecovering
				state.StatusUpdatedAt = now
			}
		case PolicyStateStatusRecovering:
			if now.Sub(state.StatusUpdatedAt) > p.quietPeriod && state.OpenEventId != nil {
				// If it has been longer than allowed window before reverting to clean
				err = p.query.CloseEvent(ctx, argus_db.CloseEventParams{
					ID:       pgtype.UUID{Bytes: *state.OpenEventId, Valid: true},
					ClosedAt: pgtype.Timestamptz{Time: now, Valid: true},
				})
				if err != nil {
					p.logger.ErrorContext(ctx, "failed to close event", log.Err(err))
					return err
				}

				state.Status = PolicyStateStatusClean
				state.OpenEventId = nil
				state.PendingFindings = make([]*PendingFinding, 0)
				state.StatusUpdatedAt = now
			}
		}

		err := p.savePolicyState(ctx, sk, state)
		if err != nil {
			continue
		}
	}

	return nil
}

func (p *PolicyEngine) InsertFinding(ctx context.Context, seriesKey types.SeriesKey, finding *types.Finding) (*pgtype.UUID, error) {
	detailsMarshaled, err := json.Marshal(finding.Details)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to marshal finding details", log.Err(err))
		return nil, err
	}

	uuidRaw, err := p.query.InsertFinding(ctx, argus_db.InsertFindingParams{
		SerialID: seriesKey.SerialId,
		Target:   seriesKey.Target,
		Port:     seriesKey.Port,
		Metric:   seriesKey.Metric,
		Detector: string(seriesKey.Detector),
		Ts: pgtype.Timestamptz{
			Time:  finding.TS,
			Valid: true,
		},
		Value:    finding.Value,
		Severity: finding.Severity,
		Details:  detailsMarshaled,
	})

	if err != nil {
		p.logger.ErrorContext(ctx, "failed to insert finding", log.Err(err))
		return nil, err
	}

	uuidValue, err := uuidRaw.UUIDValue()
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to retrieve uuid", log.Err(err))
		return nil, err
	}

	return &uuidValue, nil
}

func (p *PolicyEngine) ApplyFinding(ctx context.Context, seriesKey types.SeriesKey, finding *types.Finding, findingId pgtype.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	state, ok := p.states[seriesKey]
	if !ok {
		stateRaw, err := p.query.GetPolicyState(ctx, argus_db.GetPolicyStateParams{
			SerialID: seriesKey.SerialId,
			SrcIp:    seriesKey.SrcIP,
			Target:   seriesKey.Target,
			Port:     seriesKey.Port,
			Metric:   seriesKey.Metric,
			Detector: string(seriesKey.Detector),
		})

		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			p.logger.ErrorContext(ctx, "failed to get policy state", log.Err(err))
			return err
		}

		if errors.Is(err, pgx.ErrNoRows) {
			// Create a new empty state
			state = &PolicyState{
				Status:          PolicyStateStatusClean,
				StatusUpdatedAt: time.Now(),
				BucketState: BucketState{
					Score:          0,
					ScoreUpdatedAt: time.Time{},
				},
				OpenEventId:     nil,
				PendingFindings: make([]*PendingFinding, 0),
			}

			p.states[seriesKey] = state
		} else {
			// Convert from UUID to PendingFinding struct without the findings objects
			var pendingFindings []*PendingFinding
			for _, fd := range stateRaw.PendingFindings {
				pendingFindings = append(pendingFindings, &PendingFinding{
					Id:      fd.String(),
					Finding: nil,
				})
			}

			var openEventId *uuid.UUID
			if stateRaw.OpenEventID.Valid {
				u := uuid.UUID(stateRaw.OpenEventID.Bytes)
				openEventId = &u
			}

			state = &PolicyState{
				Status:          PolicyStateStatus(stateRaw.Status),
				StatusUpdatedAt: stateRaw.EnteredStatusAt.Time,
				BucketState: BucketState{
					Score:          stateRaw.Score,
					ScoreUpdatedAt: stateRaw.ScoreUpdatedAt.Time,
				},
				OpenEventId:     openEventId,
				PendingFindings: pendingFindings,
			}
			p.states[seriesKey] = state
		}
	}

	updateState := p.bucket.Add(state.BucketState, finding.Severity, finding.TS)
	state.BucketState.Score = updateState.NewState.Score
	state.BucketState.ScoreUpdatedAt = updateState.NewState.ScoreUpdatedAt

	// CrossedOpen and CrossedClose are mutually exclusive events
	switch state.Status {
	case PolicyStateStatusClean:
		p.logger.DebugContext(ctx, "transitioning state clean -> watching")
		state.Status = PolicyStateStatusWatching
		state.StatusUpdatedAt = finding.TS
		state.PendingFindings = []*PendingFinding{
			{
				Id:      findingId.String(),
				Finding: finding,
			},
		}

	case PolicyStateStatusWatching:
		state.PendingFindings = append(state.PendingFindings, &PendingFinding{
			Id:      findingId.String(),
			Finding: finding,
		})
		if updateState.CrossedOpen {
			// Threshold crossed; fire event
			p.logger.DebugContext(ctx, "transitioning state watching -> firing")
			tx, err := p.transactionGenerator(ctx)
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to generate transaction", log.Err(err))
				_ = tx.Rollback(ctx)
				return err
			}

			peakSeverity := finding.Severity
			for _, pf := range state.PendingFindings {
				if pf.Finding != nil && pf.Finding.Severity > peakSeverity {
					peakSeverity = pf.Finding.Severity
				}
			}

			event, err := p.query.WithTx(tx).OpenEvent(ctx, argus_db.OpenEventParams{
				SerialID:     seriesKey.SerialId,
				Target:       seriesKey.Target,
				Port:         seriesKey.Port,
				Metric:       seriesKey.Metric,
				OpenedAt:     pgtype.Timestamptz{Time: finding.TS, Valid: true},
				FindingCount: int32(len(state.PendingFindings)),
				PeakSeverity: peakSeverity,
				Detectors:    []string{string(seriesKey.Detector)},
			})
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to open event", log.Err(err))
				_ = tx.Rollback(ctx)
				return err
			}

			for _, finding := range state.PendingFindings {
				findingIdParsed, err := uuid.Parse(finding.Id)
				if err != nil {
					p.logger.ErrorContext(ctx, "failed to parse finding", log.Err(err))
					_ = tx.Rollback(ctx)
					return err
				}

				err = p.query.WithTx(tx).AttachFindingToEvent(ctx, argus_db.AttachFindingToEventParams{
					EventID: pgtype.UUID{Bytes: event.Bytes, Valid: true},
					ID:      pgtype.UUID{Bytes: findingIdParsed, Valid: true},
				})
				if err != nil {
					p.logger.ErrorContext(ctx, "failed to attach finding", log.Err(err))
					_ = tx.Rollback(ctx)
					return err
				}
			}

			uuidValue, err := uuid.Parse(event.String())
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to parse uuid", log.Err(err))
				_ = tx.Rollback(ctx)
				return err
			}

			err = tx.Commit(ctx)
			if err != nil {
				p.logger.ErrorContext(ctx, "failed to commit transaction", log.Err(err))
				return err
			}

			state.Status = PolicyStateStatusFiring
			state.OpenEventId = &uuidValue
			state.PendingFindings = make([]*PendingFinding, 0)
			state.StatusUpdatedAt = finding.TS
		}
	case PolicyStateStatusFiring:
		if state.OpenEventId == nil {
			p.logger.ErrorContext(ctx, "missing open id but policy state in firing")
			return errors.New("missing open id but policy state in firing")
		}

		err := p.attachFindingToEvent(ctx, *state.OpenEventId, findingId)
		if err != nil {
			return err
		}

	case PolicyStateStatusRecovering:
		if state.OpenEventId == nil {
			p.logger.ErrorContext(ctx, "missing open id but policy state in recovering")
			return errors.New("missing open id but policy state in recovering")
		}

		err := p.attachFindingToEvent(ctx, *state.OpenEventId, findingId)
		if err != nil {
			return err
		}

		state.Status = PolicyStateStatusFiring
		state.StatusUpdatedAt = finding.TS
	}

	err := p.savePolicyState(ctx, seriesKey, state)
	if err != nil {
		return err
	}

	return nil
}

func (p *PolicyEngine) attachFindingToEvent(ctx context.Context, eventId uuid.UUID, findingId pgtype.UUID) error {
	err := p.query.AttachFindingToEvent(ctx, argus_db.AttachFindingToEventParams{
		EventID: pgtype.UUID{Bytes: eventId, Valid: true},
		ID:      findingId,
	})
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to attach finding", log.Err(err))
		return err
	}

	return nil
}

func (p *PolicyEngine) savePolicyState(ctx context.Context, seriesKey types.SeriesKey, state *PolicyState) error {
	pendingFindingUUIDs := make([]pgtype.UUID, 0)
	for _, finding := range state.PendingFindings {
		parsedId, err := uuid.Parse(finding.Id)
		if err != nil {
			p.logger.ErrorContext(ctx, "failed to parse open id", log.Err(err))
			return err
		}

		pendingFindingUUIDs = append(pendingFindingUUIDs, pgtype.UUID{Bytes: parsedId, Valid: true})
	}

	var openEventId uuid.UUID
	if state.OpenEventId != nil {
		openEventId = *state.OpenEventId
	}

	err := p.query.UpsertPolicyState(ctx, argus_db.UpsertPolicyStateParams{
		SerialID: seriesKey.SerialId,
		SrcIp:    seriesKey.SrcIP,
		Target:   seriesKey.Target,
		Port:     seriesKey.Port,
		Metric:   seriesKey.Metric,
		Detector: string(seriesKey.Detector),
		Status:   string(state.Status),
		Score:    state.BucketState.Score,
		ScoreUpdatedAt: pgtype.Timestamptz{
			Time:  state.BucketState.ScoreUpdatedAt,
			Valid: true,
		},
		OpenEventID:     pgtype.UUID{Bytes: openEventId, Valid: state.OpenEventId != nil},
		PendingFindings: pendingFindingUUIDs,
		EnteredStatusAt: pgtype.Timestamptz{Time: state.StatusUpdatedAt, Valid: true},
	})
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to update policy state", log.Err(err))
		return err
	}

	return nil
}
