package cepheustamp

// Packet represents a STAMP test packet (unauthenticated mode).
//
// Field layout and semantics follow RFC 8762 §4.2.1.
type Packet struct {
	SequenceNumber uint32
	Timestamp      Timestamp
	ErrorEstimate  uint16

	// Reflector-only fields (MBZ in ).
	ReceiveTimestamp     Timestamp
	SenderSequenceNumber uint32
	SenderTimestamp      Timestamp
	SenderErrorEstimate  uint16
	SenderTTL            uint8
}

func Encode(p *Packet) ([]byte, error) {
	panic("not implemented")
}

func Decode(b []byte) (*Packet, error) {
	panic("not implemented")
}
