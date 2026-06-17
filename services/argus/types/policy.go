package types

type SeriesType string

const (
	SeriesTypeStamp SeriesType = "STAMP"
	SeriesTypePing  SeriesType = "PING"
	SeriesTypeTrace SeriesType = "TRACE"
)

type SeriesKey struct {
	Type     SeriesType
	SerialId string
	Target   string
	SrcIP    string
	Port     int32
	Metric   string
	Detector DetectorType
}
