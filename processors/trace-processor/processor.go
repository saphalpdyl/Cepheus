package traceprocessor

import (
	"cepheus/common"
	processor_shared "cepheus/processors/shared"
	"cepheus/processors/shared/log"
	traceprocessor_db "cepheus/processors/trace-processor/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
					if err = s.processNormalTrace(
						ctx,
						pool,
						&traceData,
						&payload,
					); err != nil {
						msg.Nak()
						continue
					}

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

func (s *TraceProcessor) processNormalTrace(
	ctx context.Context,
	pool *pgxpool.Pool,
	traceData *common.TraceData,
	payload *common.ReportPayload,
) error {
	var traceDataPayload common.TraceDataTracePayload
	if err := json.Unmarshal(traceData.Data, &traceDataPayload); err != nil {
		s.logger.ErrorContext(ctx, "failed to unmarshal normal json-based traceroute data", log.Err(err))
		return err
	}

	srcIp, err := netip.ParseAddr(traceDataPayload.Src)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse src ip", log.Err(err), "src", traceDataPayload.Src)
		return err
	}

	dstIp, err := netip.ParseAddr(traceDataPayload.Dst)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse dst ip", log.Err(err), "dst", traceDataPayload.Dst)
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to begin transaction", log.Err(err))
		return err
	}

	defer tx.Rollback(ctx)

	measurement, err := s.query.WithTx(tx).InsertTraceMeasurement(
		ctx,
		traceprocessor_db.InsertTraceMeasurementParams{
			Timestamp:  pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			Type:       string(traceData.Type),
			Src:        srcIp,
			Dst:        dstIp,
			Method:     string(traceData.Method),
			StopReason: traceDataPayload.StopReason,
			HopCount:   int32(traceDataPayload.HopCount),
			PathHash:   "",
			Raw:        traceData.Data,
		},
	)

	if err != nil {
		s.logger.ErrorContext(ctx, "failed to insert trace measurement", log.Err(err))
		return err
	}

	// Generate trace hops
	var traceHops []traceprocessor_db.InsertTraceHopParams
	for _, hop := range traceDataPayload.Hops {
		var hopIp netip.Addr
		if hop.Addr != "" {
			hopIp, err = netip.ParseAddr(hop.Addr)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to parse hop ip", log.Err(err), "hop_ip", hop.Addr)
				return err
			}
		}

		traceHops = append(traceHops, traceprocessor_db.InsertTraceHopParams{
			Timestamp:     pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			MeasurementID: measurement.ID,
			Ip:            &hopIp,
			Ttl:           int32(hop.ProbeTTL),
			RttMs:         processor_shared.Float8(time.Duration(hop.Rtt) / time.Millisecond),
			IcmpType:      processor_shared.Int4(hop.IcmpType),
			IcmpCode:      processor_shared.Int4(hop.IcmpCode),
			ReplyTtl:      processor_shared.Int4(hop.ReplyTTL),
			Asn:           processor_shared.Int4(0),
			IsNoHop:       false,
		})
	}

	for _, hop := range traceDataPayload.NoHops {
		traceHops = append(traceHops, traceprocessor_db.InsertTraceHopParams{
			Timestamp:     pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			MeasurementID: measurement.ID,
			Ip:            nil,
			Ttl:           int32(hop.ProbeTTL),
			RttMs:         pgtype.Float8{},
			IcmpType:      pgtype.Int4{},
			IcmpCode:      pgtype.Int4{},
			ReplyTtl:      pgtype.Int4{},
			Asn:           pgtype.Int4{},
			IsNoHop:       true,
		})
	}

	_, err = s.query.WithTx(tx).InsertTraceHop(ctx, traceHops)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to insert trace hops", log.Err(err))
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.ErrorContext(ctx, "failed to commit transaction", log.Err(err))
		return err
	}

	return nil
}
