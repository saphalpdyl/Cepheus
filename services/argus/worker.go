package argus

import (
	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/log"
	"cepheus/services/argus/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DiscoveredSeries is the watcher's hand-off: just the identity of a series to
// process. The worker expands it via the registry and owns its baselines.
type DiscoveredSeries struct {
	Type     types.SeriesType
	SerialId string
	Target   string
	Port     int32  // present only in STAMP
	Src      string // present only in TRACE
}

// RawSample is one fetched measurement row with its timestamp lifted out.
// Row stays opaque; the per-metric Extractor knows how to read it.
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

	workCh <-chan DiscoveredSeries
}

func NewWorker(
	query *argus_db.Queries,
	logger *slog.Logger,
	pe *PolicyEngine,
	pr *PipelineRegistry,
	detectors map[types.DetectorType]types.Detector,
	workCh <-chan DiscoveredSeries,
) *Worker {
	return &Worker{
		query:     query,
		logger:    logger,
		pe:        pe,
		pr:        pr,
		detectors: detectors,
		workCh:    workCh,
	}
}

func (w *Worker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case series := <-w.workCh:
			go w.process(ctx, series)
		}
	}
}

// process owns one series for the lifetime of the worker: it loads the
// baselines once, then re-fetches and folds on a cadence.
func (w *Worker) process(ctx context.Context, series DiscoveredSeries) {
	monitors := w.plan(ctx, series)
	if len(monitors) == 0 {
		return
	}

	w.tick(ctx, series, monitors) // run once immediately, then on the cadence

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx, series, monitors)
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

// tick fetches once from the oldest cursor across all monitors (so one query
// serves every detector on the series) and folds each monitor forward.
func (w *Worker) tick(ctx context.Context, series DiscoveredSeries, monitors []*monitor) {
	oldest := time.Now()
	for _, m := range monitors {
		if m.cursor.Before(oldest) {
			oldest = m.cursor
		}
	}

	rows, err := w.fetch(ctx, series, oldest)
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to fetch samples", log.Err(err))
		return
	}
	if len(rows) == 0 {
		return
	}

	for _, m := range monitors {
		w.fold(ctx, m, rows)
	}
}

// fetch is the ONLY place series type matters. Each case runs its own query and
// boxes the rows into a uniform []RawSample; everything downstream is type-blind.
func (w *Worker) fetch(ctx context.Context, series DiscoveredSeries, after time.Time) ([]RawSample, error) {
	switch series.Type {
	case types.SeriesTypeStamp:
		rows, err := w.query.FetchStampSamples(ctx, argus_db.FetchStampSamplesParams{
			SerialID: series.SerialId,
			Target:   series.Target,
			Port:     series.Port,
			After:    pgtype.Timestamptz{Time: after, Valid: true},
			Before:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return nil, err
		}
		out := make([]RawSample, len(rows))
		for i, r := range rows {
			out[i] = RawSample{TS: r.Timestamp.Time, Row: r}
		}
		return out, nil
	case types.SeriesTypePing:
		rows, err := w.query.FetchPingSamples(ctx, argus_db.FetchPingSamplesParams{
			SerialID: series.SerialId,
			Target:   series.Target,
			After:    pgtype.Timestamptz{Time: after, Valid: true},
			Before:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return nil, err
		}

		out := make([]RawSample, len(rows))
		for i, r := range rows {
			out[i] = RawSample{TS: r.Timestamp.Time, Row: r}
		}
		return out, nil
	case types.SeriesTypeTrace:
		dst_ip, err := netip.ParseAddr(series.Target)
		if err != nil {
			return nil, fmt.Errorf("invalid target IP: %w", err)
		}

		// Verify that the Src is not a zero value
		if series.Src == "" {
			return nil, fmt.Errorf("source IP is empty: required for TRACE series")
		}

		src_ip, err := netip.ParseAddr(series.Src)
		if err != nil {
			return nil, fmt.Errorf("invalid source IP: %w", err)
		}

		rows, err := w.query.FetchTraceSamples(ctx, argus_db.FetchTraceSamplesParams{
			SerialID: series.SerialId,
			Dst:      dst_ip,
			Src:      src_ip,
			Type:     "trace",
		})

		if err != nil {
			return nil, err
		}

		out := make([]RawSample, len(rows))
		for i, r := range rows {
			out[i] = RawSample{TS: r.Timestamp.Time, Row: r}
		}
		return out, nil

	default:
		return nil, fmt.Errorf("no fetcher for series type %q", series.Type)
	}
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
