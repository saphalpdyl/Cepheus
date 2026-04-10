package cepheustamp

import (
	"errors"
	"time"
)

// NtpUnixOffset is the number of seconds between the NTP epoch (1900-01-01)
// and the Unix epoch (1970-01-01).
const NtpUnixOffset = 2208988800

// TimestampClockFormat identifies the clock format used in a Timestamp.
type TimestampClockFormat string

const (
	ClockFormatNTP TimestampClockFormat = "NTP"
)

// Timestamp is an NTP-format timestamp: 32 bits of seconds since the NTP
// epoch (1900-01-01) followed by 32 bits of fractional seconds.
//
// Support for PTP is planned!
type Timestamp struct {
	Seconds  uint32
	Fraction uint32
}

// TimestampParams configures timestamp generation.
type TimestampParams struct {
	ClockFormat TimestampClockFormat
}

// NewTimestamp returns a Timestamp captured at the current time using the
// clock format specified in p.
func NewTimestamp(p TimestampParams) (*Timestamp, error) {
	now := time.Now()

	if p.ClockFormat == ClockFormatNTP {
		timestamp := generateNtpTimestamp(now)
		return &timestamp, nil
	}

	return nil, errors.New("invalid clock format")
}

func generateNtpTimestamp(t time.Time) Timestamp {
	seconds := uint32(t.Unix() + NtpUnixOffset)

	// NTP fraction: (nanoseconds * 2 ^ 32) / 10 ^9
	nanos := uint64(t.Nanosecond())
	fraction := uint32((nanos << 32) / 1e9)

	return Timestamp{
		Seconds:  seconds,
		Fraction: fraction,
	}
}

// FromTime converts a time.Time into a Timestamp using the specified clock format.
func FromTime(p TimestampParams, t time.Time) (*Timestamp, error) {
	if p.ClockFormat == ClockFormatNTP {
		timestamp := generateNtpTimestamp(t)
		return &timestamp, nil
	}

	return nil, errors.New("invalid clock format")
}
