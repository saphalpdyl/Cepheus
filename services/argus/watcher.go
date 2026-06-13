package argus

import (
	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/types"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Watcher discovers unique series in the db and hands each one to the worker
// exactly once. It knows nothing about metrics, detectors, or baselines — the
// worker owns all of that.
type Watcher struct {
	mu   sync.Mutex
	seen map[DiscoveredSeries]struct{}

	query  *argus_db.Queries
	workCh chan<- DiscoveredSeries

	logger *slog.Logger
}

func NewWatcher(
	query *argus_db.Queries,
	workCh chan<- DiscoveredSeries,
	logger *slog.Logger,
) *Watcher {
	return &Watcher{
		seen:   make(map[DiscoveredSeries]struct{}),
		query:  query,
		workCh: workCh,
		logger: logger,
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
			go w.getStampSeries(ctx)
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

	for _, s := range series {
		ds := DiscoveredSeries{
			Type:     types.SeriesTypeStamp,
			SerialId: s.SerialID,
			Target:   s.Target,
			Port:     s.Port,
		}

		// Emit each series exactly once; the worker tracks it from there.
		w.mu.Lock()
		if _, exists := w.seen[ds]; exists {
			w.mu.Unlock()
			continue
		}
		w.seen[ds] = struct{}{}
		w.mu.Unlock()

		w.workCh <- ds
	}

	return nil
}
