package argus

import (
	"cepheus/libs/common"
	"cepheus/services/argus/log"
	"cepheus/services/argus/types"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type inboxItem struct {
	sample RawSample
	msg    jetstream.Msg
}

// Router consumes measurement events from NATS and hands each one to the
// per-series worker that owns its baseline. Routing is by series identity, so
// every sample for a series always reaches the same worker and its state stays
// owned by a single goroutine.
type Router struct {
	consumer jetstream.Consumer
	worker   *Worker
	logger   *slog.Logger
	routes   map[DiscoveredSeries]chan inboxItem
}

func NewRouter(consumer jetstream.Consumer, worker *Worker, logger *slog.Logger) *Router {
	return &Router{
		consumer: consumer,
		worker:   worker,
		logger:   logger,
		routes:   make(map[DiscoveredSeries]chan inboxItem),
	}
}

func (r *Router) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := r.consumer.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			r.logger.WarnContext(ctx, "fetch failed", log.Err(err))
			continue
		}

		for msg := range msgs.Messages() {
			r.dispatch(ctx, msg)
		}
	}
}

func (r *Router) dispatch(ctx context.Context, msg jetstream.Msg) {
	var event common.MeasurementEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		r.logger.ErrorContext(ctx, "failed to unmarshal measurement event", log.Err(err))
		_ = msg.Term()
		return
	}

	series, ok := seriesFromEvent(event)
	if !ok {
		r.logger.ErrorContext(ctx, "unknown probe type", "type", event.Type)
		_ = msg.Term()
		return
	}

	metrics, err := decodeMetrics(event)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to decode metrics", log.Err(err), "type", event.Type)
		_ = msg.Term()
		return
	}

	inbox, ok := r.routes[series]
	if !ok {
		inbox = make(chan inboxItem, 64)
		r.routes[series] = inbox
		go r.worker.process(ctx, series, inbox)
	}

	select {
	case inbox <- inboxItem{sample: RawSample{TS: event.Timestamp, Row: metrics}, msg: msg}:
	case <-ctx.Done():
	}
}

func seriesFromEvent(event common.MeasurementEvent) (DiscoveredSeries, bool) {
	switch event.Type {
	case common.ProbeTypePing:
		return DiscoveredSeries{Type: types.SeriesTypePing, SerialId: event.SerialID, Target: event.Target}, true
	case common.ProbeTypeStamp:
		return DiscoveredSeries{Type: types.SeriesTypeStamp, SerialId: event.SerialID, Target: event.Target, Port: event.Port}, true
	case common.ProbeTypeTrace:
		return DiscoveredSeries{Type: types.SeriesTypeTrace, SerialId: event.SerialID, Target: event.Target, Src: event.Src}, true
	default:
		return DiscoveredSeries{}, false
	}
}

func decodeMetrics(event common.MeasurementEvent) (any, error) {
	switch event.Type {
	case common.ProbeTypePing:
		var m common.PingMetrics
		err := json.Unmarshal(event.Metrics, &m)
		return m, err
	case common.ProbeTypeStamp:
		var m common.StampMetrics
		err := json.Unmarshal(event.Metrics, &m)
		return m, err
	case common.ProbeTypeTrace:
		var m common.TraceMetrics
		err := json.Unmarshal(event.Metrics, &m)
		return m, err
	default:
		return nil, fmt.Errorf("unknown probe type %q", event.Type)
	}
}
