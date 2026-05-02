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

type Sample struct {
	Timestamp time.Time
	Value     float64
}
