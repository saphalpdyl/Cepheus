// Dispatcher that listen to the probeDataStream and publishes immediately to NATS
// The previous implementation used to have batching enabled but this didn't seem significant enough
package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"cepheus/common"
	"cepheus/telemetry"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/avast/retry-go"
	"github.com/nats-io/nats.go/jetstream"
)

type Dispatcher struct {
	SerialID string

	probeDataStream *ProbeDataStream
	logger          *slog.Logger

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
	SerialID        string

	JetStreamTimeout time.Duration
}

func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{
		probeDataStream: cfg.ProbeDataStream,
		logger:          cfg.Logger,
		js:              cfg.JetStream,
		jsTimeout:       cfg.JetStreamTimeout,
		SerialID:        cfg.SerialID,
	}
}

func (d *Dispatcher) Start(ctx context.Context, interval time.Duration) (err error) {
	ctx, d.cancel = context.WithCancel(ctx)
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Dispatcher.Start", nil)
	defer end()

	d.done = make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(d.done)
				return
			case data := <-d.probeDataStream.stream:
				if data.ProbeType == "" {
					break
				}

				payload := common.ReportPayload{
					Payload:       data,
					SerialID:      d.SerialID,
					SentTimestamp: time.Now(),
				}

				payloadData, err := json.Marshal(payload)
				if err != nil {
					d.logger.ErrorContext(ctx, "failed to marshal payload data")
					continue
				}

				var subject string

				switch data.ProbeType {
				case api.ProbeTypeStamp:
					subject = fmt.Sprintf("cepheus.probe.stamp.%s", d.SerialID)
				case api.ProbeTypeTrace:
					subject = fmt.Sprintf("cepheus.probe.trace.%s", d.SerialID)
				default:
					d.logger.ErrorContext(ctx, fmt.Sprintf("invalid probe type, got %s", string(data.ProbeType)))
				}

				err = retry.Do(func() error {
					ack, err := d.js.Publish(
						ctx,
						subject,
						payloadData,
					)
					if err != nil {
						d.logger.ErrorContext(ctx, "error while publishing to NAT", "subject", subject, "data", payloadData)
						return err
					}

					if ack.Duplicate {
						d.logger.WarnContext(ctx, "duplicate ACK recieved from NATS", "data", payloadData)
					}

					return nil
				},
					retry.Context(ctx),
					retry.Attempts(3),
					retry.DelayType(retry.BackOffDelay),
					retry.OnRetry(func(n uint, err error) {
						d.logger.ErrorContext(ctx, "failed to send batch to message broker, retrying", log.Err(err), "attempt", n)
					}),
				)

				if err != nil {
					d.logger.ErrorContext(ctx, "retry failed", "subject", subject, "data", payloadData)
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
