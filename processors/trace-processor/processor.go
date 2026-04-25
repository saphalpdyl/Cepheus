package traceprocessor

import (
	"cepheus/common"
	"cepheus/processors/shared/log"
	traceprocessor_db "cepheus/processors/trace-processor/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type TraceProcessor struct {
	InstanceId string

	config TraceProcessorConfig

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *traceprocessor_db.Queries
}

func NewTraceProcessor(instanceId string, config TraceProcessorConfig, logger *slog.Logger) TraceProcessor {
	return TraceProcessor{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
	}
}

func (s *TraceProcessor) Start(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.config.DatabaseURL)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to connect to database", log.Err(err))
		return err
	}
	defer pool.Close()

	s.pool = pool
	s.query = traceprocessor_db.New(pool)

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
		Name:        "PROBE_TRACE",
		Description: "Stream for TRACE probe data",
		Subjects:    []string{"cepheus.probe.trace.>"},
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update stream", log.Err(err))
		return err
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("about to consume with subject: %s", s.config.NatsListenSubject))

	consumer, err := js.CreateOrUpdateConsumer(
		ctx,
		"PROBE_TRACE",
		jetstream.ConsumerConfig{
			Name:          "probe-trace-processor",
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
				s.logger.InfoContext(ctx, "consumed data", "data", msg.Data())

				var payload common.ReportPayload
				data := msg.Data()
				if err = json.Unmarshal(data, &payload); err != nil {
					s.logger.WarnContext(ctx, "failed to unmarshal payload", log.Err(err))
					msg.Nak()
					continue
				}

				// Parse the inner data
				if payload.Payload.ProbeType != common.ProbeTypeTrace {
					s.logger.ErrorContext(ctx, "got invalid probe type", "expected", "trace", "got", payload.Payload.ProbeType)
					msg.Nak()
					continue
				}

				var traceData common.TraceData
				if err = json.Unmarshal(payload.Payload.Data, &traceData); err != nil {
					s.logger.ErrorContext(ctx, "couldn't unmarshal traceData wrapper")
					msg.Nak()
					continue
				}

				if traceData.Format != "json" {
					s.logger.ErrorContext(ctx, fmt.Sprintf("unsupported format %s", string(traceData.Format)))
					msg.Nak()
					continue
				}

				// Unmarshal json
				if traceData.Type == common.TraceProbeTypeTrace {
					var traceDataPayload common.TraceDataTracePayload
					if err = json.Unmarshal(traceData.Data, &traceDataPayload); err != nil {
						s.logger.ErrorContext(ctx, "failed to unmarshal normal json-based traceroute data", log.Err(err))
						msg.Nak()
						continue
					}

					// Do something here for now

				} else if traceData.Type == common.TraceProbeTypeTraceLb {
					// TODO: Do this
					s.logger.WarnContext(ctx, "json-based tracelb parser not implemented yet")
					msg.Ack()
					continue
				} else {
					s.logger.ErrorContext(ctx, fmt.Sprintf("unsupported type %s", string(traceData.Type)))
					msg.Nak()
					continue
				}

				msg.Ack()
			}
		}

	}()

	<-ctx.Done()

	return nil
}
