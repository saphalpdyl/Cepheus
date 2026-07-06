package argus

import (
	"cepheus/libs/common"
	"cepheus/services/argus/types"
)

type Extractor struct {
	Detectors  []types.DetectorType
	Extract    func(any) (any, error)
	MetricName string
}

type PipelineRegistry struct {
	entries map[types.SeriesType][]Extractor
}

func NewPipelineRegistry(defaultVals map[types.SeriesType][]Extractor) *PipelineRegistry {
	if defaultVals == nil {
		defaultVals = make(map[types.SeriesType][]Extractor)
	}

	return &PipelineRegistry{
		entries: defaultVals,
	}
}

func (p *PipelineRegistry) GetExtractors(st types.SeriesType) []Extractor {
	return p.entries[st]
}

// CreateDefaultRegistry create the default Cepheus' registry
// Each seriesType fans out to multiple Extractor functions that again fan out to multiple detectors
/*
 *                        ┌───────►ExtractRTTP95Ns()───────►EWMA
 *                        │
 *                        │
 *                        │                             ┌──►EWMA
 * SeriesType.STAMP───────┼───────►ExtractFwdP95Ns()────┤
 *                        │                             └──►BOCPD
 *                        │
 *                        │
 *                        └──────► ExtractLoss()───────────►FREQ
 */
func CreateDefaultRegistry() *PipelineRegistry {
	defaultEntries := map[types.SeriesType][]Extractor{
		types.SeriesTypeStamp: {
			{
				MetricName: "fwd_p50_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.FwdP50Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "bwd_p50_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.BwdP50Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "rtt_p50_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.RttP50Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "loss",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return LossSample{
						Sent:     m.Sent,
						Received: m.Received,
					}, nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeBetaB},
			},
		},
		types.SeriesTypePing: {
			{
				MetricName: "rtt_p50_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.PingMetrics)
					return float64(m.RttP50Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "packet_loss_percent",
				Extract: func(data any) (any, error) {
					m := data.(common.PingMetrics)
					return LossSample{
						Sent:     m.Sent,
						Received: m.Received,
					}, nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeBetaB},
			},
		},
		types.SeriesTypeTrace: {
			{
				MetricName: "asn_path_hash",
				Extract: func(data any) (any, error) {
					m := data.(common.TraceMetrics)
					return m.AsnPathHash, nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeFreq},
			},
			{
				MetricName: "link_path_hash",
				Extract: func(data any) (any, error) {
					m := data.(common.TraceMetrics)
					return m.LinkPathHash, nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeFreq},
			},
		},
	}

	return NewPipelineRegistry(defaultEntries)
}
