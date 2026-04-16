package cepheusagent

import (
	"math/rand"
	"time"
)

func computeJitter(interval time.Duration, jitterPerc int) time.Duration {
	if jitterPerc <= 0 {
		return interval
	}

	maxJitter := (interval * time.Duration(jitterPerc)) / 100
	jitter := time.Duration(rand.Int63n(int64(maxJitter)*2)) - maxJitter

	return interval + jitter
}
