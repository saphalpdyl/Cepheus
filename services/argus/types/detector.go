package types

import (
	"encoding/json"
	"time"
)

type DetectorType string

const (
	DetectorTypeEwma  DetectorType = "EWMA"
	DetectorTypeFreq  DetectorType = "FREQ"
	DetectorTypeBetaB DetectorType = "BETA"
	DetectorTypeMdmt  DetectorType = "MDMT"
)

type FindingDetails interface {
	DetectorName() string
}

type Finding struct {
	TS       time.Time
	Value    float64
	Severity float64
	Details  FindingDetails
}

type Sample[T any] struct {
	Timestamp time.Time
	Value     T
}

// EwmaFindingDetails  details
type EwmaFindingDetails struct {
	Z        float64
	Stddev   float64
	Variance float64
	N        int64
}

func (e *EwmaFindingDetails) DetectorName() string { return string(DetectorTypeEwma) }

// BetaBinomialFindingDetails details
type BetaBinomialFindingDetails struct {
	Tail      float64
	Lost      int64
	Sent      int64
	Rate      float64
	PostAlpha float64
	PostBeta  float64
}

func (b *BetaBinomialFindingDetails) DetectorName() string { return string(DetectorTypeBetaB) }


type FreqFindingDetails struct {
	Probability float64
	CurrentCount int64
	TotalCount int64
}

func (f *FreqFindingDetails) DetectorName() string { return string(DetectorTypeFreq) }

// Detector is the one interface the worker talks to. It is deliberately not
// generic: the worker hands over the prior baseline state and an opaque value,
// and each detector asserts the concrete type it expects on line one. That
// single assertion is what lets the same loop drive float64, string, or (later)
// object-valued samples without any type machinery leaking into the pipeline.
type Detector interface {
	// Step folds one sample into the baseline. It returns the new state to
	// persist and a finding if the sample tripped the detector.
	Step(state json.RawMessage, ts time.Time, value any) (json.RawMessage, *Finding, error)
}
