package argus

import (
	argus_db "cepheus/argus/db"
	"cepheus/argus/log"
	"cepheus/argus/types"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Argus struct {
	InstanceId string

	config       DetectorConfig
	policyEngine *PolicyEngine

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *argus_db.Queries
}

func NewArgusInstance(instanceId string, config DetectorConfig, logger *slog.Logger) Argus {
	return Argus{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
	}
}

func (d *Argus) Start(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, d.config.DatabaseURL)
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to connect to database", log.Err(err))
		return err
	}
	defer pool.Close()

	d.pool = pool
	d.query = argus_db.New(d.pool)

	generateDbTransaction := func(ctx context.Context) (pgx.Tx, error) {
		return d.pool.Begin(ctx)
	}

	interval := time.Duration(d.config.DetectionIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.logger.InfoContext(ctx, "argus running", "interval", interval)

	ewma := NewEmwa(EwmaConfig{
		Alpha:         0.001,
		Threshold:     4,
		Warmup:        40,
		Epsilon:       1e-9,
		SeverityAlpha: 2.2,
	})

	d.policyEngine, err = NewPolicyEngine(PolicyEngineConfig{
		Logger: d.logger.With("DOMAIN", "POLICY_ENGINE"),
		Query:  d.query,
		LeakyBucketConfiguration: LeakyBucketConfiguration{
			OpenThreshold:    8,
			CloseThreshold:   3,
			DecayPerSecond:   0.1,
			BaseContribution: 1,
			MagnitudeAlpha:   1.2,
		},
		LeakyBucketSweepInterval: 30,
		QuietPeriod:              120 * time.Second,
		ConfirmWindow:            60 * time.Second,
		TransactionGenerator:     generateDbTransaction,
	})
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to init PolicyEngine", log.Err(err))
		return err
	}

	err = d.policyEngine.Start(ctx)
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to start PolicyEngine", log.Err(err))
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			activeSeries, err := d.query.ListActiveSeries(ctx, pgtype.Timestamptz{
				Time:  time.Now().Add(-24 * time.Hour),
				Valid: true,
			})

			if err != nil {
				d.logger.ErrorContext(ctx, "failed to list active series", log.Err(err))
				continue
			}

			for _, series := range activeSeries {
				// Get Baseline
				for _, metric := range []string{"rtt_p95_ns", "fwd_p95_ns", "bwd_p95_ns"} {
					for _, detector := range []string{"ewma"} {
						baseline, err := d.query.GetBaseline(ctx, argus_db.GetBaselineParams{
							SerialID: series.SerialID,
							Target:   series.Target,
							Port:     series.Port,
							Metric:   metric,
							Detector: detector,
						})
						if err != nil && !errors.Is(err, pgx.ErrNoRows) {
							d.logger.ErrorContext(ctx, "failed to get baseline state", log.Err(err))
							continue
						}

						baselineNotFound := errors.Is(err, pgx.ErrNoRows)
						var baselineState []byte
						if baselineNotFound {
							baselineState = nil
						} else {
							baselineState = baseline.State
						}

						var state json.RawMessage

						if detector == "ewma" {
							ewmaState, err := d.handleEWMA(
								ctx,
								ewma,
								series,
								baselineState,
								metric,
							)

							if err != nil {
								continue
							}

							state, err = json.Marshal(ewmaState)
							if err != nil {
								d.logger.ErrorContext(ctx, "failed to marshal EWMA state", log.Err(err))
								continue
							}
						}

						if state != nil {
							err := d.query.UpsertBaseline(ctx, argus_db.UpsertBaselineParams{
								SerialID: series.SerialID,
								Target:   series.Target,
								Port:     series.Port,
								Metric:   metric,
								Detector: detector,
								State:    state,
								N:        0,
								LastSeen: pgtype.Timestamptz{
									Time:  time.Now(),
									Valid: true,
								},
							})
							if err != nil {
								d.logger.ErrorContext(ctx, "failed to upsert baseline state", log.Err(err))
								continue
							}
						}
					}
				}
			}
		}
	}
}

func (d *Argus) handleEWMA(
	ctx context.Context,
	ewma *Ewma,
	series argus_db.ListActiveSeriesRow,
	baselineState []byte,
	metric string,
) (*EwmaState, error) {
	var state EwmaState
	if baselineState == nil {
		state = EwmaState{
			Mean:     0,
			Variance: 0,
			N:        0,
			LastSeen: time.Time{},
		}
	} else {
		// Deserialize EmwaState
		if err := json.Unmarshal(baselineState, &state); err != nil {
			d.logger.ErrorContext(ctx, "failed to unmarshal baseline state", log.Err(err))
			return nil, err
		}
	}

	// Get Samples
	samples, err := d.query.FetchStampSamples(ctx, argus_db.FetchStampSamplesParams{
		SerialID: series.SerialID,
		Target:   series.Target,
		Port:     series.Port,
		After: pgtype.Timestamptz{
			Time:  state.LastSeen,
			Valid: true,
		},
		Before: pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: true,
		},
	})

	if err != nil {
		d.logger.ErrorContext(ctx, "failed to fetch stamp samples", log.Err(err))
		return nil, err
	}

	for _, sample := range samples {
		var value float64
		switch metric {
		case "rtt_p95_ns":
			value = float64(sample.RttP95Ns)
		case "fwd_p95_ns":
			value = float64(sample.FwdP95Ns)
		case "bwd_p95_ns":
			value = float64(sample.BwdP95Ns)
		default:
			d.logger.ErrorContext(ctx, "invalid metric type, missing handler")
			return nil, errors.New("invalid metric type")
		}

		finding := ewma.Step(ctx, &state, types.Sample{
			Timestamp: sample.Timestamp.Time,
			Value:     value,
		})

		if finding != nil {
			seriesKey := types.SeriesKey{
				SerialId: series.SerialID,
				Target:   series.Target,
				Port:     series.Port,
				Metric:   metric,
				Detector: finding.Details.DetectorName(),
			}

			findingId, err := d.policyEngine.InsertFinding(ctx, seriesKey, finding)

			if err != nil {
				d.logger.ErrorContext(ctx, "failed to insert finding in DB", log.Err(err))
				return nil, err
			}

			findingUUID, err := findingId.UUIDValue()
			if err != nil {
				d.logger.ErrorContext(ctx, "failed to fetch finding UUID", log.Err(err))
				return nil, err
			}

			err = d.policyEngine.ApplyFinding(ctx, seriesKey, finding, findingUUID)
			if err != nil {
				d.logger.ErrorContext(ctx, "failed to apply finding", log.Err(err))
				return nil, err
			}
		}
	}

	return &state, nil
}
