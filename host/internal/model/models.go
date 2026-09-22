package model

import (
	"time"
)

// SystemSetting stores key-value application configurations
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmailWhitelist stores trusted GitHub emails permitted to log in
type EmailWhitelist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedBy string    `gorm:"size:255" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents an administrator authenticated via OAuth
type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Provider    string    `gorm:"size:32;not null;uniqueIndex:idx_provider_user,priority:1" json:"provider"`
	ProviderID  string    `gorm:"size:128;not null;uniqueIndex:idx_provider_user,priority:2" json:"provider_id"`
	GitHubID    int64     `gorm:"index" json:"github_id"`
	Username    string    `gorm:"size:128;not null" json:"username"`
	Email       string    `gorm:"size:255;not null" json:"email"`
	AvatarURL   string    `gorm:"size:512" json:"avatar_url"`
	Role        string    `gorm:"size:32;default:'admin'" json:"role"` // "superadmin" or "admin"
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// Target represents a monitoring destination for ICMP ping
type Target struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Name         string    `gorm:"size:128;not null" json:"name"`
	Host         string    `gorm:"size:255;not null" json:"host"`
	PacketCount  int       `gorm:"default:20" json:"packet_count"` // sliding window sample size (e.g. 20 samples = 10 min)
	IntervalSec  int       `gorm:"default:30" json:"interval_sec"` // probe interval in seconds (default: 30s)
	Tags         string    `gorm:"size:255" json:"tags"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	DisableRoute bool      `gorm:"default:false" json:"disable_route"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Agent represents a distributed ping probe
type Agent struct {
	ID                   string    `gorm:"primaryKey;size:36" json:"id"`
	Name                 string    `gorm:"size:128;not null" json:"name"`
	Token                string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	PublicIP             string    `gorm:"size:128" json:"public_ip"`
	OS                   string    `gorm:"size:64" json:"os"`
	Arch                 string    `gorm:"size:64" json:"arch"`
	Version              string    `gorm:"size:32" json:"version"`
	Tags                 string    `gorm:"size:255" json:"tags"`
	DisabledRouteTargets string    `gorm:"type:text" json:"disabled_route_targets"` // comma-separated target IDs where route testing is disabled for this agent
	IsOnline             bool      `gorm:"default:false" json:"is_online"`
	LastSeenAt           time.Time `json:"last_seen_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// UpdateStatus is runtime-only state reported by the probe after a
	// Host-triggered update check. It is never persisted.
	UpdateStatus *AgentUpdateStatus `gorm:"-" json:"update_status,omitempty"`
}

// AgentUpdateStatus describes the outcome of the most recent Host-triggered
// update check performed by a probe.
type AgentUpdateStatus struct {
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	Updating       bool      `json:"updating"`
	Error          string    `json:"error,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

// PingMetric represents the ping quality measurements
type PingMetric struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID     string    `gorm:"size:36;index:idx_agent_time,priority:1;not null" json:"agent_id"`
	TargetID    string    `gorm:"size:36;index:idx_target_time,priority:1;not null" json:"target_id"`
	Timestamp   time.Time `gorm:"index:idx_agent_time,priority:2;index:idx_target_time,priority:2;index;not null" json:"timestamp"`
	PacketsSent int       `json:"packets_sent"`
	PacketsRecv int       `json:"packets_recv"`
	LossPct     float64   `gorm:"type:numeric(5,2)" json:"loss_pct"`
	MinRTT      float64   `gorm:"type:numeric(8,2)" json:"min_rtt_ms"`
	MaxRTT      float64   `gorm:"type:numeric(8,2)" json:"max_rtt_ms"`
	AvgRTT      float64   `gorm:"type:numeric(8,2)" json:"avg_rtt_ms"`
	Jitter      float64   `gorm:"type:numeric(8,2)" json:"jitter_ms"`
	StdDev      float64   `gorm:"type:numeric(8,2)" json:"std_dev_ms"`
	ErrorMsg    string    `gorm:"size:255" json:"error_msg,omitempty"`
}

// EnrichedHop represents NextTrace-style hop information with ASN and Geo metadata
type EnrichedHop struct {
	TTL         int       `json:"ttl"`
	IP          string    `json:"ip"`
	Hostname    string    `json:"hostname,omitempty"`
	RTTs        []float64 `json:"rtts_ms"`
	AvgRTT      float64   `json:"avg_rtt_ms"`
	LossPct     float64   `json:"loss_pct"`
	ASNumber    string    `json:"as_number,omitempty"`
	ASOrg       string    `json:"as_org,omitempty"`
	ISP         string    `json:"isp,omitempty"`
	Country     string    `json:"country,omitempty"`
	CountryCode string    `json:"country_code,omitempty"`
	City        string    `json:"city,omitempty"`
}

// ASNode represents a node in the Autonomous System hop path
type ASNode struct {
	ASN  string `json:"asn"`
	Name string `json:"name"`
}

// TracerouteRecord stores full traceroute path telemetry
type TracerouteRecord struct {
	ID         string        `gorm:"primaryKey;size:36" json:"id"`
	AgentID    string        `gorm:"size:36;index:idx_trace_agent_time,priority:1;index:idx_trace_agent_target,priority:1;not null" json:"agent_id"`
	TargetID   string        `gorm:"size:36;index:idx_trace_target_time,priority:1;index:idx_trace_agent_target,priority:2;not null" json:"target_id"`
	TargetHost string        `gorm:"size:255;not null" json:"target_host"`
	ResolvedIP string        `gorm:"size:128" json:"resolved_ip"`
	Timestamp  time.Time     `gorm:"index:idx_trace_agent_time,priority:2;index:idx_trace_target_time,priority:2;index;not null" json:"timestamp"`
	DurationMs int64         `json:"duration_ms"`
	Reached    bool          `json:"reached"`
	HopCount   int           `json:"hop_count"`
	RoutePath  string        `gorm:"size:64" json:"route_path"`
	ASPath     string        `gorm:"size:512" json:"as_path"`
	ASNodes    []ASNode      `gorm:"serializer:json" json:"as_nodes"`
	Hops       []EnrichedHop `gorm:"serializer:json" json:"hops"`
}
