package cepheusagent

import (
	"cepheus/common"
	"context"
	"log/slog"
)

type ProbeDataStream struct {
	stream chan common.ProbeResult

	logger *slog.Logger
}

func NewProbeDataStream(streamSize uint32) *ProbeDataStream {
	return &ProbeDataStream{
		stream: make(chan common.ProbeResult, streamSize),
	}
}

func (p *ProbeDataStream) Insert(ctx context.Context, data common.ProbeResult) bool {
	select {
	case p.stream <- data:
		return true
	case <-ctx.Done():
		return false
	default:
		p.logger.WarnContext(ctx, "probe buffer full, dropping result")
		return false
	}
}

func (p *ProbeDataStream) Pull(ctx context.Context, n int) *[]common.ProbeResult {
	buf := make([]common.ProbeResult, 0, n)

	for range n {
		select {
		case <-ctx.Done():
			return nil
		case data := <-p.stream:
			buf = append(buf, data)
		default:
			return &buf
		}
	}

	return &buf
}
