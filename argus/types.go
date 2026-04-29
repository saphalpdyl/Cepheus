package argus

import "time"

type DetectorType string

const (
	TypeEWMA DetectorType = "EWMA"
)

type Sample struct {
	ts  time.Time
	val float64
}

type Detector interface {
	Step(Sample)
	GetType() DetectorType
}
