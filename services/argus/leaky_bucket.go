package argus

import (
	"math"
	"time"
)

type LeakyBucketConfiguration struct {
	OpenThreshold    float64
	CloseThreshold   float64
	DecayPerSecond   float64
	BaseContribution float64
	MagnitudeAlpha   float64
}

type LeakyBucket struct {
	cfg LeakyBucketConfiguration
}

func NewLeakyBucket(cfg LeakyBucketConfiguration) *LeakyBucket {
	return &LeakyBucket{cfg: cfg}
}

// AddFinding mutates the bucket's score in place, returning whether
// thresholds were crossed during this update.
type BucketState struct {
	Score          float64
	ScoreUpdatedAt time.Time
}

type BucketUpdate struct {
	NewState     BucketState
	Contribution float64
	CrossedOpen  bool
	CrossedClose bool
}

func (b *LeakyBucket) Add(state BucketState, value float64, ts time.Time) BucketUpdate {
	elapsed := ts.Sub(state.ScoreUpdatedAt).Seconds()
	decayed := math.Max(0, state.Score-b.cfg.DecayPerSecond*elapsed)
	contribution := b.cfg.BaseContribution + b.cfg.MagnitudeAlpha*math.Log10(1.0+math.Abs(value))
	newScore := decayed + contribution

	return BucketUpdate{
		NewState:     BucketState{Score: newScore, ScoreUpdatedAt: ts},
		Contribution: contribution,
		CrossedOpen:  decayed < b.cfg.OpenThreshold && newScore >= b.cfg.OpenThreshold,
		CrossedClose: decayed >= b.cfg.CloseThreshold && newScore < b.cfg.CloseThreshold,
	}
}

// Decay handles the time-only update for Sweep: no finding, just elapsed time
func (b *LeakyBucket) Decay(state BucketState, ts time.Time) BucketUpdate {
	elapsed := ts.Sub(state.ScoreUpdatedAt).Seconds()
	decayed := math.Max(0, state.Score-b.cfg.DecayPerSecond*elapsed)

	return BucketUpdate{
		NewState:     BucketState{Score: decayed, ScoreUpdatedAt: ts},
		CrossedOpen:  false, // can't cross open by just decaying
		CrossedClose: state.Score >= b.cfg.CloseThreshold && decayed < b.cfg.CloseThreshold,
	}
}
