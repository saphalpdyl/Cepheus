package agent

import (
	"cepheus/api"
	"cepheus/common"
	"cepheus/stamp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"
)

type StampSenderExecutor struct {
	stampConfig stamp.Config
	logger      *slog.Logger
}

func NewStampSenderExecutor(stampCfg stamp.Config, logger *slog.Logger) *StampSenderExecutor {
	return &StampSenderExecutor{
		stampConfig: stampCfg,
		logger:      logger,
	}
}

func (e *StampSenderExecutor) Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (common.ProbeResult, error) {
	p, ok := params.(*api.AgentTaskStampSenderParams)
	if !ok {
		return common.ProbeResult{}, fmt.Errorf("stamp-sender: expected AgentTaskStampSenderParams, got %T", params)
	}

	sender, err := stamp.NewSender(stamp.SenderConfig{
		LocalAddr:  p.SourceIP,
		RemoteAddr: net.JoinHostPort(p.Target, strconv.Itoa(int(p.TargetPort))),
		Timeout:    time.Duration(1) * time.Second,
		Config:     e.stampConfig,
		OnError:    func(err error) { e.logger.ErrorContext(ctx, "stamp sender error", "err", err) },
	})
	if err != nil {
		return common.ProbeResult{}, fmt.Errorf("stamp-sender: create: %w", err)
	}
	defer sender.Close()

	go func() {
		<-ctx.Done()
	}()

	count := p.PacketCount
	if count <= 0 {
		count = 10
	}
	interval := p.PacketInterval // TODO: Rename this to PacketIntervalNs
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	probes := make([]common.StampProbeData, 0, count)
	received := 0

	startTimestamp := time.Now()

	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			return common.ProbeResult{}, ctx.Err()
		}

		// Used by lost probes as a reference for timestamp when stampPacket.SenderTimestamp is not available
		startTimestamp := time.Now()
		pkt, err := sender.Send()
		t4 := time.Now()

		if err != nil {
			// Packet lost
			// TODO: Implement a check for error type in the future instead of an assumption
			// 		that every failure is a timeout error

			probes = append(probes, common.StampProbeData{
				Tx:            startTimestamp,
				IsLost:        true,
				Rx:            time.Time{},
				Rtt:           0,
				ForwardDelay:  0,
				BackwardDelay: 0,
			})

			e.logger.DebugContext(ctx, "stamp packet failed", "seq", i, "err", err)
		} else {
			received++

			// t1: STAMP Packet sent
			// t2: Reflector receives STAMP packet
			// t3: Reflector sends out STAMP packet
			// t4: Sender receives reflector enriched STAMP packet

			t1, err := pkt.SenderTimestamp.ToTime(e.stampConfig.ErrorEstimate.ClockFormat)
			if err != nil {
				e.logger.ErrorContext(ctx, "stamp sender error", "err", err)
				return common.ProbeResult{}, err
			}

			t2, err := pkt.ReceiveTimestamp.ToTime(e.stampConfig.ErrorEstimate.ClockFormat)
			if err != nil {
				e.logger.ErrorContext(ctx, "stamp receive error", "seq", i, "err", err)
				return common.ProbeResult{}, err
			}

			t3, err := pkt.Timestamp.ToTime(e.stampConfig.ErrorEstimate.ClockFormat)
			if err != nil {
				e.logger.ErrorContext(ctx, "stamp receive error", "seq", i, "err", err)
				return common.ProbeResult{}, err
			}

			forwardDelay := t2.Sub(*t1)
			backwardDelay := t4.Sub(*t3)
			rtt := t4.Sub(*t1) - t3.Sub(*t2)

			probes = append(probes, common.StampProbeData{
				Tx:            *t1,
				IsLost:        false,
				Rx:            t4,
				Rtt:           rtt,
				ForwardDelay:  forwardDelay,
				BackwardDelay: backwardDelay,
			})

		}

		if i < count-1 {
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return common.ProbeResult{}, ctx.Err()
			}
		}
	}

	stampData := common.StampData{
		Target:    p.Target,
		Port:      p.TargetPort,
		Sent:      len(probes),
		Received:  received,
		Loss:      float64(len(probes)-received) / float64(len(probes)),
		Probes:    probes,
		Timestamp: startTimestamp,
	}

	marshaledStampData, err := json.Marshal(&stampData)
	if err != nil {
		e.logger.ErrorContext(ctx, "stamp marshal error", "err", err)
		return common.ProbeResult{}, err
	}

	return common.ProbeResult{
		TaskID:    spec.TaskID,
		ProbeType: common.ProbeTypeStamp,
		Kind:      string(spec.Type),
		Timestamp: time.Now(),
		Data:      marshaledStampData,
	}, nil
}
