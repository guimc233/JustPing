package protocol

import "time"

// MessageType represents the type of WebSocket message
type MessageType string

const (
	TypeRegisterRequest  MessageType = "register_request"
	TypeRegisterResponse MessageType = "register_response"
	TypeHeartbeat        MessageType = "heartbeat"
	TypeHeartbeatAck     MessageType = "heartbeat_ack"
	TypeTargetSync       MessageType = "target_sync"
	TypePingReport       MessageType = "ping_report"
	TypeTracerouteReport MessageType = "traceroute_report"
	TypeProxyOpen        MessageType = "proxy_open"
	TypeProxyOpenResult  MessageType = "proxy_open_result"
	TypeProxyData        MessageType = "proxy_data"
	TypeProxyClose       MessageType = "proxy_close"
	TypeError            MessageType = "error"
)

// Envelope wraps all WebSocket messages
type Envelope struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Payload   any         `json:"payload"`
}

// RegisterRequest is sent by the agent when connecting
type RegisterRequest struct {
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
}

// RegisterResponse is returned by the host after authentication
type RegisterResponse struct {
	Success bool   `json:"success"`
	AgentID string `json:"agent_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// TargetConfig represents an ICMP ping target assigned to the agent
type TargetConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	PacketCount  int    `json:"packet_count"` // e.g. 10 or 20
	IntervalSec  int    `json:"interval_sec"` // e.g. 60
	DisableRoute bool   `json:"disable_route,omitempty"`
}

// TargetSyncPayload is sent from Host to Agent
type TargetSyncPayload struct {
	Targets []TargetConfig `json:"targets"`
}

// SinglePingResult represents the result for a single target in a round
type SinglePingResult struct {
	TargetID    string    `json:"target_id"`
	TargetHost  string    `json:"target_host"`
	Timestamp   time.Time `json:"timestamp"`
	PacketsSent int       `json:"packets_sent"`
	PacketsRecv int       `json:"packets_recv"`
	LossPct     float64   `json:"loss_pct"`
	MinRTT      float64   `json:"min_rtt_ms"`
	MaxRTT      float64   `json:"max_rtt_ms"`
	AvgRTT      float64   `json:"avg_rtt_ms"`
	Jitter      float64   `json:"jitter_ms"`
	StdDev      float64   `json:"std_dev_ms"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
}

// PingReportPayload is sent from Agent to Host with one or more test results
type PingReportPayload struct {
	AgentID string             `json:"agent_id"`
	Results []SinglePingResult `json:"results"`
}

// HeartbeatPayload represents periodic agent heartbeat
type HeartbeatPayload struct {
	AgentID string  `json:"agent_id"`
	Uptime  uint64  `json:"uptime_sec"`
	LoadAvg float64 `json:"load_avg,omitempty"`
}

// TracerouteHop represents a single hop in a traceroute path (1..30)
type TracerouteHop struct {
	TTL      int       `json:"ttl"`
	IP       string    `json:"ip,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	RTTs     []float64 `json:"rtts_ms"`
	AvgRTT   float64   `json:"avg_rtt_ms"`
	LossPct  float64   `json:"loss_pct"`
}

// TracerouteReportPayload is sent from Agent to Host with complete route trace results
type TracerouteReportPayload struct {
	AgentID    string          `json:"agent_id"`
	TargetID   string          `json:"target_id,omitempty"`
	TargetHost string          `json:"target_host"`
	ResolvedIP string          `json:"resolved_ip"`
	Timestamp  time.Time       `json:"timestamp"`
	DurationMs int64           `json:"duration_ms"`
	Reached    bool            `json:"reached"`
	Hops       []TracerouteHop `json:"hops"`
}

// ProxyOpenPayload asks a probe to dial a TCP target for one HTTPS CONNECT tunnel.
type ProxyOpenPayload struct {
	TunnelID string `json:"tunnel_id"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// ProxyOpenResultPayload is the probe's dial result.
type ProxyOpenResultPayload struct {
	TunnelID string `json:"tunnel_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ProxyDataPayload carries one chunk of tunnel bytes, base64-encoded.
type ProxyDataPayload struct {
	TunnelID string `json:"tunnel_id"`
	Data     string `json:"data"`
}

// ProxyClosePayload closes a tunnel from either side.
type ProxyClosePayload struct {
	TunnelID string `json:"tunnel_id"`
	Reason   string `json:"reason,omitempty"`
}
