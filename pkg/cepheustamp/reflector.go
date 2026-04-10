package cepheustamp

import "net"

// Reflector receives STAMP test packets and sends reflected replies back to
// the sender, populating the reflector-side fields.
type Reflector struct {
	conn *net.UDPConn
}

func NewReflector(local *net.UDPAddr) (*Reflector, error) {
	panic("not implemented")
}

// Serve runs the reflector loop until the underlying connection is closed.
// Each received packet is reflected back to its sender.
func (r *Reflector) Serve() error {
	panic("not implemented")
}

func (r *Reflector) Close() error {
	if r.conn == nil {
		return nil
	}
	return r.conn.Close()
}
