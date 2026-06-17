package model

import "time"

// ScanStatus is the lifecycle state of a SAST job.
type ScanStatus string

const (
	ScanStatusNew      ScanStatus = "new"
	ScanStatusProcess  ScanStatus = "process"
	ScanStatusDone     ScanStatus = "done"
	ScanStatusFailed   ScanStatus = "failed"
	ScanStatusCanceled ScanStatus = "canceled"
)

// SAST is the persisted state of a job. Kept intentionally small (mirrors the
// dast table). `LastMessage` holds the latest Codex emit while running
// (reasoning / exec_command), or the error message on failure; `LastUpdate` is
// bumped alongside it so the Manager can watch it as a liveness/heartbeat signal.
type SAST struct {
	ID                string `gorm:"column:id;primaryKey;size:36" json:"id"`
	ProjectID         string `gorm:"column:project_id;size:36;not null;index:idx_sast_project" json:"project_id"` // UUIDv4 from the Manager
	ResultSwaggerPath string `gorm:"column:result_swagger_path;size:512" json:"result_swagger_path,omitempty"`
	ResultReportPath  string `gorm:"column:result_report_path;size:512" json:"result_report_path,omitempty"`
	// ResultSwaggerBaseURL is the base URL Codex verified against the source code
	// (it may correct a wrong/incomplete value the Manager supplied), e.g.
	// https://api.example.com/api/v1.
	ResultSwaggerBaseURL string     `gorm:"column:result_swagger_base_url;size:512" json:"result_swagger_base_url,omitempty"`
	Status               ScanStatus `gorm:"column:status;size:16;not null" json:"status"`
	// Progress (0-100) and Phase mirror the dast/report tables so the Manager can
	// surface real SAST progress instead of inferring it. LastMessage remains the
	// fine-grained activity heartbeat.
	Progress    int       `gorm:"column:progress;not null;default:0" json:"progress"`
	Phase       string    `gorm:"column:phase;size:64" json:"phase"`
	LastMessage string    `gorm:"column:last_message;type:text" json:"last_message"`
	LastUpdate  time.Time `gorm:"column:last_update;not null" json:"last_update"`
}

func (SAST) TableName() string { return "sast" }
