package agent

import (
	agentv1 "cepheus/libs/api/gen/cepheus/agent/v1"
	"cepheus/libs/api/gen/cepheus/agent/v1/agentv1connect"
	goscamper "cepheus/libs/scamper-client"
	"cepheus/libs/stamp"
	"cepheus/libs/telemetry"
	"cepheus/services/agent/log"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/avast/retry-go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// The control/management plane for agent
// - Connects to the control plane (server), and resolves configuration
// - Manages lifecycle of probes in goroutines, and restarts them if they fail
// - Reports batched probe results to the control plane at assigned intervals

type Agent struct {
	SerialId           string
	generation         int
	controlPlaneConfig ControlPlaneConfig

	probeDataStream *ProbeDataStream

	lastConfigurationPulled time.Time

	mu          sync.RWMutex
	agentConfig *AgentConfig

	// initial configuration
	scamperBinPath string

	logger *slog.Logger

	// configClient is the Connect client to the control plane's AgentConfigService.
	configClient agentv1connect.AgentConfigServiceClient
}

type AgentInitConfig struct {
	SerialId           string
	ControlPlaneConfig ControlPlaneConfig
	ScamperBinPath     string

	Logger *slog.Logger
}

func NewAgent(cfg AgentInitConfig) *Agent {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	return &Agent{
		SerialId:           cfg.SerialId,
		generation:         0,
		controlPlaneConfig: cfg.ControlPlaneConfig,
		logger:             cfg.Logger,
		scamperBinPath:     cfg.ScamperBinPath,
		probeDataStream:    NewProbeDataStream(100),
		// Connect derives the route from the service/method, so only the base URL
		// of the control plane is needed here.
		configClient: agentv1connect.NewAgentConfigServiceClient(
			httpClient,
			cfg.ControlPlaneConfig.ControlPlane.URL,
		),
	}
}

func (a *Agent) Run(ctx context.Context) (err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.Run", &err)
	defer end()

	err = a.pullAgentConfiguration(ctx)
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to pull agent configuration", log.Err(err))
		return err
	}

	// Dispatcher
	nc, err := nats.Connect(
		a.agentConfig.ReportEndpoint,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(100),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to connect to NATS server", log.Err(err), "endpoint", a.agentConfig.ReportEndpoint)
		return err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to create a JetStream context", log.Err(err))
		return err
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "PROBE_STAMP",
		Description: "Stream for STAMP probe data",
		Subjects:    []string{"cepheus.probe.stamp.>"},
	})

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        "PROBE_TRACE",
		Description: "Stream for Trace probe data",
		Subjects:    []string{"cepheus.probe.trace.>"},
	})

	if err != nil {
		a.logger.ErrorContext(ctx, "couldn't create or update stream", log.Err(err))
	}

	dispatcher := NewDispatcher(
		DispatcherConfig{
			ProbeDataStream:  a.probeDataStream,
			Logger:           a.logger.With(log.Domain(log.DomainDispatcher)),
			BatchSize:        10,
			JetStream:        js,
			JetStreamTimeout: time.Duration(a.agentConfig.ReportTimeoutSeconds) * time.Second,
			SerialID:         a.SerialId,
			GetAgentConfig: func() string {
				a.mu.RLock()
				defer a.mu.RUnlock()

				return a.agentConfig.ID
			},
		},
	)
	err = dispatcher.Start(ctx, time.Duration(a.agentConfig.ReportTimeoutSeconds)*time.Second)
	if err != nil {
		a.logger.ErrorContext(ctx, "error starting dispatcher", log.Err(err))
		return err
	}
	defer dispatcher.Stop()

	scamper, err := goscamper.NewClient(
		goscamper.ScamperClientConfig{
			BinPath:    a.scamperBinPath,
			SocketPath: "/tmp/scamper.sock",
			PPS:        uint32(a.agentConfig.ScamperPPS),
			Format:     goscamper.ScamperFormatJSON,
		},
	)
	if err != nil {
		var scamperError goscamper.ConfigError
		if errors.Is(err, &scamperError) {
			a.logger.ErrorContext(ctx, err.Error())
		} else {
			a.logger.ErrorContext(ctx, "error initializing scamper-client client", log.Err(err))
		}

		return err
	}

	err = scamper.Start(ctx)
	if err != nil {
		a.logger.ErrorContext(ctx, "cannot start scamper-client", log.Err(err))
		return err
	}

	// TODO: TEMP stuff
	stampConfig := stamp.Config{
		ErrorEstimate: stamp.ErrorEstimateConfig{
			ClockFormat:  stamp.ClockFormatNTP,
			Multiplier:   1,
			Scale:        22,
			Synchronized: true,
		},
	}

	supervisor := NewSupervisor(SupervisorConfig{
		Ctx:             ctx,
		Scamper:         scamper,
		Logger:          a.logger,
		ProbeDataStream: a.probeDataStream,
		Executors: map[TaskType]Executor{
			TaskTypeStampSender: NewStampSenderExecutor(
				stampConfig,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(string(TaskTypeStampSender))),
			),
			TaskTypeStampReflector: NewStampReflectorExecutor(
				stampConfig,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(string(TaskTypeStampReflector))),
			),
			TaskTypeTrace: NewTraceExecutor(
				scamper,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(string(TaskTypeTrace))),
			),
			TaskTypePing: NewPingExecutor(
				scamper,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(string(TaskTypePing))),
			),
		},
	})

	a.logger.InfoContext(ctx, "started supervisor")
	supervisor.SetDesiredTasks(a.agentConfig.Tasks)

	go func() {
		ticker := time.NewTicker(time.Duration(a.controlPlaneConfig.ControlPlane.ConfigPullIntervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := a.pullAgentConfiguration(ctx)
				if err != nil {
					continue
				}

				a.lastConfigurationPulled = time.Now()

				supervisor.SetDesiredTasks(a.agentConfig.Tasks)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	err = scamper.Stop()
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to stop scamper-client", log.Err(err))
	}

	return nil
}

func (a *Agent) pullAgentConfiguration(ctx context.Context) error {
	ctx, end, span := telemetry.SpanWithErr(ctx, "Agent.pullAgentConfiguration", nil)
	defer end()

	var agentConfig *AgentConfig

	err := retry.Do(
		func() error {
			var err error

			agentConfig, err = a.pullConfiguration(ctx)

			if err != nil {
				span.AddEvent("agent.config.pull failed")
			}

			return err
		},
		retry.Context(ctx),
		retry.Attempts(3),
		retry.Delay(5*time.Second),
	)

	if err != nil {
		a.logger.ErrorContext(ctx, "error with pulling agent configuration")
		return err
	}

	if a.generation == 0 {
		a.mu.Lock()
		a.agentConfig = agentConfig
		a.mu.Unlock()
	}

	return nil
}

func (a *Agent) pullConfiguration(ctx context.Context) (config *AgentConfig, err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.PullConfiguration", &err)
	defer end()

	resp, err := a.configClient.GetAgentConfig(ctx, connect.NewRequest(&agentv1.GetAgentConfigRequest{
		SerialId: a.SerialId,
	}))
	if err != nil {
		a.logger.Error("failed to fetch configuration", log.Err(err))
		return nil, err
	}

	configResult, err := agentConfigFromProto(resp.Msg)
	if err != nil {
		a.logger.Error("failed to decode agent configuration", log.Err(err))
		return nil, err
	}

	a.logger.Info("configuration pulled", "serial_id", a.SerialId,
		"generation", configResult.Generation, "tasks", len(configResult.Tasks))
	return configResult, nil
}
