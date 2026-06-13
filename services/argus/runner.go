package argus

import (
	"context"
	"log/slog"
	"time"

	argus_db "cepheus/services/argus/db"
	"cepheus/services/argus/log"
	"cepheus/services/argus/types"

	"github.com/jackc/pgx/v5"
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
		LeakyBucketSweepInterval: 30 * time.Second,
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

	// detectors maps a DetectorType to its implementation. Add an entry here as
	// each detector lands; the worker looks them up by type and skips any that
	// aren't registered yet.
	detectors := map[types.DetectorType]types.Detector{
		types.DetectorTypeEwma: ewma,
	}

	registry := CreateDefaultRegistry()
	workCh := make(chan DiscoveredSeries, 64)

	worker := NewWorker(d.query, d.logger, d.policyEngine, registry, detectors, workCh)
	go worker.Start(ctx)

	watcher := NewWatcher(d.query, workCh, d.logger)
	go watcher.Start(ctx)

	<-ctx.Done()
	return nil
}
