package types

import "time"

type FindingDetails interface {
	DetectorName() string
}

type Finding struct {
	TS       time.Time
	Value    float64
	Severity float64
	Details  FindingDetails
}

type Sample[T comparable] struct {
	Timestamp time.Time
	Value     T
}

// Finding details
type EwmaFindingDetails struct {
	Z        float64
	Stddev   float64
	Variance float64
	N        int64
}

func (e *EwmaFindingDetails) DetectorName() string { return "ewma" }
