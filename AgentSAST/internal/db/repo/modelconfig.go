package repo

import (
	"strings"

	"gorm.io/gorm"
)

// ModelConfig reads per-agent LLM assignments from the Manager-owned table.
type ModelConfig struct {
	db *gorm.DB
}

func NewModelConfig(db *gorm.DB) *ModelConfig {
	return &ModelConfig{db: db}
}

// EnabledModel returns the enabled model_name for agentType (sast|dast).
func (r *ModelConfig) EnabledModel(agentType string) (string, error) {
	var rec struct {
		ModelName string `gorm:"column:model_name"`
	}
	err := r.db.Table("AgentModelConfig").
		Select("model_name").
		Where("agent_type = ? AND enabled = 1", strings.ToLower(strings.TrimSpace(agentType))).
		Take(&rec).Error
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(rec.ModelName), nil
}
