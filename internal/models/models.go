package models

import (
	"time"
)

type Agent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Hostname  string    `gorm:"uniqueIndex" json:"hostname"` 
	OS        string    `json:"os"`
	UserFullName string    `json:"user_full_name"` 
    Organization string    `json:"organization"`
	IPAddress string    `json:"ip_address"`
	Status       string    `json:"status"`    
	ActiveTime   string    `json:"active_time"` 

	LastSeen     time.Time `json:"last_seen"`
}

type Activity struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   uint      `json:"agent_id"` 
	Agent     Agent     `json:"agent"`
	Window    string    `json:"window"`
	// Timestamp time.Time `json:"timestamp"`
	Timestamp time.Time `json:"timestamp"`
}

type Screenshot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   uint      `json:"agent_id"`
	FilePath  string    `json:"file_path"` 
	Timestamp time.Time `json:"timestamp"`
}