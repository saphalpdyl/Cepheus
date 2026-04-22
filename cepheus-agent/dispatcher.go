// dispatcher is responsible for batching telemetry data in supervisor's dataStream buffer
// and send it to external message broker ( NATS in this case ) at regular intervals.
package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"cepheus/telemetry"
	"context"
	"log/slog"
	"time"

	"github.com/avast/retry-go"
)

type Dispatcher struct {
	probeDataStream *ProbeDataStream
	logger          *slog.Logger

	// batcher config
	batchSize   uint32
	batchbuffer []api.ProbeResult // a buffer of size `batchSize` to hold telemetry data for retries

	ticker *time.Ticker

	// Shutdown management
	done   chan struct{}
	cancel context.CancelFunc
}

func NewDispatcher(
	probeDataStream *ProbeDataStream,
	logger *slog.Logger,
	batchSize uint32,
) *Dispatcher {
	return &Dispatcher{
		probeDataStream: probeDataStream,
		logger:          logger,
		batchSize:       batchSize,
		batchbuffer:     make([]api.ProbeResult, 0, batchSize),
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
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Dispatcher.dispatch", nil)
	defer end()

	// 1. We mark upto maxBatchSize as ready to be sent
	// 2. We try sending that batch to the message broker
	// 3. If success, we remove them from the buffer
	// 4. If fails, we keep them in the buffer and try agian later

	// Edge cases:
	// - If n is the max batch size and we have m ( m < n ) items in the buffer,
	//   we drain the buffer upto m times and try send them by storing them in dispatcher
	//  local buffer. If it fails, we keep them in the local buffer and try again.
	//  However, if there are >0 items in the probe data buffer by the time we are on our next tick
	//	we will try to fill up the batch before trying to send another.

	// Send all the data in batches into the same subject
	// The go data procesors will unfurl the data and send it to the right place

	err = retry.Do(func() error {
		if len(d.batchbuffer) > 0 {
			// retry
			d.logger.InfoContext(ctx, "retrying to telemetry batche from buffer")

		}
		return nil
	},
		retry.Context(ctx),
		retry.Attempts(10),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			d.logger.ErrorContext(ctx, "failed to send batch to message broker, retrying", log.Err(err), "attempt", n)
		}),
	)

	return nil
}
