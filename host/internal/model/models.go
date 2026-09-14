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
	Provider    string    `gorm:"size:32;default:'github'" json:"provider"`
	ProviderID  string    `gorm:"size:128;index" json:"provider_id"`
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
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Host        string    `gorm:"size:255;not null" json:"host"`
	PacketCount int       `gorm:"default:15" json:"packet_count"`
	IntervalSec int       `gorm:"default:60" json:"interval_sec"`
	Tags        string    `gorm:"size:255" json:"tags"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Agent represents a distributed ping probe
type Agent struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	Name       string    `gorm:"size:128;not null" json:"name"`
	Token      string    `gorm:"uniqueIndex;size:64;not null" json:"token"`
	PublicIP   string    `gorm:"size:128" json:"public_ip"`
	OS         string    `gorm:"size:64" json:"os"`
	Arch       string    `gorm:"size:64" json:"arch"`
	Version    string    `gorm:"size:32" json:"version"`
	Tags       string    `gorm:"size:255" json:"tags"`
	IsOnline   bool      `gorm:"default:false" json:"is_online"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
