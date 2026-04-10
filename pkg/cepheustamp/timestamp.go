package cepheustamp

import "time"

// Timestamp is an NTP-format timestamp: 32 bits of seconds since the NTP
// epoch (1900-01-01) followed by 32 bits of fractional seconds.
type Timestamp struct {
	Seconds  uint32
	Fraction uint32
}

func Now() Timestamp {
	panic("not implemented")
}

func FromTime(t time.Time) Timestamp {
	panic("not implemented")
}

func (ts Timestamp) Time() time.Time {
	panic("not implemented")
}
