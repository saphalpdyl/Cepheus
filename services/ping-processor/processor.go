package pingprocessor

import (
	"cepheus/libs/common"
	processor_shared "cepheus/libs/common/pgx"
	pingprocessor_db "cepheus/services/ping-processor/db"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/avast/retry-go"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	log "cepheus/services/ping-processor/log"
)

const (
	flushInterval = 2 * time.Second
	flushSize     = 50
	pendingBuffer = 256
)

type PingProcessor struct {
	InstanceId string

	config PingProcessorConfig

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *pingprocessor_db.Queries
	js     jetstream.JetStream
}

type LatencyStats struct {
	Mean   time.Duration
	P50    time.Duration
	P95    time.Duration
	StdDev time.Duration
}

type pendingPing struct {
	msg         jetstream.Msg
	measurement pingprocessor_db.InsertPingMeasurementParams
	probes      []pingprocessor_db.InsertPingProbesParams
}

func NewPingProcessor(instanceId string, config PingProcessorConfig, logger *slog.Logger) PingProcessor {
	return PingProcessor{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
	}
}

func (s *PingProcessor) Start(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.config.DatabaseURL)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to connect to database", log.Err(err))
		return err
	}
	defer pool.Close()

	s.pool = pool
	s.query = pingprocessor_db.New(pool)

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
	s.js = js

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "PROBE_PING",
		Description: "Stream for ping probe data",
		Subjects:    []string{"cepheus.probe.ping.>"},
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update stream", log.Err(err))
		return err
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "MEASUREMENTS",
		Description: "Stream for processed measurement events consumed by argus",
		Subjects:    []string{"cepheus.measurement.>"},
		MaxAge:      24 * time.Hour,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update measurements stream", log.Err(err))
		return err
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("about to consume with subject: %s", s.config.NatsListenSubject))

	consumer, err := js.CreateOrUpdateConsumer(
		ctx,
		"PROBE_PING",
		jetstream.ConsumerConfig{
			Name:          "probe-ping-processor",
			FilterSubject: s.config.NatsListenSubject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverNewPolicy,
		},
	)

	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update consumer", log.Err(err))
		return err
	}

	pending := make(chan pendingPing, pendingBuffer)
	go s.runFlusher(ctx, pending)

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
				if err = json.Unmarshal(msg.Data(), &payload); err != nil {
					s.logger.WarnContext(ctx, "failed to unmarshal payload", log.Err(err))
					_ = msg.Nak()
					continue
				}

				if payload.Payload.ProbeType != common.ProbeTypePing {
					s.logger.ErrorContext(ctx, "got invalid probe type", "expected", "ping", "got", payload.Payload.ProbeType)
					_ = msg.Nak()
					continue
				}

				var pingData common.PingDataPayload
				if err = json.Unmarshal(payload.Payload.Data, &pingData); err != nil {
					s.logger.WarnContext(ctx, "failed to unmarshal ping data", log.Err(err))
					_ = msg.Nak()
					continue
				}

				measurement, probes, metrics := s.buildPing(
					ctx,
					payload.SerialID,
					&payload.AgentConfigId,
					payload.Payload.Timestamp,
					pingData,
				)

				if err = s.publishMeasurement(ctx, payload.SerialID, pingData.Dst, payload.Payload.Timestamp, metrics); err != nil {
					s.logger.ErrorContext(ctx, "failed to publish measurement event", log.Err(err))
					_ = msg.Nak()
					continue
				}

				select {
				case pending <- pendingPing{msg: msg, measurement: measurement, probes: probes}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	<-ctx.Done()

	return nil
}

func (s *PingProcessor) publishMeasurement(ctx context.Context, serialId string, target string, timestamp time.Time, metrics common.PingMetrics) error {
	metricsData, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	event := common.MeasurementEvent{
		Type:      common.ProbeTypePing,
		SerialID:  serialId,
		Target:    target,
		Timestamp: timestamp,
		Metrics:   metricsData,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("cepheus.measurement.ping.%s", serialId)

	return retry.Do(
		func() error {
			_, err := s.js.Publish(ctx, subject, eventData)
			return err
		},
		retry.Context(ctx),
		retry.Attempts(3),
		retry.DelayType(retry.BackOffDelay),
	)
}

func (s *PingProcessor) runFlusher(ctx context.Context, pending <-chan pendingPing) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []pendingPing
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-pending:
			batch = append(batch, p)
			if len(batch) >= flushSize {
				s.flush(ctx, batch)
				batch = nil
			}
		case <-ticker.C:
			s.flush(ctx, batch)
			batch = nil
		}
	}
}

