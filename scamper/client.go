package scamper

import (
	"bufio"
	"context"
	"net"
	"os/exec"
	"sync"
)

type ScamperClient struct {
	BinPath    string
	SocketPath string
	PPS        uint32
	Window     uint32

	cmd     *exec.Cmd
	conn    *net.Conn
	scanner *bufio.Scanner

	mu sync.Mutex
}

type ScamperClientConfig struct {
	BinPath    string
	SocketPath string
	PPS        uint32
	Window     uint32
}

func NewClient(cfg ScamperClientConfig) (*ScamperClient, error) {
	if cfg.PPS == 0 {
		cfg.PPS = 100
	}

	if cfg.SocketPath == "" {
		cfg.SocketPath = "/tmp/scamper.sock"
	}

	if cfg.BinPath == "" {
		return nil, &ConfigError{
			Field:   "cfg.BinPath",
			Message: "bin path cannot be empty",
		}
	}

	return &ScamperClient{
		BinPath:    cfg.BinPath,
		SocketPath: cfg.SocketPath,
		PPS:        cfg.PPS,
		Window:     cfg.Window,
	}, nil
}

func (s *ScamperClient) Start(ctx context.Context) error {
	return nil
}

func (s *ScamperClient) Stop() error {
	return nil
}

// type Scamper struct {
// 	BinPath    string
// 	SocketPath string
// 	PPS        int
// 	Cmd        *exec.Cmd
// 	Conn       net.Conn
// 	mu         sync.Mutex
// 	scanner    *bufio.Scanner

// 	logger *slog.Logger
// }

// func (s *Scamper) Start(ctx context.Context) error {
// 	// clean up stale socket
// 	os.Remove(s.SocketPath)

// 	s.Cmd = exec.CommandContext(ctx, s.BinPath,
// 		"-U", s.SocketPath,
// 		"-p", fmt.Sprintf("%d", s.PPS),
// 		"-O", "json",
// 	)
// 	if err := s.Cmd.Start(); err != nil {
// 		return fmt.Errorf("failed to start scamper: %w", err)
// 	}

// 	// wait for socket to appear
// 	for i := 0; i < 20; i++ {
// 		select {
// 		case <-ctx.Done():
// 			s.Cmd.Process.Kill()
// 			return ctx.Err()
// 		default:
// 		}
// 		if _, err := os.Stat(s.SocketPath); err == nil {
// 			break
// 		}
// 		time.Sleep(100 * time.Millisecond)
// 	}

// 	conn, err := net.Dial("unix", s.SocketPath)
// 	if err != nil {
// 		s.Cmd.Process.Kill()
// 		return fmt.Errorf("failed to connect to socket: %w", err)
// 	}

// 	s.Conn = conn
// 	s.scanner = bufio.NewScanner(conn)
// 	return nil
// }

// func (s *Scamper) Stop() error {
// 	if s.Conn != nil {
// 		s.Conn.Close()
// 	}
// 	if s.Cmd != nil && s.Cmd.Process != nil {
// 		return s.Cmd.Process.Kill()
// 	}
// 	return nil
// }

// func (s *Scamper) send(ctx context.Context, command string) (json.RawMessage, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	// Check context before sending
// 	if err := ctx.Err(); err != nil {
// 		return nil, err
// 	}

// 	_, err := fmt.Fprintf(s.Conn, "%s\n", command)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to send command: %w", err)
// 	}

// 	// Use a goroutine to unblock the scanner when the context is cancelled.
// 	// Setting a read deadline on the connection causes scanner.Scan() to return.
// 	done := make(chan struct{})
// 	defer close(done)
// 	go func() {
// 		select {
// 		case <-ctx.Done():
// 			s.Conn.SetReadDeadline(time.Now())
// 		case <-done:
// 		}
// 	}()

// 	// read lines until we get a JSON result
// 	for s.scanner.Scan() {
// 		line := s.scanner.Bytes()
// 		if len(line) == 0 {
// 			continue
// 		}
// 		if line[0] == '{' {
// 			out := make([]byte, len(line))
// 			copy(out, line)
// 			s.Conn.SetReadDeadline(time.Time{})
// 			return json.RawMessage(out), nil
// 		}

// 	}

// 	if ctx.Err() != nil {
// 		return nil, ctx.Err()
// 	}
// 	return nil, fmt.Errorf("connection closed before result")
// }

// func (s *Scamper) Traceroute(ctx context.Context, dst string) (*TraceResult, error) {
// 	raw, err := s.send(ctx, fmt.Sprintf("trace -P icmp-paris %s", dst))
// 	if err != nil {
// 		return nil, err
// 	}
// 	var result TraceResult
// 	return &result, json.Unmarshal(raw, &result)
// }

// func (s *Scamper) IsRunning() bool {
// 	if s.Cmd == nil || s.Cmd.Process == nil {
// 		return false
// 	}
// 	// process.Signal(0) checks if process exists
// 	return s.Cmd.Process.Signal(syscall.Signal(0)) == nil
// }
