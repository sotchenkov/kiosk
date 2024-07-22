package main

import (
	"context"
	"fmt"
	"kiosk/internal/config"
	"kiosk/internal/http-server/handlers"
	"kiosk/internal/lib/docker"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg, err := config.MustLoad()
	if err != nil {
		os.Exit(1)
	}

	zl := setupLogger(cfg.Env)
	zl.Info().Msg("logger is configured")

	dockerCLI := docker.NewCLI()
	zl.Info().Msg("docker client has been created")

	router := gin.Default()
	ctx := context.Background()

	router.GET("/", handlers.RootHandler(ctx, zl, cfg, dockerCLI))
	router.GET("/clean", handlers.Clean(ctx, zl, cfg, dockerCLI))
	zl.Info().Msg("router and handlers has been created")

	zl.Info().Msg("starting server...")
	fmt.Println(cfg.ListenPort)
	router.Run(":" + cfg.ListenPort)
}

func setupLogger(env string) *zerolog.Logger {
	var logger zerolog.Logger

	switch env {
	case envLocal:
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)
	case envProd:
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	default:
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	}

	return &logger
}
