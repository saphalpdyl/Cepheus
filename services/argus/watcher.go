package argus

import (
	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/types"
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Watcher is responsible for watching the db for new unique series, get the baselines
// create the work packet and hand it off to the worker
type Watcher struct {
	seen   map[types.SeriesKey]struct{}
	query  *argus_db.Queries
	logger *slog.Logger
	pr     *PipelineRegistry
}

func NewWatcher(
	query *argus_db.Queries,
	pr *PipelineRegistry,
	logger *slog.Logger,
) *Watcher {
	return &Watcher{
		seen:   make(map[types.SeriesKey]struct{}),
		query:  query,
		logger: logger,
		pr:     pr,
	}
}

func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

	}
}

func (w *Watcher) getStampSeries(ctx context.Context) error {
	series, err := w.query.ListActiveStampSeries(ctx, pgtype.Timestamptz{
		Time:  time.Now().Add(-24 * time.Hour),
		Valid: true,
	})

	if err != nil {
		w.logger.ErrorContext(ctx, "failed to list active stamp series", "error", err)
		return err
	}

	// A work packet will contain data for single series with multiple detector baselines
	// Hence, for now, when the work is being done, one fetch can fan out to multiple detectors

	for _, s := range series {
		extractors := w.pr.GetExtractors(types.SeriesTypeStamp)
		for _, extractor := range extractors {
			for _, detector := range extractor.Detectors {
				_ = types.SeriesKey{
					SerialId: s.SerialID,
					Target:   s.Target,
					Port:     s.Port,
					Metric:   extractor.MetricName,
					Detector: detector,
				}
			}
		}
	}

	return nil
}