func (s *PingProcessor) flush(ctx context.Context, batch []pendingPing) {
	if len(batch) == 0 {
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to start transaction", log.Err(err))
		s.nakAll(ctx, batch)
		return
	}
	defer tx.Rollback(ctx)

	q := s.query.WithTx(tx)
	for i := range batch {
		measurement, err := q.InsertPingMeasurement(ctx, batch[i].measurement)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to insert ping measurement", log.Err(err))
			s.nakAll(ctx, batch)
			return
		}

		if len(batch[i].probes) == 0 {
			continue
		}

		for j := range batch[i].probes {
			batch[i].probes[j].MeasurementID = pgtype.UUID{Bytes: measurement.Bytes, Valid: measurement.Valid}
		}

		if _, err := q.InsertPingProbes(ctx, batch[i].probes); err != nil {
			s.logger.ErrorContext(ctx, "failed to insert ping probes", log.Err(err))
			s.nakAll(ctx, batch)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.ErrorContext(ctx, "failed to commit transaction", log.Err(err))
		s.nakAll(ctx, batch)
		return
	}

	for i := range batch {
		if err := batch[i].msg.Ack(); err != nil {
			s.logger.ErrorContext(ctx, "failed to ack message", log.Err(err))
		}
	}
}

func (s *PingProcessor) nakAll(ctx context.Context, batch []pendingPing) {
	for i := range batch {
		if err := batch[i].msg.Nak(); err != nil {
			s.logger.ErrorContext(ctx, "failed to nak message", log.Err(err))
		}
	}
}

func (s *PingProcessor) buildPing(
	ctx context.Context,
	serialId string,
	agentConfigId *string,
	timestamp time.Time,
	pingData common.PingDataPayload,
) (pingprocessor_db.InsertPingMeasurementParams, []pingprocessor_db.InsertPingProbesParams, common.PingMetrics) {
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

	rtts := make([]time.Duration, 0, len(pingData.Responses))
	for _, r := range pingData.Responses {
		rtts = append(rtts, msToDuration(r.Rtt))
	}

	stats := computeStats(rtts)

	sent := int32(pingData.PingSent)
	received := int32(pingData.Statistics.Replies)
	loss := 0.0
	if sent > 0 {
		loss = 1.0 - float64(received)/float64(sent)
	}

	measurement := pingprocessor_db.InsertPingMeasurementParams{
		Timestamp:     pgtype.Timestamptz{Time: timestamp, Valid: true},
		SerialID:      serialId,
		AgentConfigID: *parsedAgentConfigId,
		Target:        pingData.Dst,
		Sent:          sent,
		Received:      received,
		Loss:          loss,
		RttMinNs:      int64(msToDuration(pingData.Statistics.Min)),
		RttAvgNs:      int64(msToDuration(pingData.Statistics.Avg)),
		RttMaxNs:      int64(msToDuration(pingData.Statistics.Max)),
		RttP50Ns:      int64(stats.P50),
		RttP95Ns:      int64(stats.P95),
		RttStddevNs:   int64(msToDuration(pingData.Statistics.Stddev)),
	}

	var probeRows []pingprocessor_db.InsertPingProbesParams
	for _, r := range pingData.Responses {
		probeRows = append(probeRows, pingprocessor_db.InsertPingProbesParams{
			Tx: pgtype.Timestamptz{
				Time:  time.Unix(int64(r.Tx.Sec), int64(r.Tx.Usec)*1000),
				Valid: true,
			},
			Rx: pgtype.Timestamptz{
				Time:  time.Unix(int64(r.Rx.Sec), int64(r.Rx.Usec)*1000),
				Valid: true,
			},
			IsLost: false,
			Seq:    processor_shared.Int4(r.Seq),
			Rtt:    processor_shared.Int8(int64(msToDuration(r.Rtt))),
		})
	}

	lostCount := sent - received
	for i := int32(0); i < lostCount; i++ {
		probeRows = append(probeRows, pingprocessor_db.InsertPingProbesParams{
			Tx:     pgtype.Timestamptz{Time: timestamp, Valid: true},
			Rx:     pgtype.Timestamptz{},
			IsLost: true,
			Seq:    pgtype.Int4{},
			Rtt:    pgtype.Int8{},
		})
	}

	metrics := common.PingMetrics{
		RttP95Ns: int64(stats.P95),
		RttP50Ns: int64(stats.P50),
		Sent:     int64(sent),
		Received: int64(received),
	}

	return measurement, probeRows, metrics
}

func msToDuration(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

func computeStats(values []time.Duration) LatencyStats {
	if len(values) == 0 {
		return LatencyStats{}
	}

	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	var sum int64
	for _, v := range values {
		sum += int64(v)
	}
	mean := time.Duration(sum / int64(len(values)))

	return LatencyStats{
		Mean: mean,
		P50:  percentile(values, 0.50),
		P95:  percentile(values, 0.95),
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return time.Duration(float64(sorted[lo])*(1-frac) + float64(sorted[hi])*frac)
}
