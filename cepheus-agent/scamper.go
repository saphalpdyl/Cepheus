package cepheusagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Scamper struct {
	BinPath    string
	SocketPath string
	PPS        int
	Cmd        *exec.Cmd
	Conn       net.Conn
	mu         sync.Mutex
	scanner    *bufio.Scanner

	logger *slog.Logger
}

type TraceHop struct {
	Addr string  `json:"addr"`
	RTT  float64 `json:"rtt"`
	TTL  int     `json:"probe_ttl"`
}

func (h *TraceHop) ToMap() map[string]any {
	return map[string]any{
		"addr":      h.Addr,
		"rtt":       h.RTT,
		"probe_ttl": h.TTL,
	}
}

type TraceResult struct {
	Type string     `json:"type"`
	Src  string     `json:"src"`
	Dst  string     `json:"dst"`
	Hops []TraceHop `json:"hops"`
}

func (r *TraceResult) ToMap() map[string]any {
	hops := make([]map[string]any, len(r.Hops))
	for i := range r.Hops {
		hops[i] = r.Hops[i].ToMap()
	}
	return map[string]any{
		"type": r.Type,
		"src":  r.Src,
		"dst":  r.Dst,
		"hops": hops,
	}
}

func NewScamper(binPath string, pps int, logger *slog.Logger) *Scamper {
	if pps == 0 {
		pps = 100
	}
	return &Scamper{
		BinPath:    binPath,
		SocketPath: "/tmp/scamper.sock",
		PPS:        pps,
		logger:     logger,
	}
}

func (s *Scamper) Start(ctx context.Context) error {
	// clean up stale socket
	os.Remove(s.SocketPath)

	s.Cmd = exec.CommandContext(ctx, s.BinPath,
		"-U", s.SocketPath,
		"-p", fmt.Sprintf("%d", s.PPS),
		"-O", "json",
	)
	if err := s.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scamper: %w", err)
	}

	// wait for socket to appear
	for i := 0; i < 20; i++ {
		select {
		case <-ctx.Done():
			s.Cmd.Process.Kill()
			return ctx.Err()
		default:
		}
		if _, err := os.Stat(s.SocketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	conn, err := net.Dial("unix", s.SocketPath)
	if err != nil {
		s.Cmd.Process.Kill()
		return fmt.Errorf("failed to connect to socket: %w", err)
	}

	s.Conn = conn
	s.scanner = bufio.NewScanner(conn)
	return nil
}

func (s *Scamper) Stop() error {
	if s.Conn != nil {
		s.Conn.Close()
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		return s.Cmd.Process.Kill()
	}
	return nil
}

func (s *Scamper) send(ctx context.Context, command string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check context before sending
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	_, err := fmt.Fprintf(s.Conn, "%s\n", command)
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Use a goroutine to unblock the scanner when the context is cancelled.
	// Setting a read deadline on the connection causes scanner.Scan() to return.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.Conn.SetReadDeadline(time.Now())
		case <-done:
		}
	}()

	// read lines until we get a JSON result
	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if line[0] == '{' {
			out := make([]byte, len(line))
			copy(out, line)
			s.Conn.SetReadDeadline(time.Time{})
			return json.RawMessage(out), nil
		}

	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("connection closed before result")
}

func (s *Scamper) Traceroute(ctx context.Context, dst string) (*TraceResult, error) {
	raw, err := s.send(ctx, fmt.Sprintf("trace -P icmp-paris %s", dst))
	if err != nil {
		return nil, err
	}
	var result TraceResult
	return &result, json.Unmarshal(raw, &result)
}

func (s *Scamper) IsRunning() bool {
	if s.Cmd == nil || s.Cmd.Process == nil {
		return false
	}
	// process.Signal(0) checks if process exists
	return s.Cmd.Process.Signal(syscall.Signal(0)) == nil
}
