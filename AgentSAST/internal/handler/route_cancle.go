package handler

import (
	"agentsast/internal/db/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /api/sast/cancel?id=<uuid>
func (d *Deps) cancel(c *gin.Context) {
	id := c.Query("id")
	rec, err := d.Repo.Get(id)
	if err != nil {
		d.notFoundOr500(c, err)
		return
	}
	switch rec.Status {
	case model.ScanStatusDone, model.ScanStatusFailed, model.ScanStatusCanceled:
		c.JSON(http.StatusConflict, gin.H{"id": id, "status": rec.Status, "message": "already finished"})
		return
	}
	signaled := d.Jobs.Cancel(id)
	_ = d.Repo.Update(id, map[string]any{"status": model.ScanStatusCanceled, "last_message": "canceled by user"})
	c.JSON(http.StatusOK, gin.H{"id": id, "status": model.ScanStatusCanceled, "signaled": signaled})
}
