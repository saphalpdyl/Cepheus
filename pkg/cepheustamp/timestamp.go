package cepheustamp

import (
	"errors"
	"time"
)

const NtpUnixOffset = 2208988800

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

type TimestampParams struct {
	ClockFormat TimestampClockFormat
}

// Generate current timestamp with specified clock format
func Now(p TimestampParams) (*Timestamp, error) {
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

func FromTime(p TimestampParams, t time.Time) (*Timestamp, error) {
	if p.ClockFormat == ClockFormatNTP {
		timestamp := generateNtpTimestamp(t)
		return &timestamp, nil
	}

	return nil, errors.New("invalid clock format")
}
