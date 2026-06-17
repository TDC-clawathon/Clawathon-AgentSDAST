package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /api/sast/status?id=<uuid>
func (d *Deps) status(c *gin.Context) {
	rec, err := d.Repo.Get(c.Query("id"))
	if err != nil {
		d.notFoundOr500(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                      rec.ID,
		"project_id":              rec.ProjectID,
		"status":                  rec.Status,
		"progress":                rec.Progress,
		"phase":                   rec.Phase,
		"result_swagger_base_url": rec.ResultSwaggerBaseURL,
		"last_message":            rec.LastMessage,
		"last_update":             rec.LastUpdate,
	})
}
