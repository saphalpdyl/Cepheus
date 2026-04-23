// dispatcher is responsible for batching telemetry data in supervisor's dataStream buffer
// and send it to external message broker ( NATS in this case ) at regular intervals.

// Dispatcher is greedy when batching, so if a batch fails and it retries, it will try
// to fill the buffer until it fills.
package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"cepheus/telemetry"
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/avast/retry-go"
	"github.com/nats-io/nats.go/jetstream"
)

type Dispatcher struct {
	probeDataStream *ProbeDataStream
	logger          *slog.Logger

	// batcher config
	batchSize   uint32
	batchBuffer []api.ProbeResult // a buffer of size `batchSize` to hold telemetry data for retries

	ticker *time.Ticker

	// Shutdown management
	done   chan struct{}
	cancel context.CancelFunc

	// NATS JetStream
	// no abstractions for now
	js        jetstream.JetStream
	jsTimeout time.Duration
}

type DispatcherConfig struct {
	ProbeDataStream *ProbeDataStream
	Logger          *slog.Logger
	BatchSize       uint32
	JetStream       jetstream.JetStream

	JetStreamTimeout time.Duration
}

func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{
		probeDataStream: cfg.ProbeDataStream,
		logger:          cfg.Logger,
		batchSize:       cfg.BatchSize,
		js:              cfg.JetStream,
		jsTimeout:       cfg.JetStreamTimeout,
		batchBuffer:     make([]api.ProbeResult, cfg.BatchSize),
	}
}

func (d *Dispatcher) Start(ctx context.Context, interval time.Duration) (err error) {
	ctx, d.cancel = context.WithCancel(ctx)
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Dispatcher.Start", nil)
	defer end()

	done := make(chan struct{})

	if d.ticker != nil {
		d.ticker.Stop()
	}

	d.ticker = time.NewTicker(interval)
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				d.ticker.Stop()
				return
			case <-d.ticker.C:
				if err := d.dispatch(ctx); err != nil {
					d.logger.ErrorContext(ctx, "failed to dispatch telemetry data", log.Err(err))
				}
			}
		}
	}()

	return nil
}

func (d *Dispatcher) Stop() {
	if d.cancel != nil {
		d.cancel()
	}

	<-d.done
}

func (d *Dispatcher) dispatch(ctx context.Context) (err error) {
	ctx, end, span := telemetry.SpanWithErr(ctx, "Dispatcher.dispatch", nil)
	defer end()

	// 1. We mark upto maxBatchSize as ready to be sent
	// 2. We try sending that batch to the message broker
	// 3. If success, we remove them from the buffer
	// 4. If fails, we keep them in the buffer and try agian later

	// Edge case:
	// - If n is the max batch size and we have m ( m < n ) items in the buffer,
	//   we drain the buffer upto m times and try send them by storing them in dispatcher
	//  local buffer. If it fails, we keep them in the local buffer and try again.
	//  However, if there are >0 items in the probe data buffer by the time we are on our next tick
	//	we will try to fill up the batch before trying to send another.

	// Send all the data in batches into the same subject
	// The go data procesors will unfurl the data and send it to the right place

	d.logger.InfoContext(ctx, "starting dispatch...")

	err = retry.Do(func() error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second) // TODO: Hardcoded here change it
		defer cancel()

		if len(d.batchBuffer) > 0 {
			// retry
			span.AddEvent("retry")
			d.logger.InfoContext(ctx, "retrying to telemetry batche from buffer")
		}

		// 40/50 of batchBuffer filled ( for example )
		// batch size of 20 not good
		// take the minimum of 20 & (50 - 40 = 10)
		pullSize := min(int(d.batchSize), int(d.batchSize)-len(d.batchBuffer))
		if pullSize > 0 {
			res := d.probeDataStream.Pull(ctx, pullSize)
			if res == nil {
				// Context finished
				return nil
			}

			d.batchBuffer = append(d.batchBuffer, (*res)...)
		}

		payload, err := json.Marshal(map[string]any{
			"type":      "batch_upload", // TODO: make configurable
			"timestamp": time.Now().Unix(),
			"count":     len(d.batchBuffer),
			"data":      d.batchBuffer,
		})

		if err != nil {
			d.logger.ErrorContext(ctx, "could not marshal payload data", "data", d.batchBuffer)
			return err
		}

		// TODO: implement deduplication throguh NATS MsgIDHeader
		ack, err := d.js.Publish(ctx, "cepheus.probe.batch.upload", payload)
		if err != nil {
			d.logger.ErrorContext(ctx, "NATS Publish returned error", log.Err(err))
			return err
		}

		if ack.Duplicate {
			d.logger.WarnContext(ctx, "published a duplicate buffer")
		}

		d.logger.InfoContext(ctx, "dispatch completed", "buffer_size", len(d.batchBuffer), "js_seq", ack.Sequence, "probe_data_buffer_size", len(d.probeDataStream.stream))

		// Drain local buffer
		clear(d.batchBuffer)
		d.batchBuffer = d.batchBuffer[:0]

		return nil
	},
		retry.Context(ctx),
		retry.Attempts(3),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			d.logger.ErrorContext(ctx, "failed to send batch to message broker, retrying", log.Err(err), "attempt", n)
		}),
	)

	return nil
}
