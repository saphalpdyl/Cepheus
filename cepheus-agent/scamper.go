package cepheusagent

import (
	"bufio"
	"encoding/json"
	"fmt"
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
}

type PingResult struct {
	Type     string  `json:"type"`
	Src      string  `json:"src"`
	Dst      string  `json:"dst"`
	PingMin  float64 `json:"min_rtt"`
	PingMax  float64 `json:"max_rtt"`
	PingAvg  float64 `json:"avg_rtt"`
	Loss     float64 `json:"loss"`
	Sent     int     `json:"probe_count"`
	Received int     `json:"reply_count"`
}

type TraceHop struct {
	Addr string  `json:"addr"`
	RTT  float64 `json:"rtt"`
	TTL  int     `json:"probe_ttl"`
}

type TraceResult struct {
	Type string     `json:"type"`
	Src  string     `json:"src"`
	Dst  string     `json:"dst"`
	Hops []TraceHop `json:"hops"`
}

type TracelbNode struct {
	Addr string `json:"addr"`
}

type TracelbLink struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type TracelbResult struct {
	Type  string        `json:"type"`
	Src   string        `json:"src"`
	Dst   string        `json:"dst"`
	Nodes []TracelbNode `json:"nodes"`
	Links []TracelbLink `json:"links"`
}

func NewScamper(binPath string, pps int) *Scamper {
	if pps == 0 {
		pps = 100
	}
	return &Scamper{
		BinPath:    binPath,
		SocketPath: "/tmp/scamper.sock",
		PPS:        pps,
	}
}

func (s *Scamper) Start() error {
	// clean up stale socket
	os.Remove(s.SocketPath)

	s.Cmd = exec.Command(s.BinPath,
		"-U", s.SocketPath,
		"-p", fmt.Sprintf("%d", s.PPS),
		"-O", "json",
	)
	if err := s.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scamper: %w", err)
	}

	// wait for socket to appear
	for i := 0; i < 20; i++ {
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

func (s *Scamper) send(command string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := fmt.Fprintf(s.Conn, "%s\n", command)
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// read lines until we get a JSON result
	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if line[0] == '{' {
			return json.RawMessage(line), nil
		}
	}
	return nil, fmt.Errorf("connection closed before result")
}

func (s *Scamper) Ping(dst string, count int) (*PingResult, error) {
	raw, err := s.send(fmt.Sprintf("ping -c %d %s", count, dst))
	if err != nil {
		return nil, err
	}
	var result PingResult
	return &result, json.Unmarshal(raw, &result)
}

func (s *Scamper) Traceroute(dst string) (*TraceResult, error) {
	raw, err := s.send(fmt.Sprintf("trace -P icmp-paris %s", dst))
	if err != nil {
		return nil, err
	}
	var result TraceResult
	return &result, json.Unmarshal(raw, &result)
}

func (s *Scamper) Tracelb(dst string) (*TracelbResult, error) {
	raw, err := s.send(fmt.Sprintf("tracelb %s", dst))
	if err != nil {
		return nil, err
	}
	var result TracelbResult
	return &result, json.Unmarshal(raw, &result)
}

func (s *Scamper) IsRunning() bool {
	if s.Cmd == nil || s.Cmd.Process == nil {
		return false
	}
	// process.Signal(0) checks if process exists
	return s.Cmd.Process.Signal(syscall.Signal(0)) == nil
}
