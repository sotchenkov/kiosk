package handlers

import (
	"context"
	"kiosk/internal/config"
	"kiosk/internal/lib/docker"
	"kiosk/internal/lib/random"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// RootHandler возвращает обработчик для корневого маршрута.
func RootHandler(ctx context.Context, zl *zerolog.Logger, cfg *config.Config, dockerCLI *docker.DockerClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		userContainer := initializeUserContainer(ctx, c, cfg, zl, dockerCLI, clientIP)
		handleContainerState(ctx, c, zl, cfg, dockerCLI, userContainer, clientIP)
	}
}

// initializeUserContainer инициализирует контейнер пользователя
func initializeUserContainer(ctx context.Context, c *gin.Context, cfg *config.Config, zl *zerolog.Logger, dockerCLI *docker.DockerClient, clientIP string) docker.UContainer {
	userContainer := docker.UContainer{}
	var err error

	userContainer.Route, err = c.Cookie(cfg.CookieName)
	if err != nil || userContainer.Route == "" {
		zl.Debug().Str("user", "cookie not found").Str("client", clientIP).Send()
		userContainer.Route = random.NewRandomString(65)
		zl.Debug().Str("user", "new cookie").Str("cookie", userContainer.Route).Str("client", clientIP).Send()
	}

	userContainer.Name = userContainer.Route[:15]
	if userContainer.Exist(ctx, zl, dockerCLI.Client) {
		zl.Info().Str("route", userContainer.Route).Str("container_name", userContainer.Name).Str("client", clientIP).
			Msg("Using existing container")
		return userContainer
	}

	zl.Info().Str("route", userContainer.Route).Str("container_name", userContainer.Name).Str("client", clientIP).
		Msg("Trying to follow a new route")

	return userContainer
}

// handleContainerState получает состояние контейнера
func handleContainerState(ctx context.Context, c *gin.Context, zl *zerolog.Logger, cfg *config.Config, dockerCLI *docker.DockerClient, userContainer docker.UContainer, clientIP string) {
	if userContainer.Exist(ctx, zl, dockerCLI.Client) {
		zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
			Msg(userContainer.Route + "; State - " + userContainer.CState + "; Status - " + userContainer.CStatus + "; name - " + userContainer.Name)
	} else {
		zl.Debug().Str("container", "not found").Str("client", clientIP).Msg("Container " + userContainer.Name + " not found")
	}

	switch userContainer.CState {
	case "running":
		handleRunningContainer(c, zl, cfg, userContainer, clientIP)
	case "exited":
		handleExitedContainer(ctx, c, zl, cfg, dockerCLI, userContainer, clientIP)
	default:
		handleContainerNotFound(ctx, c, zl, cfg, dockerCLI, userContainer, clientIP)
	}
}

func handleRunningContainer(c *gin.Context, zl *zerolog.Logger, cfg *config.Config, userContainer docker.UContainer, clientIP string) {
	zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
		Msg("Container " + userContainer.Name + " is running. Redirect")
	c.SetCookie(cfg.CookieName, userContainer.Route, 3600, cfg.RedirectURL, cfg.ControllerHost, false, true)
	c.Redirect(307, cfg.RedirectURL)
}

func handleExitedContainer(ctx context.Context, c *gin.Context, zl *zerolog.Logger, cfg *config.Config, dockerCLI *docker.DockerClient, userContainer docker.UContainer, clientIP string) {
	if !userContainer.StartContainer(ctx, dockerCLI.Client) {
		zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
			Msg("Container " + userContainer.Name + " exited and cannot be started")
		return
	}

	zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
		Msg("Container " + userContainer.Name + " exited, now started")

	readyChan := make(chan bool)
	go docker.ListenContainerStart(ctx, zl, dockerCLI.Client, userContainer.Name, cfg, readyChan)

	select {
	case <-readyChan:
		zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
			Msg("Container " + userContainer.Name + " is ready. Redirect")
		c.SetCookie(cfg.CookieName, userContainer.Route, 3600, cfg.RedirectURL, cfg.ControllerHost, false, true)
		c.Redirect(307, cfg.RedirectURL)
	case <-time.After(30 * time.Second):
		zl.Debug().Str("container", userContainer.CState).Str("route", userContainer.Route).Str("client", clientIP).
			Msg("Container " + userContainer.Name + " took too long to start")
	}
}

func handleContainerNotFound(ctx context.Context, c *gin.Context, zl *zerolog.Logger, cfg *config.Config, dockerCLI *docker.DockerClient, userContainer docker.UContainer, clientIP string) {
	zl.Debug().Str("container", "not found").Str("client", clientIP).Msg("Container " + userContainer.Name + " not found")

	if userContainer.CreateContainer(ctx, zl, dockerCLI.Client, cfg) {
		zl.Debug().Str("container", "created").Str("route", userContainer.Route).Str("client", clientIP).
			Msg("Container " + userContainer.Name + " created")
		if !userContainer.StartContainer(ctx, dockerCLI.Client) {
			zl.Debug().Str("container", "not started").Str("route", userContainer.Route).Str("client", clientIP).
				Msg("Container " + userContainer.Name + " could not be started")
			return
		}

		readyChan := make(chan bool)
		go docker.ListenContainerStart(ctx, zl, dockerCLI.Client, userContainer.Name, cfg, readyChan)

		select {
		case <-readyChan:
			zl.Debug().Str("container", "running").Str("route", userContainer.Route).Str("client", clientIP).
				Msg("Container " + userContainer.Name + " is running. Redirect")
			c.SetCookie(cfg.CookieName, userContainer.Route, 3600, cfg.RedirectURL, cfg.ControllerHost, false, true)
			c.Redirect(307, cfg.RedirectURL)
		case <-time.After(30 * time.Second):
			zl.Debug().Str("container", "timeout").Str("route", userContainer.Route).Str("client", clientIP).
				Msg("Container " + userContainer.Name + " took too long to start")
		}
	} else {
		zl.Debug().Str("container", "not created").Str("route", userContainer.Route).Str("client", clientIP).
			Msg("Container " + userContainer.Name + " could not be created")
	}
}
