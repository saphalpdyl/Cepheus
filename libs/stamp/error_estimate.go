package stamp

import "errors"

// ErrorEstimate represents the error estimate field of a STAMP packet
// as defined in RFC 8762 §4.2. The estimated error equals
// Multiplier * 2^(-32) * 2^Scale seconds.
type ErrorEstimate struct {

	// Synchronization bit, 1 indicates that the clock is synchronized
	Sync bool

	// Clock format specifies NTP or PTP ( currently only NTP is supported )
	ClockFormat TimestampClockFormat

	// RFC 8186 defines Z Field to be interpreted as:
	// 0:	NTP 64-bit format of a timestamp
	// 1: 	PTPv2 truncated format of a timestamp
	ZBit bool

	// The error estimate is equal to Multiplier*2^(-32)*2^Scale (in seconds).

	// Scale factor ( 0-63 )
	Scale uint8

	// Multiplier factor ( 0-255 )
	Multiplier uint8
}

// ErrorEstimateDecode parses a 2-octet error estimate field into an ErrorEstimate.
func ErrorEstimateDecode(val uint16) ErrorEstimate {
	zBit := (val & 0x4000) != 0

	return ErrorEstimate{
		Sync:        (val & 0x8000) != 0,
		ZBit:        zBit,
		Scale:       uint8((val & 0x3F00) >> 8),
		Multiplier:  uint8(val & 0xFF),
		ClockFormat: ClockFormatNTP, // TODO: Change this to a conditional after PTP is supported
	}
}

// Encode serializes the ErrorEstimate into its 2-octet wire format.
func (e *ErrorEstimate) Encode() uint16 {
	var val uint16

	//	0                   1
	//	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
	//
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |S|Z|   Scale   |   Multiplier  |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	if e.Sync {
		val |= uint16(0x8000)
	}

	val |= uint16(0x0) // TODO: Change this to a conditional after PTP is supported
	val |= (uint16(e.Scale) & 0x3F) << 8
	val |= uint16(e.Multiplier)

	return val
}

// NewErrorEstimate creates an ErrorEstimate with validated parameters.
// Scale must be in the range [0, 63].
func NewErrorEstimate(
	synchronized bool,
	clockFormat TimestampClockFormat,
	scale uint8,
	multiplier uint8,
) (*ErrorEstimate, error) {

	if scale > 63 {
		return nil, errors.New("Scale out of range: 0 < scale < 63")
	}

	return &ErrorEstimate{
		Sync:        synchronized,
		Scale:       scale,
		Multiplier:  multiplier,
		ZBit:        false,
		ClockFormat: clockFormat,
	}, nil
}
