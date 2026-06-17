package handler

import (
	"agentsast/internal/db/model"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /api/sast/result?id=<uuid> — returns the enriched swagger + report CONTENT
// (read from MinIO), only when the job finished successfully.
func (d *Deps) result(c *gin.Context) {
	rec, err := d.Repo.Get(c.Query("id"))
	if err != nil {
		d.notFoundOr500(c, err)
		return
	}
	if rec.Status != model.ScanStatusDone {
		c.JSON(http.StatusConflict, gin.H{"error": "result not ready", "status": rec.Status})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	swagger, err := d.Store.GetText(ctx, rec.ProjectID+"/sast/openapi.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read swagger: " + err.Error()})
		return
	}
	report, err := d.Store.GetText(ctx, rec.ProjectID+"/sast/report.md")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read report: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"swagger": swagger, "report": report})
}
