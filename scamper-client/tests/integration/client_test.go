//go:build integration

package integration_test

import (
	"cepheus/scamper-client"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scamperBinPath = "/opt/agent/scamper-client"

func newClient(t *testing.T, ctx context.Context) *scamper_client.ScamperClient {
	t.Helper()

	client, err := scamper_client.NewClient(scamper_client.ScamperClientConfig{
		BinPath: scamperBinPath,
		PPS:     100,
		Window:  10,
		Format:  scamper_client.ScamperFormatWarts,
	})
	require.NoError(t, err)

	err = client.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { client.Stop() })

	return client
}

func TestClient_StartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newClient(t, ctx)
}

func TestClient_Trace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newClient(t, ctx)

	resultCh, err := client.Send("trace -P icmp-paris 1.1.1.1")
	require.NoError(t, err)

	select {
	case result := <-resultCh:
		assert.NotEmpty(t, result.Data, "expected non-empty warts data")
		t.Logf("got trace result: %d bytes", len(result.Data))
	case <-ctx.Done():
		t.Fatal("timed out waiting for trace result")
	}
}

func TestClient_ConcurrentTraces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := newClient(t, ctx)

	targets := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}
	channels := make([]<-chan scamper_client.ReaderResult, len(targets))

	for i, target := range targets {
		ch, err := client.Send("trace -P icmp-paris " + target)
		require.NoError(t, err, "failed to send trace for %s", target)
		channels[i] = ch
	}

	for i, ch := range channels {
		select {
		case result := <-ch:
			assert.NotEmpty(t, result.Data, "expected non-empty warts data for %s", targets[i])
			t.Logf("trace to %s: %d bytes", targets[i], len(result.Data))
		case <-ctx.Done():
			t.Fatalf("timed out waiting for trace result to %s", targets[i])
		}
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	client := newClient(t, ctx)
	_ = client

	cancel()

	// Give StartRead time to notice the cancellation
	time.Sleep(100 * time.Millisecond)
}
