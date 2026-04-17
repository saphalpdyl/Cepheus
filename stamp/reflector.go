package stamp

import (
	"fmt"
	"net"
)

//   0                   1                   2                   3
//   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |                        Sequence Number                        |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |                          Timestamp                            |
//  |                                                               |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |         Error Estimate        |            MBZ                |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |                          Receive Timestamp                    |
//  |                                                               |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |                 Session-Sender Sequence Number                |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |                  Session-Sender Timestamp                     |
//  |                                                               |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  | Session-Sender Error Estimate |            MBZ                |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//  |Ses-Sender TTL |                      MBZ                      |
//  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// Reflector receives STAMP test packets and sends reflected replies back to
// the sender, populating the reflector-side fields.
type ReflectorConfig struct {
	LocalAddr string
	HMACKey   []byte
	OnError   func(error)
	Config    Config
}

type Reflector struct {
	Server    *net.PacketConn
	LocalAddr string
	Config    Config
	seq       uint32
	onError   func(error)
}

func NewReflector(cfg ReflectorConfig) (*Reflector, error) {
	return &Reflector{
		Server:    nil,
		seq:       0,
		Config:    cfg.Config,
		LocalAddr: cfg.LocalAddr,
		onError:   cfg.OnError,
	}, nil
}

// Serve runs the reflector loop until the underlying connection is closed.
// Each received packet is reflected back to its sender.
func (r *Reflector) Serve() error {
	udpServer, err := net.ListenPacket("udp", r.LocalAddr)
	if err != nil {
		return err
	}

	defer udpServer.Close()

	r.Server = &udpServer

	for {
		buf := make([]byte, 1500)
		readBytes, addr, err := udpServer.ReadFrom(buf)
		if err != nil {
			r.handleError(err)
			continue
		}

		// Capture receive timestamp as early as possible after reading.
		rxTimestamp, err := NewTimestamp(TimestampParams{
			ClockFormat: r.Config.ErrorEstimate.ClockFormat,
		})
		if err != nil {
			r.handleError(err)
			continue
		}

		if readBytes < 44 { // TODO: Change when after auth mode is implemented
			r.handleError(fmt.Errorf("packet too short: got %d bytes, need 44", readBytes))
			continue
		}

		senderPkt, err := DecodeSenderPacket(nil, buf)
		if err != nil {
			r.handleError(err)
			continue
		}

		errorEstimate, err := NewErrorEstimate(
			r.Config.ErrorEstimate.Synchronized,
			r.Config.ErrorEstimate.ClockFormat,
			r.Config.ErrorEstimate.Scale,
			r.Config.ErrorEstimate.Multiplier,
		)
		if err != nil {
			r.handleError(err)
			continue
		}

		timestamp, err := NewTimestamp(TimestampParams{
			ClockFormat: r.Config.ErrorEstimate.ClockFormat,
		})
		if err != nil {
			r.handleError(err)
			continue
		}

		reflectorPkt := ReflectorPacket{
			SequenceNumber:       r.seq,
			Timestamp:            *timestamp,
			ErrorEstimate:        errorEstimate.Encode(),
			ReceiveTimestamp:     *rxTimestamp,
			SenderSequenceNumber: senderPkt.SequenceNumber,
			SenderTimestamp:      senderPkt.Timestamp,
			SenderErrorEstimate:  senderPkt.ErrorEstimate,
			SenderTTL:            1,
		}

		r.seq++

		reflectorPktBytes, err := reflectorPkt.Encode(nil)
		if err != nil {
			r.handleError(err)
			continue
		}

		_, err = udpServer.WriteTo(reflectorPktBytes, addr)
		if err != nil {
			r.handleError(err)
			continue
		}
	}
}

func (r *Reflector) handleError(err error) {
	if r.onError != nil {
		r.onError(err)
	}
}

func (r *Reflector) Close() error {
	if r.Server == nil {
		return nil
	}
	return (*r.Server).Close()
}
