## Go STAMP 
Package stamp implements the Simple Two-way Active Measurement Protocol
(STAMP) as defined in [RFC 8762](https://www.rfc-editor.org/rfc/rfc8762.pdf).
It provides two layers:
  - A low-level packet codec: Packet, Encode, Decode.
  - A high-level session API: Sender and Reflector.

Only the unauthenticated base mode is supported in v0. STAMP Optional
Extensions (RFC 8972) and HMAC authentication are out of scope for now.

### Use
**Note**: This document was written in **Apr-19-2026** and is subject to change in the future.

#### STAMP Config
```go
stampConfig := stamp.Config{
		ErrorEstimate: stamp.ErrorEstimateConfig{
			ClockFormat:  stamp.ClockFormatNTP,
			Multiplier:   1,
			Scale:        22,
			Synchronized: clockStateSynchronized,
		},
	}
```

#### Sender
```go
sender, err := stamp.NewSender(stamp.SenderConfig{
		LocalAddr:  p.SourceIP,
		RemoteAddr: net.JoinHostPort(target, targetPort),
		Timeout:    time.Duration(1) * time.Second,
		Config:     stampConfig,
		OnError:    func(err error) { logger.ErrorContext(ctx, "stamp sender error", "err", err) },
	})
	if err != nil {
        ...
	}
	defer sender.Close()

    reflectorPkt, err := sender.Send()
```

#### Reflector
```go
	reflector, err := stamp.NewReflector(stamp.ReflectorConfig{
		LocalAddr: net.JoinHostPort(sourceIP, listenPort),
		HMACKey:   nil,
		OnError: func(err error) {
			if ctx.Err() == nil {
				...
			}
		},
		Config: stampConfig,
	})

    err := reflector.Serve(ctx)
```