package cepheusstamp

import (
	"errors"
	"fmt"
	"net"
	"time"
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
	LocalAddr  string
	RemoteAddr string
	HMACKey    []byte
	OnError    func(error)
	Timeout    time.Duration
	Config     Config
}

// Sender originates STAMP test packets and matches reflected replies against
// outstanding sequence numbers.
type Sender struct {
	Conn    *net.UDPConn
	HMACKey []byte
	seq     uint32
	onError func(error)
	timeout time.Duration
	Config  Config
}

// NewSender resolves the local and remote UDP addresses from cfg, opens a
// connected UDP socket, and returns a ready-to-use Sender.
func NewSender(cfg SenderConfig) (*Sender, error) {
	if cfg.Config.ErrorEstimate.ClockFormat != ClockFormatNTP {
		return nil, errors.New("invalid clock format: valid options are NTP")
	}

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

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &Sender{
		Conn:    conn,
		HMACKey: cfg.HMACKey,
		seq:     0,
		onError: cfg.OnError,
		timeout: timeout,
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
func (s *Sender) Send() (*ReflectorPacket, error) {
	if s.HMACKey != nil {
		panic("not implemented")
	}

	timestamp, err := NewTimestamp(TimestampParams{
		ClockFormat: s.Config.ErrorEstimate.ClockFormat,
	})
	if err != nil {
		return nil, err
	}

	errorEstimate, err := NewErrorEstimate(
		s.Config.ErrorEstimate.Synchronized,
		s.Config.ErrorEstimate.ClockFormat,
		s.Config.ErrorEstimate.Scale,
		s.Config.ErrorEstimate.Multiplier,
	)
	if err != nil {
		return nil, err
	}

	senderPkt := SenderPacket{
		SequenceNumber: s.seq,
		Timestamp:      *timestamp,
		ErrorEstimate:  errorEstimate.Encode(),
	}

	buf, err := senderPkt.Encode(nil)
	if err != nil {
		return nil, err
	}

	_, err = s.Conn.Write(buf)
	if err != nil {
		return nil, err
	}

	// Wait for the reflected reply
	s.Conn.SetReadDeadline(time.Now().Add(s.timeout))
	rxBuf := make([]byte, 1500)
	n, err := s.Conn.Read(rxBuf)
	if err != nil {
		return nil, fmt.Errorf("waiting for reply: %w", err)
	}

	if n < 44 {
		return nil, fmt.Errorf("reply too short: got %d bytes, need 44", n)
	}

	reply, err := DecodeReflectorPacket(nil, rxBuf[:n])
	if err != nil {
		return nil, fmt.Errorf("decoding reply: %w", err)
	}

	s.seq++

	return reply, nil
}

// Close releases the underlying socket.
func (s *Sender) Close() error {
	if s.Conn == nil {
		return nil
	}
	return s.Conn.Close()
}
