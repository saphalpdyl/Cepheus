package argus

import (
	"cepheus/services/argus/types"
	"context"
	"math"
	"time"
)

type EwmaConfig struct {
	Alpha     float64
	Threshold float64
	Warmup    int64
	Epsilon   float64

	// Severity
	SeverityAlpha float32
}
type Ewma struct {
	config EwmaConfig
}

type EwmaState struct {
	Mean     float64   `json:"mean"`
	Variance float64   `json:"variance"`
	N        int64     `json:"n"`
	LastSeen time.Time `json:"last_seen"`
}

func NewEmwa(config EwmaConfig) *Ewma {
	return &Ewma{config: config}
}

func (e *Ewma) Step(_ context.Context, state *EwmaState, s types.Sample) *types.Finding {
	if state.N == 0 {
		state.Mean = s.Value
		state.Variance = 0.0
		state.N = 1
		state.LastSeen = s.Timestamp
	}

	stddev := math.Sqrt(state.Variance + e.config.Epsilon)
	z := (s.Value - state.Mean) / stddev

	var finding *types.Finding
	if state.N >= e.config.Warmup && math.Abs(z) >= e.config.Threshold {
		finding = &types.Finding{
			TS:       s.Timestamp,
			Value:    s.Value,
			Severity: 1 + float64(e.config.SeverityAlpha)*math.Log10(1+math.Abs(z)),
			Details:  nil,
		}
	}

	delta := s.Value - state.Mean
	state.Mean += e.config.Alpha * delta
	state.Variance = (1 - e.config.Alpha) * (state.Variance + e.config.Alpha*delta*delta)

	state.N++
	state.LastSeen = s.Timestamp

	// attach finding if it exists
	if finding != nil {
		findingDetails := types.EwmaFindingDetails{
			Z:        z,
			Stddev:   stddev,
			Variance: state.Variance,
			N:        state.N,
		}

		finding.Details = &findingDetails
	}

	return finding
}
