// Package repo is the data-access layer for SAST jobs.
package repo

import (
	"time"

	"agentsast/internal/db/model"

	"gorm.io/gorm"
)

type SAST struct{ db *gorm.DB }

func NewSAST(db *gorm.DB) *SAST { return &SAST{db: db} }

func (r *SAST) Create(rec *model.SAST) error {
	rec.LastUpdate = time.Now()
	return r.db.Create(rec).Error
}

func (r *SAST) Get(id string) (*model.SAST, error) {
	var rec model.SAST
	if err := r.db.First(&rec, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

// Update patches columns and refreshes last_update.
func (r *SAST) Update(id string, fields map[string]any) error {
	fields["last_update"] = time.Now()
	return r.db.Model(&model.SAST{}).Where("id = ?", id).Updates(fields).Error
}

// SetMessage records the latest activity (or error) as last_message and bumps
// last_update — the heartbeat the Manager watches.
func (r *SAST) SetMessage(id, msg string) error {
	return r.db.Model(&model.SAST{}).Where("id = ?", id).
		Updates(map[string]any{"last_message": msg, "last_update": time.Now()}).Error
}

// SetProgress records the structured phase + progress (0-100) the Manager reads.
func (r *SAST) SetProgress(id, phase string, progress int) error {
	return r.db.Model(&model.SAST{}).Where("id = ?", id).
		Updates(map[string]any{"phase": phase, "progress": progress, "last_update": time.Now()}).Error
}
