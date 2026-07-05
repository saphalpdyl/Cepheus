package argus

import (
	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/log"
	"cepheus/services/argus/types"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DiscoveredSeries is the router's hand-off: just the identity of a series to
// process. The worker expands it via the registry and owns its baselines.
type DiscoveredSeries struct {
	Type     types.SeriesType
	SerialId string
	Target   string
	Port     int32  // present only in STAMP
	Src      string // present only in TRACE
}

// RawSample is one measurement with its timestamp lifted out. Row stays opaque;
// the per-metric Extractor knows how to read it.
type RawSample struct {
	TS  time.Time
	Row any
}

// monitor is one detector watching one metric of a series: its key, how to read
// the value, its live state, and where to resume. The worker owns these and
// advances state/cursor in place every tick.
type monitor struct {
	key       types.SeriesKey
	extractor Extractor
	detector  types.Detector
	state     json.RawMessage
	cursor    time.Time
}

type Worker struct {
	query     *argus_db.Queries
	logger    *slog.Logger
	pe        *PolicyEngine
	pr        *PipelineRegistry
	detectors map[types.DetectorType]types.Detector
}

func NewWorker(
	query *argus_db.Queries,
	logger *slog.Logger,
	pe *PolicyEngine,
	pr *PipelineRegistry,
	detectors map[types.DetectorType]types.Detector,
) *Worker {
	return &Worker{
		query:     query,
		logger:    logger,
		pe:        pe,
		pr:        pr,
		detectors: detectors,
	}
}

// process owns one series for its lifetime: it loads the baselines once, then
// folds every sample the router routes to its inbox.
func (w *Worker) process(ctx context.Context, series DiscoveredSeries, inbox <-chan inboxItem) {
	// Since this is a one-time process for one series
	// the plan method, queries the pipeline registry, to prepare the
	// monitors(basically a wrapper of one detector with the extractor function and context)
	// and instantiates them
	monitors := w.plan(ctx, series)

	for {
		select {
		case <-ctx.Done():
			return
		case item := <-inbox:
			batch := drainInbox(inbox, item)

			// The fanout— a "row" packet fans out to multiple monitors
			if len(monitors) > 0 {
				rows := make([]RawSample, len(batch))
				for i, it := range batch {
					rows[i] = it.sample
				}
				for _, m := range monitors {
					w.fold(ctx, m, rows)
				}
			}

			for _, it := range batch {
				if err := it.msg.Ack(); err != nil {
					w.logger.ErrorContext(ctx, "failed to ack message", log.Err(err))
				}
			}
		}
	}
}

// drainInbox takes the first sample plus any already queued, so a burst folds
// and persists the baseline once instead of once per sample.
func drainInbox(inbox <-chan inboxItem, first inboxItem) []inboxItem {
	batch := []inboxItem{first}
	for {
		select {
		case it := <-inbox:
			batch = append(batch, it)
		default:
			return batch
		}
	}
}

// plan loads the baseline for every (metric, detector) the registry defines for
// this series type and returns one live monitor each.
func (w *Worker) plan(ctx context.Context, series DiscoveredSeries) []*monitor {
	var monitors []*monitor

	for _, extractor := range w.pr.GetExtractors(series.Type) {
		for _, detectorType := range extractor.Detectors {
			det, ok := w.detectors[detectorType]
			if !ok {
				// Detector not implemented yet (e.g. BETA, FREQ). Skip.
				continue
			}

			key := types.SeriesKey{
				Type:     series.Type,
				SerialId: series.SerialId,
				Target:   series.Target,
				Port:     series.Port,
				Metric:   extractor.MetricName,
				Detector: detectorType,
			}

			baseline, err := w.query.GetBaseline(ctx, argus_db.GetBaselineParams{
				SerialID: series.SerialId,
				Target:   series.Target,
				Port:     series.Port,
				Metric:   extractor.MetricName,
				Detector: string(detectorType),
				SrcIp:    "",
			})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				w.logger.ErrorContext(ctx, "failed to get baseline", log.Err(err), "key", key)
				continue
			}

			m := &monitor{key: key, extractor: extractor, detector: det}
			if err == nil { // baseline exists; resume from it
				m.state = baseline.State
				if baseline.LastSeen.Valid {
					m.cursor = baseline.LastSeen.Time
				}
			}

			monitors = append(monitors, m)
		}
	}

	return monitors
}

// fold runs every sample newer than the monitor's cursor through its detector,
// advancing the monitor's state/cursor in place and persisting if anything moved.
func (w *Worker) fold(ctx context.Context, m *monitor, rows []RawSample) {
	start := m.cursor

	for _, rs := range rows {
		if !rs.TS.After(m.cursor) {
			continue // already folded into this monitor's baseline
		}

		value, err := m.extractor.Extract(rs.Row)
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to extract value", log.Err(err))
			continue
		}

		next, finding, err := m.detector.Step(m.state, rs.TS, value)
		if err != nil {
			w.logger.ErrorContext(ctx, "detector step failed", log.Err(err))
			continue
		}
		m.state = next
		m.cursor = rs.TS

		if finding != nil {
			w.report(ctx, m.key, finding)
		}
	}

	if m.cursor.After(start) { // only persist if we folded something new
		if err := w.saveBaseline(ctx, m.key, m.state, m.cursor); err != nil {
			w.logger.ErrorContext(ctx, "failed to save baseline", log.Err(err))
		}
	}
}

func (w *Worker) report(ctx context.Context, key types.SeriesKey, finding *types.Finding) {
	findingId, err := w.pe.InsertFinding(ctx, key, finding)
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to insert finding", log.Err(err))
		return
	}

	if err := w.pe.ApplyFinding(ctx, key, finding, *findingId); err != nil {
		w.logger.ErrorContext(ctx, "failed to apply finding", log.Err(err))
	}
}

func (w *Worker) saveBaseline(ctx context.Context, key types.SeriesKey, state json.RawMessage, lastSeen time.Time) error {
	// TODO: Move baseline update to the policy engine so that we can
	// leave the update-freeze-on-finding concern there
	return w.query.UpsertBaseline(ctx, argus_db.UpsertBaselineParams{
		SerialID: key.SerialId,
		SrcIp:    key.SrcIP,
		Target:   key.Target,
		Port:     key.Port,
		Metric:   key.Metric,
		Detector: string(key.Detector),
		State:    state,
		N:        0,
		LastSeen: pgtype.Timestamptz{Time: lastSeen, Valid: true},
	})
}
