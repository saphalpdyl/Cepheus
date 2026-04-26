package common

import (
	"encoding/json"
	"time"
)

type ProbeResult struct {
	TaskID    string          `json:"task_id"`
	ProbeType ProbeType       `json:"probe_type"`
	Kind      string          `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type ReportPayload struct {
	Payload       ProbeResult `json:"payload"`
	SerialID      string      `json:"serial_id"`
	SentTimestamp time.Time   `json:"sent_timestamp"`
	AgentConfigId string      `json:"agent_config_id"`
}

// Used to differentiate data from different probes in dispatcher.go
type ProbeType string

const (
	ProbeTypeStamp ProbeType = "stamp"
	ProbeTypeTrace ProbeType = "trace"
)

// STAMP Data
type StampData struct {
	Target   string  `json:"target"`
	Port     int     `json:"port"`
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	Loss     float64 `json:"loss"`
	AvgRTT   int64   `json:"avg_rtt"`
	MinRTT   int64   `json:"min_rtt"`
	MaxRTT   int64   `json:"max_rtt"`
	P50RTT   int64   `json:"p50_rtt"`
	P95RTT   int64   `json:"p95_rtt"`
}

// TraceRoute Data
type TraceProbeMethod string

const (
	TraceMethodICMPParis TraceProbeMethod = "icmp-paris"
	TraceMethodUDPParis  TraceProbeMethod = "udp-paris"
	TraceMethodTCP       TraceProbeMethod = "tcp"
)

type TraceProbeType string

const (
	TraceProbeTypeTrace   TraceProbeType = "trace"
	TraceProbeTypeTraceLb TraceProbeType = "tracelb"
)

type TraceData struct {
	Type   TraceProbeType   `json:"type"`
	Method TraceProbeMethod `json:"method"`
	Format string           `json:"format"`
	Data   []byte           `json:"data"`
}

type TraceDataTracePayload struct {
	Type       string `json:"type"`
	Version    string `json:"version"`
	Userid     int    `json:"userid,omitempty"`
	Method     string `json:"method"`
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	IcmpSum    int    `json:"icmp_sum,omitempty"`
	StopReason string `json:"stop_reason"`
	StopData   int    `json:"stop_data,omitempty"`
	Start      struct {
		Sec   int    `json:"sec"`
		Usec  int    `json:"usec"`
		Ftime string `json:"ftime"`
	} `json:"start"`
	HopCount   int      `json:"hop_count"`
	Attempts   int      `json:"attempts"`
	Hoplimit   int      `json:"hoplimit,omitempty"`
	Firsthop   int      `json:"firsthop,omitempty"`
	Wait       int      `json:"wait"`
	WaitProbe  int      `json:"wait_probe,omitempty"`
	Tos        int      `json:"tos,omitempty"`
	ProbeSize  int      `json:"probe_size"`
	ProbeCount int      `json:"probe_count"`
	Flags      []string `json:"flags,omitempty"`
	Hops       []struct {
		Addr      string `json:"addr"`
		ProbeTTL  int    `json:"probe_ttl"`
		ProbeID   int    `json:"probe_id"`
		ProbeSize int    `json:"probe_size"`
		Tx        struct {
			Sec  int `json:"sec"`
			Usec int `json:"usec"`
		} `json:"tx"`
		Rtt       float64  `json:"rtt"`
		ReplyTTL  int      `json:"reply_ttl"`
		ReplyTos  int      `json:"reply_tos,omitempty"`
		Flags     []string `json:"flags,omitempty"`
		ReplyIpid int      `json:"reply_ipid,omitempty"`
		ReplySize int      `json:"reply_size"`
		IcmpType  int      `json:"icmp_type"`
		IcmpCode  int      `json:"icmp_code"`
		IcmpQTTL  int      `json:"icmp_q_ttl,omitempty"`
		IcmpQIpl  int      `json:"icmp_q_ipl,omitempty"`
		IcmpQTos  int      `json:"icmp_q_tos,omitempty"`
	} `json:"hops,omitempty"`
	NoHops []struct {
		ProbeTTL  int `json:"probe_ttl"`
		ProbeID   int `json:"probe_id"`
		ProbeSize int `json:"probe_size"`
		Tx        struct {
			Sec  int `json:"sec"`
			Usec int `json:"usec"`
		} `json:"tx"`
	} `json:"no_hops,omitempty"`
}

type TraceDateTracelbPayload struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Userid  int    `json:"userid,omitempty"`
	Method  string `json:"method"`
	Src     string `json:"src"`
	Dst     string `json:"dst"`
	Sport   int    `json:"sport,omitempty"`
	Dport   int    `json:"dport,omitempty"`
	Start   struct {
		Sec   int    `json:"sec"`
		Usec  int    `json:"usec"`
		Ftime string `json:"ftime"`
	} `json:"start"`
	ProbeSize   int `json:"probe_size"`
	Firsthop    int `json:"firsthop,omitempty"`
	Attempts    int `json:"attempts"`
	Confidence  int `json:"confidence,omitempty"`
	Tos         int `json:"tos,omitempty"`
	Gaplimit    int `json:"gaplimit,omitempty"`
	WaitTimeout int `json:"wait_timeout"`
	WaitProbe   int `json:"wait_probe,omitempty"`
	Probec      int `json:"probec"`
	ProbecMax   int `json:"probec_max,omitempty"`
	Nodec       int `json:"nodec"`
	Linkc       int `json:"linkc"`
	Nodes       []struct {
		Addr  string `json:"addr"`
		QTTL  int    `json:"q_ttl,omitempty"`
		Linkc int    `json:"linkc"`
		Links [][]struct {
			Addr   string `json:"addr"`
			Probes []struct {
				Tx struct {
					Sec  int `json:"sec"`
					Usec int `json:"usec"`
				} `json:"tx"`
				Replyc  int `json:"replyc"`
				TTL     int `json:"ttl"`
				Attempt int `json:"attempt"`
				Flowid  int `json:"flowid"`
				Replies []struct {
					Rx struct {
						Sec  int `json:"sec"`
						Usec int `json:"usec"`
					} `json:"rx"`
					TTL      int     `json:"ttl"`
					Rtt      float64 `json:"rtt"`
					Ipid     int     `json:"ipid,omitempty"`
					IcmpType int     `json:"icmp_type"`
					IcmpCode int     `json:"icmp_code"`
					IcmpQTos int     `json:"icmp_q_tos,omitempty"`
					IcmpQTTL int     `json:"icmp_q_ttl,omitempty"`
				} `json:"replies,omitempty"`
			} `json:"probes,omitempty"`
		} `json:"links,omitempty"`
	} `json:"nodes,omitempty"`
}
