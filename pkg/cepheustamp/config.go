package cepheustamp

type Config struct {
	ErrorEstimateScale        uint8
	ErrorEstimateMultiplier   uint8
	ErrorEstimateClockFormat  TimestampClockFormat
	ErrorEstimateSynchronized bool
}
