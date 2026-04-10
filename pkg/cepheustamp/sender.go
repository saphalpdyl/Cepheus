package cepheustamp

import (
	"errors"
	"net"
)

// Packet format for unauth mode ( RFC 8762 )
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                        Sequence Number                        |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                          Timestamp                            |
//    |                                                               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |         Error Estimate        |                               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+                               +
//    |                                                               |
//    |                                                               |
//    |                        MBZ  (30 octets)                       |
//    |                                                               |
//    |                                                               |
//    |                                                               |
//    |                                                               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// SenderConfig holds the parameters needed to construct a Sender.
type SenderConfig struct {
	LocalAddr   string
	RemoteAddr  string
	HMACKey     *[]byte
	ClockFormat TimestampClockFormat
	Config      Config
}

// Sender originates STAMP test packets and matches reflected replies against
// outstanding sequence numbers.
type Sender struct {
	Conn        *net.UDPConn
	HMACKey     *[]byte
	seq         uint32
	ClockFormat TimestampClockFormat
	Config      Config
}

// NewSender resolves the local and remote UDP addresses from cfg, opens a
// connected UDP socket, and returns a ready-to-use Sender.
func (s *Sender) NewSender(cfg SenderConfig) (*Sender, error) {
	localAddr, err := net.ResolveUDPAddr("udp", cfg.LocalAddr)
	if err != nil {
		return nil, err
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", cfg.RemoteAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		return nil, err
	}

	if cfg.ClockFormat != ClockFormatNTP {
		return nil, errors.New("Invalid clock format: valid options are NTP")
	}

	return &Sender{
		Conn:    conn,
		HMACKey: cfg.HMACKey,
		seq:     0,
		Config:  cfg.Config,
	}, nil
}

// Send transmits a single STAMP test packet (RFC 8762 §3.2) and blocks
// until the reflected response arrives or the connection's read deadline
// is exceeded.
//
// On success it returns the decoded ReflectorPacket containing the
// reflector's receive/send timestamps and the echoed sender fields.
// The internal sequence number is incremented after each successful send.
//
// Only unauthenticated mode is currently supported; Send panics if
// HMACKey is set.
func (s *Sender) Send() (uint32, error) {
	if s.HMACKey != nil {
		panic("not implemented")
	}

	timestamp, err := NewTimestamp(TimestampParams{
		ClockFormat: s.ClockFormat,
	})

	if err != nil {
		return 0, err
	}

	errorEstimate, err := NewErrorEstimate(
		s.Config.ErrorEstimateSynchronized,
		s.Config.ErrorEstimateClockFormat,
		s.Config.ErrorEstimateScale,
		s.Config.ErrorEstimateMultiplier,
	)

	if err != nil {
		return 0, err
	}

	// Create packet
	senderPkt := SenderPacket{
		SequenceNumber: s.seq,
		Timestamp:      *timestamp,
		ErrorEstimate:  errorEstimate.Encode(),
	}

	buf, err := senderPkt.Encode(nil)

	if err != nil {
		return 0, err
	}

	_, err = s.Conn.Write(buf)

	if err != nil {
		return 0, err
	}

	return s.seq, nil
}

// Close releases the underlying socket.
func (s *Sender) Close() error {
	if s.Conn == nil {
		return nil
	}
	return s.Conn.Close()
}
