package argus

import (
	"cepheus/services/argus/types"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const (
	priorAlpha = 0.5 // Jeffreys prior keeps the Beta proper before any data
	priorBeta  = 0.5
)

type LossSample struct {
	Sent     int64 `json:"sent"`
	Received int64 `json:"received"`
}

type BetaBinomialState struct {
	Lost     int64     `json:"lost"`
	Received int64     `json:"received"`
	N        int64     `json:"n"`
	LastSeen time.Time `json:"last_seen"`
}

type BetaBinomialConfig struct {
	Threshold float64
	Warmup    int64
}

type BetaBinomial struct {
	config BetaBinomialConfig
}

func NewBetaBinomial(config BetaBinomialConfig) *BetaBinomial {
	return &BetaBinomial{config: config}
}

func (b *BetaBinomial) Step(state json.RawMessage, ts time.Time, value any) (json.RawMessage, *types.Finding, error) {
	v, ok := value.(LossSample)
	if !ok {
		return state, nil, fmt.Errorf("beta binomial: want LossSample, got %T", value)
	}

	var st BetaBinomialState
	if len(state) > 0 {
		if err := json.Unmarshal(state, &st); err != nil {
			return state, nil, err
		}
	}

	finding := b.step(&st, ts, v)

	next, err := json.Marshal(st)
	if err != nil {
		return state, nil, err
	}

	return next, finding, nil
}

func (b *BetaBinomial) step(st *BetaBinomialState, ts time.Time, s LossSample) *types.Finding {
	lost := s.Sent - s.Received
	if lost < 0 {
		lost = 0
	}

	// Posterior params from the baseline BEFORE folding in this batch.
	postAlpha := priorAlpha + float64(st.Lost)
	postBeta := priorBeta + float64(st.Received)

	var finding *types.Finding
	if s.Sent > 0 {
		tail := betaBinomialUpperTail(lost, s.Sent, postAlpha, postBeta)
		totalSent := st.Lost + st.Received

		if totalSent >= b.config.Warmup && lost > 0 && tail <= b.config.Threshold {
			sev := math.Min(12, -math.Log10(math.Max(tail, 1e-12)))
			rate := float64(lost) / float64(s.Sent) // float division, not int
			finding = &types.Finding{
				TS:       ts,
				Value:    rate,
				Severity: sev,
				Details: &types.BetaBinomialFindingDetails{
					Tail:      tail,
					Lost:      lost,
					Sent:      s.Sent,
					Rate:      rate,
					PostAlpha: postAlpha,
					PostBeta:  postBeta,
				},
			}
		}
	}

	// Accumulate this batch into the baseline.
	st.Lost += lost
	st.Received += s.Received
	st.N++
	st.LastSeen = ts

	return finding
}

// betaBinomialUpperTail returns P(X >= k) for X ~ BetaBinomial(n, a, b).
func betaBinomialUpperTail(k, n int64, a, bb float64) float64 {
	if k <= 0 {
		return 1.0
	}
	if k > n {
		return 0.0
	}
	logBetaAB := logBeta(a, bb)
	var tail float64
	for i := k; i <= n; i++ {
		fi := float64(i)
		logPmf := logChoose(n, i) +
			logBeta(fi+a, float64(n)-fi+bb) -
			logBetaAB
		tail += math.Exp(logPmf)
	}
	if tail > 1.0 {
		tail = 1.0
	}
	return tail
}

func logBeta(a, b float64) float64 {
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	return la + lb - lab
}

func logChoose(n, k int64) float64 {
	ln1, _ := math.Lgamma(float64(n + 1))
	lk1, _ := math.Lgamma(float64(k + 1))
	lnk1, _ := math.Lgamma(float64(n-k) + 1)
	return ln1 - lk1 - lnk1
}
