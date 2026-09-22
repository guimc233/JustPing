package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/buildinfo"
)

// getVersion reports the running Host version. Unauthenticated so the public UI
// footer can display it.
func getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": buildinfo.Version})
}
