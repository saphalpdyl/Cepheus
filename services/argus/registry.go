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

func CreateDefaultRegistry() *PipelineRegistry {
	defaultEntries := map[types.SeriesType][]Extractor{
		types.SeriesTypeStamp: {
			{
				MetricName: "fwd_p95_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.FwdP95Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "bwd_p95_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.BwdP95Ns), nil
				},
				Detectors: []types.DetectorType{types.DetectorTypeEwma},
			},
			{
				MetricName: "rtt_p95_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.StampMetrics)
					return float64(m.RttP95Ns), nil
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
				MetricName: "rtt_p95_ns",
				Extract: func(data any) (any, error) {
					m := data.(common.PingMetrics)
					return float64(m.RttP95Ns), nil
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
