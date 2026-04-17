package cepheusagent

import (
	"cepheus/api"
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

func (e *StampSenderExecutor) Execute(ctx context.Context, params api.TaskParams) (api.ProbeResult, error) {
	p, ok := params.(*api.AgentTaskStampSenderParams)
	if !ok {
		return api.ProbeResult{}, fmt.Errorf("stamp-sender: expected AgentTaskStampSenderParams, got %T", params)
	}

	sender, err := stamp.NewSender(stamp.SenderConfig{
		LocalAddr:  p.SourceIP,
		RemoteAddr: net.JoinHostPort(p.Target, strconv.Itoa(int(p.TargetPort))),
		Timeout:    time.Duration(10) * time.Second,
		Config:     e.stampConfig,
		OnError:    func(err error) { e.logger.ErrorContext(ctx, "stamp sender error", "err", err) },
	})
	if err != nil {
		return api.ProbeResult{}, fmt.Errorf("stamp-sender: create: %w", err)
	}
	defer sender.Close()

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
			return api.ProbeResult{}, ctx.Err()
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
				return api.ProbeResult{}, err
			}
			rtts = append(rtts, *rtt)
		}

		if i < count-1 {
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return api.ProbeResult{}, ctx.Err()
			}
		}
	}

	stats := computeRTTStats(rtts)
	return api.ProbeResult{
		Kind:      "stamp-sender",
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
	}, nil
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
