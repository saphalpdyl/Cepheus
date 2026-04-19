package scamper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ScamperClient struct {
	BinPath    string
	SocketPath string
	PPS        uint32
	Window     uint32
	Format     ScamperFormat

	cmd    *exec.Cmd
	conn   net.Conn
	reader *bufio.Reader

	mu sync.Mutex

	pendingQ []chan<- ReaderResult
	waiters  map[string]chan<- ReaderResult // scamper id -> channel mapping

	done chan struct{}
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
		Format:     cfg.Format,
		done:       make(chan struct{}, 1),
		waiters:    make(map[string]chan<- ReaderResult, 50),
		pendingQ:   make([]chan<- ReaderResult, 0, 50),
	}, nil
}

func (s *ScamperClient) Start(ctx context.Context) error {
	os.Remove(s.SocketPath)

	s.cmd = exec.CommandContext(ctx, s.BinPath,
		"-U", s.SocketPath,
		"-p", fmt.Sprintf("%d", s.PPS),
		"-w", fmt.Sprintf("%d", s.Window),
		"-O", string(s.Format),
	)

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scamper: %w", err)
	}

	// wait for socket to appear
	for i := 0; i < 20; i++ {
		select {
		case <-ctx.Done():
			s.cmd.Process.Kill()
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
		s.cmd.Process.Kill()
		return fmt.Errorf("failed to connect to socket: %w", err)
	}

	s.conn = conn
	s.reader = bufio.NewReader(conn)

	// Send attach before starting the reader to avoid the race where
	// StartRead processes the OK before the channel is queued in pendingQ.
	attachCh, err := s.Send(fmt.Sprintf("attach format %s", string(s.Format)))
	if err != nil {
		return err
	}

	go s.StartRead(ctx)

	// Wait for attach to be ACKed
	select {
	case <-attachCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (s *ScamperClient) Stop() error {
	close(s.done)
	s.conn.Close()
	return nil
}

func (s *ScamperClient) Send(cmd string) (<-chan ReaderResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := fmt.Fprintf(s.conn, "%s\n", cmd)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan ReaderResult, 1)
	s.pendingQ = append(s.pendingQ, resultChan)

	return resultChan, nil
}

func (s *ScamperClient) StartRead(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
		case <-s.done:
		}

		s.conn.SetReadDeadline(time.Now())
	}()

	for {
		line, err := s.reader.ReadString('\n')

		if err != nil {

			if ctx.Err() != nil {
				return ctx.Err()
			}

			fmt.Printf("error here %v", err)

			select {
			case <-s.done:
				return nil
			default:
				return err
			}
		}

		line = strings.TrimRight(line, "\n")

		if strings.HasPrefix(line, "ERR") {
			s.mu.Lock()
			if len(s.pendingQ) == 0 {
				s.mu.Unlock()
				fmt.Println("OK without a command pending")
				continue
			}

			s.pendingQ = s.pendingQ[1:]
			s.mu.Unlock()

			continue
		}

		if strings.HasPrefix(line, "MORE") {
			continue
		}

		if strings.HasPrefix(line, "OK") {
			var id string
			fmt.Sscanf(line, "OK %s", &id)

			s.mu.Lock()
			if len(s.pendingQ) == 0 {
				s.mu.Unlock()
				fmt.Println("OK without a command pending")
				continue
			}

			// pop from queue
			waiterChan := s.pendingQ[0]
			s.pendingQ = s.pendingQ[1:]

			if id != "" {
				s.waiters[id] = waiterChan
			} else {
				// Don't care about non-id OKs ( eg. for 'attach' commands )
				// Just send a empty value back
				waiterChan <- ReaderResult{
					Data: nil,
				}
			}
			s.mu.Unlock()

			continue
		}

		if strings.HasPrefix(line, "DATA") {
			var length int
			var id string
			fmt.Sscanf(line, "DATA %d %s", &length, &id)

			buf := make([]byte, length)
			io.ReadFull(s.reader, buf)

			if id == "" {
				continue
			}

			s.mu.Lock()
			waiterChan, ok := s.waiters[id]
			if !ok {
				s.mu.Unlock()
				fmt.Printf("missing id %s\n", id)
				continue
			}
			delete(s.waiters, id)
			s.mu.Unlock()

			waiterChan <- ReaderResult{
				Data: buf,
			}

			continue
		}
	}
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

// // wait for socket to appear
// for i := 0; i < 20; i++ {
// 	select {
// 	case <-ctx.Done():
// 		s.Cmd.Process.Kill()
// 		return ctx.Err()
// 	default:
// 	}
// 	if _, err := os.Stat(s.SocketPath); err == nil {
// 		break
// 	}
// 	time.Sleep(100 * time.Millisecond)
// }

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
