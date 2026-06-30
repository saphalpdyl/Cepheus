package stampprocessor

import (
	"cepheus/libs/common"
	processor_shared "cepheus/libs/common/pgx"
	stampprocessor_db "cepheus/services/stamp-processor/db"
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

	log "cepheus/services/stamp-processor/log"
)

const (
	flushInterval = 2 * time.Second
	flushSize     = 50
	pendingBuffer = 256
)

type StampProcessor struct {
	InstanceId string

	config StampProcessorConfig

	logger *slog.Logger
	pool   *pgxpool.Pool
	query  *stampprocessor_db.Queries
	js     jetstream.JetStream
}

type LatencyStats struct {
	Mean   time.Duration
	P50    time.Duration
	P95    time.Duration
	StdDev time.Duration
}

type pendingStamp struct {
	msg         jetstream.Msg
	measurement stampprocessor_db.InsertStampMeasurementParams
	probes      []stampprocessor_db.InsertStampProbesParams
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
	s.js = js

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "PROBE_STAMP",
		Description: "Stream for STAMP probe data",
		Subjects:    []string{"cepheus.probe.stamp.>"},
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update stream", log.Err(err))
		return err
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "MEASUREMENTS",
		Description: "Stream for processed measurement events consumed by argus",
		Subjects:    []string{"cepheus.measurement.>"},
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create or update measurements stream", log.Err(err))
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

	pending := make(chan pendingStamp, pendingBuffer)
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

				measurement, probes, metrics := s.buildStamp(ctx, payload.SerialID, &payload.AgentConfigId, stampData)

				if err = s.publishMeasurement(ctx, payload.SerialID, stampData.Target, int32(stampData.Port), stampData.Timestamp, metrics); err != nil {
					s.logger.ErrorContext(ctx, "failed to publish measurement event", log.Err(err))
					_ = msg.Nak()
					continue
				}

				select {
				case pending <- pendingStamp{msg: msg, measurement: measurement, probes: probes}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	<-ctx.Done()

	return nil
}

func (s *StampProcessor) publishMeasurement(ctx context.Context, serialId string, target string, port int32, timestamp time.Time, metrics common.StampMetrics) error {
	metricsData, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	event := common.MeasurementEvent{
		Type:      common.ProbeTypeStamp,
		SerialID:  serialId,
		Target:    target,
		Port:      port,
		Timestamp: timestamp,
		Metrics:   metricsData,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("cepheus.measurement.stamp.%s", serialId)

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

func (s *StampProcessor) runFlusher(ctx context.Context, pending <-chan pendingStamp) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []pendingStamp
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

func (s *StampProcessor) flush(ctx context.Context, batch []pendingStamp) {
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
		measurement, err := q.InsertStampMeasurement(ctx, batch[i].measurement)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to insert stamp measurement", log.Err(err))
			s.nakAll(ctx, batch)
			return
		}

		if len(batch[i].probes) == 0 {
			continue
		}

		for j := range batch[i].probes {
			batch[i].probes[j].MeasurementID = pgtype.UUID{Bytes: measurement.Bytes, Valid: measurement.Valid}
		}

		if _, err := q.InsertStampProbes(ctx, batch[i].probes); err != nil {
			s.logger.ErrorContext(ctx, "failed to insert stamp probes", log.Err(err))
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

func (s *StampProcessor) nakAll(ctx context.Context, batch []pendingStamp) {
	for i := range batch {
		if err := batch[i].msg.Nak(); err != nil {
			s.logger.ErrorContext(ctx, "failed to nak message", log.Err(err))
		}
	}
}

func (s *StampProcessor) buildStamp(
	ctx context.Context,
	serialId string,
	agentConfigId *string,
	stampData common.StampData,
) (stampprocessor_db.InsertStampMeasurementParams, []stampprocessor_db.InsertStampProbesParams, common.StampMetrics) {
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

	rtts := make([]time.Duration, 0, len(stampData.Probes))
	fwds := make([]time.Duration, 0, len(stampData.Probes))
	bwds := make([]time.Duration, 0, len(stampData.Probes))

	for _, probe := range stampData.Probes {
		if probe.IsLost {
			continue
		}

		rtts = append(rtts, probe.Rtt)
		fwds = append(fwds, probe.ForwardDelay)
		bwds = append(bwds, probe.BackwardDelay)
	}

	rttStats := computeStats(rtts)
	fwdStats := computeStats(fwds)
	bwdStats := computeStats(bwds)

	measurement := stampprocessor_db.InsertStampMeasurementParams{
		Timestamp:     pgtype.Timestamptz{Time: stampData.Timestamp, Valid: true},
		SerialID:      serialId,
		AgentConfigID: *parsedAgentConfigId,
		Target:        stampData.Target,
		Port:          int32(stampData.Port),
		Sent:          int32(stampData.Sent),
		Received:      int32(stampData.Received),
		Loss:          stampData.Loss,
		RttP95Ns:      int64(rttStats.P95),
		FwdP95Ns:      int64(fwdStats.P95),
		BwdP95Ns:      int64(bwdStats.P95),
	}

	var probeRows []stampprocessor_db.InsertStampProbesParams
	for _, probe := range stampData.Probes {
		probeRows = append(probeRows, stampprocessor_db.InsertStampProbesParams{
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

	metrics := common.StampMetrics{
		RttP95Ns: int64(rttStats.P95),
		FwdP95Ns: int64(fwdStats.P95),
		BwdP95Ns: int64(bwdStats.P95),
		Sent:     int64(stampData.Sent),
		Received: int64(stampData.Received),
	}

	return measurement, probeRows, metrics
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
