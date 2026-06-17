package handler

import (
	"agentsast/internal/db/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type runRequest struct {
	ProjectID string   `json:"project_id" binding:"required"`
	BaseURL   string   `json:"base_url" binding:"required"`
	Model     string   `json:"model"`
	Modes     []string `json:"modes"`
}

// POST /api/sast/run
func (d *Deps) run(c *gin.Context) {
	var req runRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := d.Jobs.Start(req.ProjectID, req.BaseURL, req.Model, req.Modes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"id":         id,
		"project_id": req.ProjectID,
		"status":     model.ScanStatusNew,
	})
}
