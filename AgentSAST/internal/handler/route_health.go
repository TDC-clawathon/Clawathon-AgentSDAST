package handler

import (
	"agentsast/internal/db"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /api/sast/health
func (d *Deps) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	mysqlOK := db.Ping(d.DB) == nil
	minioOK := d.Store.Ping(ctx) == nil

	llm := "ok"
	if d.LLMBaseURL == "" || d.LLMAPIKey == "" {
		llm = "not_configured"
	}

	code := http.StatusOK
	if !mysqlOK || !minioOK {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{
		"agent": "AgentSAST",
		"ping": gin.H{
			"mysql": okStr(mysqlOK),
			"minio": okStr(minioOK),
			"llm":   llm,
		},
	})
}
