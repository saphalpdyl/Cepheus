//go:build integration

package integration_test

import (
	cepheusstamp "cepheus/stamp"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reflectorAddr = "127.0.0.1:19862"

func startReflector(t *testing.T, ctx context.Context) {
	t.Helper()
	reflector, err := cepheusstamp.NewReflector(cepheusstamp.ReflectorConfig{
		LocalAddr: reflectorAddr,
		HMACKey:   nil,
		OnError:   func(err error) { t.Logf("reflector error: %v", err) },
		Config:    config,
	})
	require.NoError(t, err)

	t.Cleanup(func() { reflector.Close() })

	go func() {
		if err := reflector.Serve(ctx); err != nil {
			t.Logf("reflector serve exited: %v", err)
		}
	}()

	// Give the reflector time to bind and start listening.
	time.Sleep(50 * time.Millisecond)
}

func newSender(t *testing.T) *cepheusstamp.Sender {
	t.Helper()
	sender, err := cepheusstamp.NewSender(cepheusstamp.SenderConfig{
		Config:     config,
		LocalAddr:  "127.0.0.1:0",
		RemoteAddr: reflectorAddr,
		HMACKey:    nil,
		Timeout:    5 * time.Second,
		OnError:    func(err error) { t.Logf("sender error: %v", err) },
	})
	require.NoError(t, err)
	t.Cleanup(func() { sender.Close() })
	return sender
}

func Test_ReflectorBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startReflector(t, ctx)
	sender := newSender(t)

	response, err := sender.Send()
	require.NoError(t, err)

	// Echoed sender fields
	assert.Equal(t, uint32(0), response.SenderSequenceNumber)

	// Timestamp ordering: SenderTimestamp < ReceiveTimestamp < Timestamp (RFC 8762§4.3.1)
	clockFmt := config.ErrorEstimate.ClockFormat
	senderTs, err1 := response.SenderTimestamp.ToTime(clockFmt)
	receiveTs, err2 := response.ReceiveTimestamp.ToTime(clockFmt)
	reflectorTs, err3 := response.Timestamp.ToTime(clockFmt)
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	assert.True(t, senderTs.Before(*receiveTs),
		"SenderTimestamp (%v) should be before ReceiveTimestamp (%v)", senderTs, receiveTs)
	assert.True(t, receiveTs.Before(*reflectorTs),
		"ReceiveTimestamp (%v) should be before Timestamp (%v)", receiveTs, reflectorTs)
}

func Test_ReflectorMultiplePackets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startReflector(t, ctx)
	sender := newSender(t)

	const count = 5
	for i := 0; i < count; i++ {
		response, err := sender.Send()
		require.NoError(t, err, "packet %d failed", i)

		// Sender sequence numbers increment: 0, 1, 2, ...
		assert.Equal(t, uint32(i), response.SenderSequenceNumber,
			"packet %d: wrong echoed sender sequence number", i)

		// Reflector sequence numbers increment: 0, 1, 2, ...
		assert.Equal(t, uint32(i), response.SequenceNumber,
			"packet %d: wrong reflector sequence number", i)
	}
}
