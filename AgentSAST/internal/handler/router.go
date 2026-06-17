// Package handler exposes the AgentSAST HTTP API under /api/sast.
package handler

import (
	"errors"
	"net/http"

	"agentsast/internal/db/repo"
	"agentsast/internal/service/job"
	"agentsast/internal/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Deps holds everything the handlers need.
type Deps struct {
	DB         *gorm.DB
	Repo       *repo.SAST
	Store      *store.Client
	Jobs       *job.Service
	LLMBaseURL string
	LLMAPIKey  string
}

// Router builds the Gin engine with the /api/sast routes and permissive CORS
// (so the static manager dashboard can call it from the browser).
func Router(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors())

	// Root health check for AgentBase Runtime (platform polls GET /health).
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	g := r.Group("/api/sast")
	g.GET("/health", d.health)
	g.POST("/run", d.run)
	g.GET("/status", d.status)
	g.GET("/result", d.result)
	g.POST("/cancel", d.cancel)
	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (d *Deps) notFoundOr500(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func okStr(ok bool) string {
	if ok {
		return "ok"
	}
	return "down"
}
