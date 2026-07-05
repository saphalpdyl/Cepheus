package argus

import (
	"cepheus/services/argus/types"
	"encoding/json"
	"fmt"
	"time"
)

type FreqState struct {
	Entries map[string]int `json:"entries"`
	N int64 `json:"n"`
	LastSeen int64 `json:"last_seen"`

	TotalEntries int64 `json:"total_entries"`
}

type FreqDetectorConfig struct {
	Warmup int
	GoodThresholdPercent float64 // percentage below which we consider it an anomaly
}

type FreqDetector struct {
	config FreqDetectorConfig
}

func NewFreqDetector(config FreqDetectorConfig) *FreqDetector {
	return &FreqDetector{config: config}
}

func (f *FreqDetector) Step(state json.RawMessage, ts time.Time, value any) (json.RawMessage, *types.Finding, error) {
	v, ok := value.(string)
	if !ok {
		return state, nil, fmt.Errorf("freq detector: want string, got %T", value)
	}

	var st FreqState
	if len(state) > 0 {
		if err := json.Unmarshal(state, &st); err != nil {
			return state, nil, err
		}
	}

	entry, ok := st.Entries[v]
	if !ok {
		st.Entries[v] = 1
	} else {
		st.Entries[v] = entry + 1
	}
	st.TotalEntries++

	prob := float64(st.Entries[v]) / float64(st.TotalEntries)
	if prob < f.GoodThresholdPercent && st.N >= int64(f.config.Warmup) {
		finding := &types.Finding{
			TS: ts,
			Value:     prob,
			Severity:  12 * (1.0 - prob), // crunches to range 0-12
			Details: &types.FreqFindingDetails{
				Probability:  prob,
				CurrentCount: int64(st.Entries[v]),
				TotalCount:   st.TotalEntries,
			},
		}
		st.N++
		st.LastSeen = ts.Unix()
		next, err := json.Marshal(st)
		if err != nil {
			return state, nil, err
		}

		return next, finding, nil
	}

	return state, nil, nil
} 

