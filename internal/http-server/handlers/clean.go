package handlers

import (
	"context"
	"kiosk/internal/config"
	"kiosk/internal/lib/docker"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func Clean(ctx context.Context, zl *zerolog.Logger, cfg *config.Config, dockerCLI *docker.DockerClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetCookie(cfg.CookieName, "", 3600, "/", cfg.ControllerHost, false, true)
		c.JSON(http.StatusOK, gin.H{"state": "clean"})
		zl.Debug().Msg(c.ClientIP() + " - cookie cleaned")
	}
}
