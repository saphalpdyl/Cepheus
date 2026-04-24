package stampprocessor

import (
	"cepheus/processors/shared/log"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type StampProcessor struct {
	InstanceId string

	config StampProcessorConfig

	logger *slog.Logger
}

func NewStampProcessor(instanceId string, config StampProcessorConfig, logger *slog.Logger) StampProcessor {
	return StampProcessor{
		InstanceId: instanceId,
		logger:     logger,
		config:     config,
	}
}

func (s *StampProcessor) Start(ctx context.Context) error {
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
				s.logger.InfoContext(ctx, "consumed data", "data", msg.Data())
				msg.Ack()
			}
		}

	}()

	<-ctx.Done()

	return nil
}
