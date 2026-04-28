package stampprocessor

import (
	"cepheus/common"
	processor_shared "cepheus/processors/shared"
	"cepheus/processors/shared/log"
	stampprocessor_db "cepheus/processors/stamp-processor/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type StampProcessor struct {
	InstanceId string

	config StampProcessorConfig

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *stampprocessor_db.Queries
}

func NewStampProcessor(instanceId string, config StampProcessorConfig, logger *slog.Logger) StampProcessor {
	return StampProcessor{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
	}
}

func (s *StampProcessor) Start(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.config.DatabaseURL)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to connect to database", log.Err(err))
		return err
	}
	defer pool.Close()

	s.pool = pool
	s.query = stampprocessor_db.New(pool)

	nc, err := nats.Connect(
		s.config.NatsConnectURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(100),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to connect to nats", "url", s.config.NatsConnectURL, log.Err(err))
		return err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to connect to jetstream", log.Err(err))
		return err
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "PROBE_STAMP",
		Description: "Stream for STAMP probe data",
		Subjects:    []string{"cepheus.probe.stamp.>"},
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update stream", log.Err(err))
		return err
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("about to consume with subject: %s", s.config.NatsListenSubject))

	consumer, err := js.CreateOrUpdateConsumer(
		ctx,
		"PROBE_STAMP",
		jetstream.ConsumerConfig{
			Name:          "probe-stamp-processor",
			FilterSubject: s.config.NatsListenSubject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverNewPolicy,
		},
	)

	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update consumer", log.Err(err))
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
			if err != nil {
				s.logger.WarnContext(ctx, "consume failed", log.Err(err), "subject", s.config.NatsListenSubject)
				continue
			}

			for msg := range msgs.Messages() {

				var payload common.ReportPayload
				data := msg.Data()
				if err = json.Unmarshal(data, &payload); err != nil {
					s.logger.WarnContext(ctx, "failed to unmarshal payload", log.Err(err))
					_ = msg.Nak()
					continue
				}

				// Parse the inner data
				if payload.Payload.ProbeType != common.ProbeTypeStamp {
					s.logger.ErrorContext(ctx, "got invalid probe type", "expected", "stamp", "got", payload.Payload.ProbeType)
					_ = msg.Nak()
					continue
				}

				marshaledMap, err := json.Marshal(payload.Payload.Data)
				if err != nil {
					s.logger.ErrorContext(ctx, "failed to marshal data map to json", log.Err(err))
					_ = msg.Nak()
					continue
				}

				var stampData common.StampData
				if err = json.Unmarshal(marshaledMap, &stampData); err != nil {
					s.logger.WarnContext(ctx, "failed to unmarshal inner payload data", log.Err(err))
					_ = msg.Nak()
					continue
				}

				if err = s.insertStampData(
					ctx,
					payload.SerialID,
					&payload.AgentConfigId,
					stampData,
				); err != nil {
					_ = msg.Nak()
					continue
				}

				if err != nil {
					s.logger.ErrorContext(ctx, "failed to insert stamp data", log.Err(err))
					_ = msg.Nak()
					continue
				}

				err = msg.Ack()
				if err != nil {
					s.logger.ErrorContext(ctx, "failed to ack message", log.Err(err))
					return
				}
			}
		}

	}()

	<-ctx.Done()

	return nil
}

func (s *StampProcessor) insertStampData(
	ctx context.Context,
	serialId string,
	agentConfigId *string,
	stampData common.StampData,
) error {
	parsedAgentConfigId := &pgtype.UUID{
		Bytes: [16]byte{},
		Valid: false,
	}

	if agentConfigId != nil {
		var err error
		parsedAgentConfigId, err = processor_shared.UUID(*agentConfigId)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to parse agent config id", log.Err(err))
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to start transaction", log.Err(err))
		return err
	}
	defer tx.Rollback(ctx)

	// Insert stamp measurement first
	measurement, err := s.query.WithTx(tx).InsertStampMeasurement(ctx, stampprocessor_db.InsertStampMeasurementParams{
		Timestamp:     pgtype.Timestamptz{Time: stampData.Timestamp, Valid: true},
		SerialID:      serialId,
		AgentConfigID: *parsedAgentConfigId,
		Target:        stampData.Target,
		Port:          int32(stampData.Port),
		Sent:          int32(stampData.Sent),
		Received:      int32(stampData.Received),
		Loss:          stampData.Loss,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to insert stamp measurement", log.Err(err))
		return err
	}

	var stampProbesParams []stampprocessor_db.InsertStampProbesParams
	for _, probe := range stampData.Probes {
		stampProbesParams = append(stampProbesParams, stampprocessor_db.InsertStampProbesParams{
			MeasurementID: pgtype.UUID{
				Bytes: measurement.Bytes,
				Valid: measurement.Valid,
			},
			Tx: pgtype.Timestamptz{
				Time:  probe.Tx,
				Valid: true,
			},
			IsLost: probe.IsLost,
			Rx: pgtype.Timestamptz{
				Time:  probe.Rx,
				Valid: !probe.IsLost,
			},
			Rtt: pgtype.Int8{
				Int64: int64(probe.Rtt),
				Valid: !probe.IsLost,
			},
			ForwardDelay: pgtype.Int8{
				Int64: int64(probe.ForwardDelay),
				Valid: !probe.IsLost,
			},
			BackwardDelay: pgtype.Int8{
				Int64: int64(probe.BackwardDelay),
				Valid: !probe.IsLost,
			},
		})
	}

	_, err = s.query.WithTx(tx).InsertStampProbes(ctx, stampProbesParams)
	if err != nil {
		s.logger.ErrorContext(ctx, `failed to insert stamp measurement`, log.Err(err))
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.ErrorContext(ctx, "failed to commit transaction", log.Err(err))
		return err
	}

	return nil
}
