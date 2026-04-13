package cepheusstamp

type ErrorEstimateConfig struct {
	Scale        uint8
	Multiplier   uint8
	ClockFormat  TimestampClockFormat
	Synchronized bool
}

type Config struct {
	ErrorEstimate ErrorEstimateConfig
}
