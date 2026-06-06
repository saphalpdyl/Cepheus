package scamper_client

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
	waiters  map[string]chan<- ReaderResult // scamper-client id -> channel mapping

	done chan struct{}
}

func NewClient(cfg ScamperClientConfig) (*ScamperClient, error) {
	if cfg.PPS == 0 {
		cfg.PPS = 100
	}

	if cfg.SocketPath == "" {
		cfg.SocketPath = "/tmp/scamper-client.sock"
	}

	if cfg.BinPath == "" {
		return nil, &ConfigError{
			Field:   "cfg.BinPath",
			Message: "bin path cannot be empty",
		}
	}

	if cfg.Format == "" {
		cfg.Format = ScamperFormatWarts
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
		return fmt.Errorf("failed to start scamper-client: %w", err)
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
