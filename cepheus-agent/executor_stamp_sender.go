package cepheusagent

import (
	"cepheus/api"
	"cepheus/common"
	"cepheus/stamp"
	"context"
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

	rtts := make([]time.Duration, 0, count)
	sent := 0

	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			// Compute from whatever we have collected, sent >= half of Packetcount
			if sent < (p.PacketCount / 2) {
				return common.ProbeResult{}, nil
			}
			return computeProbeResult(rtts, spec, sent, p), ctx.Err()
		}

		pkt, err := sender.Send()
		t4 := time.Now()
		sent++
		if err != nil {
			e.logger.DebugContext(ctx, "stamp packet failed", "seq", i, "err", err)
		} else {
			rtt, err := computeRTT(pkt, t4, e.stampConfig.ErrorEstimate.ClockFormat)
			if err != nil {
				e.logger.ErrorContext(ctx, "error computing RTT for sender")
				return common.ProbeResult{}, err
			}
			rtts = append(rtts, *rtt)
		}

		if i < count-1 {
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return common.ProbeResult{}, ctx.Err()
			}
		}
	}

	return computeProbeResult(rtts, spec, sent, p), nil
}

func computeProbeResult(rtts []time.Duration, spec *api.Task, sent int, p *api.AgentTaskStampSenderParams) common.ProbeResult {
	stats := computeRTTStats(rtts)
	return common.ProbeResult{
		TaskID:    spec.TaskID,
		ProbeType: common.ProbeTypeStamp,
		Kind:      string(spec.Type),
		Timestamp: time.Now(),
		Data: map[string]any{
			"target":   p.Target,
			"port":     p.TargetPort,
			"sent":     sent,
			"received": len(rtts),
			"loss":     float64(sent-len(rtts)) / float64(sent),
			"avg_rtt":  stats.Avg,
			"min_rtt":  stats.Min,
			"max_rtt":  stats.Max,
			"p50_rtt":  stats.P50,
			"p95_rtt":  stats.P95,
		},
	}
}

func computeRTT(pkt *stamp.ReflectorPacket, t4 time.Time, clockFormat stamp.TimestampClockFormat) (*time.Duration, error) {
	t1, err := pkt.SenderTimestamp.ToTime(clockFormat)
	if err != nil {
		return nil, err
	}
	t2, err := pkt.ReceiveTimestamp.ToTime(clockFormat)
	if err != nil {
		return nil, err
	}

	t3, err := pkt.Timestamp.ToTime(clockFormat)
	if err != nil {
		return nil, err
	}

	rtt := t4.Sub(*t1) - t3.Sub(*t2)
	return &rtt, nil
}

type rttStats struct {
	Avg, Min, Max, P50, P95 time.Duration
}

func computeRTTStats(rtts []time.Duration) rttStats {
	if len(rtts) == 0 {
		return rttStats{}
	}

	sorted := make([]time.Duration, len(rtts))
	copy(sorted, rtts)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}

	var sum time.Duration
	for _, r := range sorted {
		sum += r
	}

	return rttStats{
		Avg: sum / time.Duration(len(sorted)),
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		P50: sorted[len(sorted)*50/100],
		P95: sorted[len(sorted)*95/100],
	}
}
