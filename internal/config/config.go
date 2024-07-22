package config

import (
	"log"

	"github.com/docker/docker/api/types/network"
	"github.com/ilyakaznacheev/cleanenv"
)

// Переменные окружения
type Config struct {
	Env             string `env:"ENV"`
	ListenPort      string `env:"LISTENPORT"`
	ImageName       string `env:"IMAGENAME"`
	RedirectURL     string `env:"REDIRECTURL"`
	Network         string `env:"DOCKERNETWORK"`
	RedirectPrefix  string `env:"REDIRECTPREFIX"`
	LBport          string `env:"LBPORT"`
	CookieName      string `env:"COOKIENAME"`
	ControllerHost  string `env:"CONTROLLERHOST"`
	EndpointsConfig map[string]*network.EndpointSettings
}

func MustLoad() (*Config, error) {
	var cfg Config

	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		log.Printf("couldn't parse the .env file: %v", err)
		return nil, err
	}

	cfg.EndpointsConfig = map[string]*network.EndpointSettings{
		cfg.Network: {
			NetworkID: cfg.Network,
		},
	}
	
	log.Print("config is loaded")
	return &cfg, nil
}
