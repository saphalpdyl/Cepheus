package cepheustamp

import "net"

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

// Sender originates STAMP test packets and matches reflected replies against
// outstanding sequence numbers.
type Sender struct {
	conn *net.UDPConn
}

func NewSender(local, remote *net.UDPAddr) (*Sender, error) {
	panic("not implemented")
}

// Send transmits a single test packet and returns the sequence number used.
func (s *Sender) Send() (uint32, error) {
	panic("not implemented")
}

// Receive blocks until a reflected packet arrives or the underlying
// connection is closed.
func (s *Sender) Receive() (*Packet, error) {
	panic("not implemented")
}

// Close releases the underlying socket.
func (s *Sender) Close() error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}
