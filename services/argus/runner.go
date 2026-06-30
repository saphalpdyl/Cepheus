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
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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

	ewma := NewEmwa(EwmaConfig{
		Alpha:         0.001,
		Threshold:     4,
		Warmup:        40,
		Epsilon:       1e8,
		SeverityAlpha: 2.2,
	})

	betab := NewBetaBinomial(BetaBinomialConfig{
		Threshold: 0.0001,
		Warmup:    50,
	})

	d.policyEngine, err = NewPolicyEngine(PolicyEngineConfig{
		Logger: d.logger.With("DOMAIN", "POLICY_ENGINE"),
		Query:  d.query,
		LeakyBucketConfiguration: LeakyBucketConfiguration{
			OpenThreshold:    5,
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
		types.DetectorTypeEwma:  ewma,
		types.DetectorTypeBetaB: betab,
	}

	registry := CreateDefaultRegistry()
	worker := NewWorker(d.query, d.logger, d.policyEngine, registry, detectors)

	nc, err := nats.Connect(
		d.config.NatsConnectURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(100),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to connect to nats", "url", d.config.NatsConnectURL, log.Err(err))
		return err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to connect to jetstream", log.Err(err))
		return err
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "MEASUREMENTS",
		Description: "Stream for processed measurement events consumed by argus",
		Subjects:    []string{"cepheus.measurement.>"},
	})
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to create or update measurements stream", log.Err(err))
		return err
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, "MEASUREMENTS", jetstream.ConsumerConfig{
		Name:          "argus",
		Durable:       "argus",
		FilterSubject: "cepheus.measurement.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to create or update consumer", log.Err(err))
		return err
	}

	router := NewRouter(consumer, worker, d.logger)
	go router.Start(ctx)

	d.logger.InfoContext(ctx, "argus running")

	<-ctx.Done()
	return nil
}
