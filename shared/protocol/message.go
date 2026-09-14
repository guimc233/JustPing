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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	PacketCount int    `json:"packet_count"` // e.g. 10 or 20
	IntervalSec int    `json:"interval_sec"` // e.g. 60
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
