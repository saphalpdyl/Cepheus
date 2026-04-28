package traceprocessor

import (
	"cepheus/common"
	processor_shared "cepheus/common/pgx"
	traceprocessor_db "cepheus/trace-processor/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	log "cepheus/trace-processor/log"

	asn "github.com/superfrink/go-cymru-asn"
)

type TraceProcessor struct {
	InstanceId string

	config TraceProcessorConfig

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *traceprocessor_db.Queries

	// IP -> ASN mapper client
	asnClient *asn.Client
}

func NewTraceProcessor(instanceId string, config TraceProcessorConfig, logger *slog.Logger) TraceProcessor {
	return TraceProcessor{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
		asnClient: asn.NewClient(
			asn.WithTimeout(time.Second * 5),
		),
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
					if err = s.processNormalTrace(ctx, pool, &traceData, &payload, payload.SerialID, payload.AgentConfigId); err != nil {
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

func (s *TraceProcessor) processNormalTrace(ctx context.Context, pool *pgxpool.Pool, traceData *common.TraceData, payload *common.ReportPayload, serialId string, agentConfigId string) error {
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

	parsedAgentConfigId, err := processor_shared.UUID(agentConfigId)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to parse agent config id", log.Err(err))
		return err
	}

	measurement, err := s.query.WithTx(tx).InsertTraceMeasurement(
		ctx,
		traceprocessor_db.InsertTraceMeasurementParams{
			Timestamp:     pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			SerialID:      serialId,
			AgentConfigID: *parsedAgentConfigId,
			Type:          string(traceData.Type),
			Src:           srcIp,
			Dst:           dstIp,
			Method:        string(traceData.Method),
			StopReason:    traceDataPayload.StopReason,
			HopCount:      int32(traceDataPayload.HopCount),
			PathHash:      "",
			Raw:           traceData.Data,
		},
	)

	if err != nil {
		s.logger.ErrorContext(ctx, "failed to insert trace measurement", log.Err(err))
		return err
	}

	// Generate trace hops
	var traceHops []traceprocessor_db.InsertTraceHopParams
	var hopIPs []netip.Addr
	for _, hop := range traceDataPayload.Hops {
		var hopIp netip.Addr
		if hop.Addr != "" {
			hopIp, err = netip.ParseAddr(hop.Addr)
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to parse hop ip", log.Err(err), "hop_ip", hop.Addr)
				return err
			}
		}

		// Used for feeds
		hopIPs = append(hopIPs, hopIp)

		traceHops = append(traceHops, traceprocessor_db.InsertTraceHopParams{
			Timestamp:     pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			MeasurementID: measurement.ID,
			Ip:            &hopIp,
			Ttl:           int32(hop.ProbeTTL),
			Rtt:           processor_shared.Int8(int64(hop.Rtt * float64(time.Millisecond))), // Converting milliseconds to nanoseconds to conform to standards
			IcmpType:      processor_shared.Int4(hop.IcmpType),
			IcmpCode:      processor_shared.Int4(hop.IcmpCode),
			ReplyTtl:      processor_shared.Int4(hop.ReplyTTL),
			Asn:           processor_shared.Int4(0),
			IsNoHop:       false,
		})
	}

	existingIPs, err := s.query.WithTx(tx).GetExistingIPs(ctx, hopIPs)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to query existing ips", log.Err(err))
		return err
	}

	if err = s.enrichMissingHops(ctx, hopIPs, existingIPs, tx); err != nil {
		return err
	}

	for _, hop := range traceDataPayload.NoHops {
		traceHops = append(traceHops, traceprocessor_db.InsertTraceHopParams{
			Timestamp:     pgtype.Timestamptz{Time: payload.Payload.Timestamp, Valid: true},
			MeasurementID: measurement.ID,
			Ip:            nil,
			Ttl:           int32(hop.ProbeTTL),
			Rtt:           pgtype.Int8{},
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

func (s *TraceProcessor) enrichMissingHops(ctx context.Context, allIPs []netip.Addr, existingIPs []netip.Addr, tx pgx.Tx) error {
	// Get non-existing IPs
	existingIPsSet := make(map[netip.Addr]bool)
	for _, ip := range existingIPs {
		existingIPsSet[ip] = true
	}

	var missingIPs []string
	for _, ip := range allIPs {
		if !existingIPsSet[ip] {
			missingIPs = append(missingIPs, ip.String())
		}
	}

	// TODO: Instead of this, lazily load in ASN Prefixes from RipeSTAT
	asnResp, err := s.asnClient.Lookup(ctx, missingIPs)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to lookup missing ips", log.Err(err))
		return err
	}

	var asDetails []traceprocessor_db.UpsertAsDetailsParams

	for _, r := range asnResp.Results {
		ip, err := netip.ParseAddr(r.IP)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to parse ip inside as lookup", log.Err(err))
			return err
		}

		asDetails = append(asDetails, traceprocessor_db.UpsertAsDetailsParams{
			Ip:  ip,
			Asn: processor_shared.Int4(r.ASN),
			BgpPrefix: pgtype.Text{
				String: r.BGPPrefix,
				Valid:  r.BGPPrefix != "NA",
			},
			Name: pgtype.Text{
				String: r.ASName,
				Valid:  r.ASName != "NA",
			},
			Cc: pgtype.Text{
				String: r.CountryCode,
				Valid:  r.CountryCode != "",
			},
		})
	}

	_, err = s.query.WithTx(tx).UpsertAsDetails(ctx, asDetails)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update as details", log.Err(err))
		return err
	}
	return nil
}
