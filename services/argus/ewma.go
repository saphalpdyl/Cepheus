package argus

import (
	"cepheus/services/argus/types"
	"encoding/json"
	"fmt"
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

// Step satisfies types.Detector. It unwraps the JSON baseline state, asserts
// the float64 it needs, runs the math, and hands back the state to persist.
func (e *Ewma) Step(state json.RawMessage, ts time.Time, value any) (json.RawMessage, *types.Finding, error) {
	v, ok := value.(float64)
	if !ok {
		return state, nil, fmt.Errorf("ewma: want float64, got %T", value)
	}

	var st EwmaState
	if len(state) > 0 {
		if err := json.Unmarshal(state, &st); err != nil {
			return state, nil, err
		}
	}

	finding := e.step(&st, ts, v)

	next, err := json.Marshal(st)
	if err != nil {
		return state, nil, err
	}

	return next, finding, nil
}

// step is the pure EWMA math, mutating state in place.
func (e *Ewma) step(state *EwmaState, ts time.Time, value float64) *types.Finding {
	if state.N == 0 {
		state.Mean = value
		state.Variance = 0.0
		state.N = 1
		state.LastSeen = ts
	}

	stddev := math.Sqrt(state.Variance + e.config.Epsilon)
	z := (value - state.Mean) / stddev

	var finding *types.Finding
	if state.N >= e.config.Warmup && math.Abs(z) >= e.config.Threshold {
		finding = &types.Finding{
			TS:       ts,
			Value:    value,
			Severity: 1 + float64(e.config.SeverityAlpha)*math.Log10(1+math.Abs(z)),
			Details:  nil,
		}
	}

	delta := value - state.Mean
	state.Mean += e.config.Alpha * delta
	state.Variance = (1 - e.config.Alpha) * (state.Variance + e.config.Alpha*delta*delta)

	state.N++
	state.LastSeen = ts

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
