package stamp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// SenderPacket is an unauthenticated STAMP sender test packet (RFC 8762 §3.2).
// The wire format is 44 octets: sequence number, timestamp, error estimate,
// and 30 octets of MBZ (must be zero) padding.
type SenderPacket struct {
	SequenceNumber uint32
	Timestamp      Timestamp
	ErrorEstimate  uint16
}

// Encode serializes the SenderPacket into a 44-octet buffer.
// hmacKey must be nil; authenticated mode is not yet supported.
func (p *SenderPacket) Encode(hmacKey []byte) ([]byte, error) {
	if hmacKey != nil {
		// Auth mode not supported
		panic("not implemented")
	}

	buf := make([]byte, 44)

	binary.BigEndian.PutUint32(buf[0:4], p.SequenceNumber)
	binary.BigEndian.PutUint32(buf[4:8], p.Timestamp.Seconds)
	binary.BigEndian.PutUint32(buf[8:12], p.Timestamp.Fraction)
	binary.BigEndian.PutUint16(buf[12:14], p.ErrorEstimate)

	return buf, nil
}

// DecodeSenderPacket parses a 44-octet buffer into a SenderPacket.
// hmacKey must be nil; authenticated mode is not yet supported.
func DecodeSenderPacket(hmacKey []byte, b []byte) (*SenderPacket, error) {
	if hmacKey != nil {
		// Auth mode not supported
		panic("not implemented")
	}

	if len(b) < 44 {
		return nil, errors.New("packet too short")
	}

	p := &SenderPacket{
		SequenceNumber: binary.BigEndian.Uint32(b[0:4]),
		Timestamp: Timestamp{
			Seconds:  binary.BigEndian.Uint32(b[4:8]),
			Fraction: binary.BigEndian.Uint32(b[8:12]),
		},
		ErrorEstimate: binary.BigEndian.Uint16(b[12:14]),
	}

	return p, nil
}

// ReflectorPacket is an unauthenticated STAMP reflector response packet
// (RFC 8762 §4.2). It echoes the sender's fields and adds the reflector's
// own timestamps, totalling 44 octets on the wire.
type ReflectorPacket struct {
	SequenceNumber       uint32
	Timestamp            Timestamp
	ErrorEstimate        uint16
	ReceiveTimestamp     Timestamp
	SenderSequenceNumber uint32
	SenderTimestamp      Timestamp
	SenderErrorEstimate  uint16
	SenderTTL            uint8
}

// Encode serializes the ReflectorPacket into a 44-octet buffer.
// hmacKey must be nil; authenticated mode is not yet supported.
func (p *ReflectorPacket) Encode(hmacKey []byte) ([]byte, error) {
	if hmacKey != nil {
		// Auth mode not supported
		panic("not implemented")
	}

	buf := make([]byte, 44)

	binary.BigEndian.PutUint32(buf[0:4], p.SequenceNumber)
	binary.BigEndian.PutUint32(buf[4:8], p.Timestamp.Seconds)
	binary.BigEndian.PutUint32(buf[8:12], p.Timestamp.Fraction)
	binary.BigEndian.PutUint16(buf[12:14], p.ErrorEstimate)

	// 14-15: MBZ

	binary.BigEndian.PutUint32(buf[16:20], p.ReceiveTimestamp.Seconds)
	binary.BigEndian.PutUint32(buf[20:24], p.ReceiveTimestamp.Fraction)
	binary.BigEndian.PutUint32(buf[24:28], p.SenderSequenceNumber)
	binary.BigEndian.PutUint32(buf[28:32], p.SenderTimestamp.Seconds)
	binary.BigEndian.PutUint32(buf[32:36], p.SenderTimestamp.Fraction)
	binary.BigEndian.PutUint16(buf[36:38], p.SenderErrorEstimate)

	// 38-39: MBZ

	buf[40] = p.SenderTTL

	// 41-43: MBZ

	return buf, nil
}

// DecodeReflectorPacket parses a 44-octet buffer into a ReflectorPacket.
// hmacKey must be nil; authenticated mode is not yet supported.
func DecodeReflectorPacket(hmacKey []byte, data []byte) (*ReflectorPacket, error) {
	if hmacKey != nil {
		// Auth mode not supported
		panic("not implemented")
	}

	if len(data) < 44 {
		return nil, fmt.Errorf("packet too short: got %d bytes, need 44", len(data))
	}

	p := &ReflectorPacket{
		SequenceNumber: binary.BigEndian.Uint32(data[0:4]),
		Timestamp: Timestamp{
			Seconds:  binary.BigEndian.Uint32(data[4:8]),
			Fraction: binary.BigEndian.Uint32(data[8:12]),
		},
		ErrorEstimate: binary.BigEndian.Uint16(data[12:14]),
		// 14-15: MBZ
		ReceiveTimestamp: Timestamp{
			Seconds:  binary.BigEndian.Uint32(data[16:20]),
			Fraction: binary.BigEndian.Uint32(data[20:24]),
		},
		SenderSequenceNumber: binary.BigEndian.Uint32(data[24:28]),
		SenderTimestamp: Timestamp{
			Seconds:  binary.BigEndian.Uint32(data[28:32]),
			Fraction: binary.BigEndian.Uint32(data[32:36]),
		},
		SenderErrorEstimate: binary.BigEndian.Uint16(data[36:38]),
		// 38-39: MBZ
		SenderTTL: data[40],
		// 41-43: MBZ
	}

	return p, nil
}
