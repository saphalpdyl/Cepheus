package cepheustamp

import (
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

type SenderConfig struct {
	LocalAddr  string
	RemoteAddr string
	HMACKey    *[]byte
}

// Sender originates STAMP test packets and matches reflected replies against
// outstanding sequence numbers.
type Sender struct {
	Conn    *net.UDPConn
	HMACKey *[]byte
	seq     uint32
}

func (s *Sender) NewSender(config SenderConfig) (*Sender, error) {
	localAddr, err := net.ResolveUDPAddr("udp", config.LocalAddr)
	if err != nil {
		return nil, err
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", config.RemoteAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		return nil, err
	}

	return &Sender{
		Conn:    conn,
		HMACKey: config.HMACKey,
		seq:     0,
	}, nil
}

// Send transmits a single test packet and returns the sequence number used.
func (s *Sender) Send() (uint32, error) {
	if s.HMACKey == nil {
		// Authenticated mode; not supported
		panic("not implemented")
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
