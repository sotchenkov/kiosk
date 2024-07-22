package docker

import (
	"context"
	"io"
	"kiosk/internal/config"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

type DockerClient struct {
	Client *client.Client
}

type UContainer struct {
	Name    string
	Route   string
	ISExist bool
	CState  string
	CStatus string
	CID     string
}

// NewCLI создаёт новый клиент докера
func NewCLI() *DockerClient {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	return &DockerClient{Client: cli}
}

// Exist проверяет, существует ли контейнер с заданным именем
func (cont *UContainer) Exist(ctx context.Context, zl *zerolog.Logger, cli *client.Client) bool {
	// Получаем список всех контейнеров
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		zl.Err(err).Msg("Error when getting the list of containers")
		return false
	}

	// TODO: Эт будет за линейное время работать, можно использовать мапу, чтобы работало за константу
	// Итерация по списку контейнеров для поиска контейнера с нужным именем
	for _, c := range containers {
		// При балансировке может вернуться несколько имен. Берем первое
		for _, n := range c.Names {
			if n == "/"+cont.Name {
				zl.Debug().
					Str("search", "/"+cont.Name).
					Str("found", n).
					Msg("Search for created containers. " + "Found: " + n + " like a " + "/" + cont.Name)

				cont.ISExist = true
				cont.CState = c.State
				cont.CStatus = c.Status
				cont.CID = c.ID
				return true
			}
		}
	}
	return false
}

func (cont *UContainer) StopContainer(ctx context.Context, zl *zerolog.Logger, cli *client.Client, name string, ops string) bool {
	containes, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		zl.Err(err)
	}

	var contID string

	for _, c := range containes {
		for _, n := range c.Names {
			// log.Println(n)
			if n == "/"+name {
				contID = c.ID
				break
			}
		}
	}

	if ops == "stop" {
		if err := cli.ContainerStop(ctx, contID, container.StopOptions{}); err != nil {
			zl.Err(err).Msg("Failed to stop container")
			return false
		}
		return true
	}
	return false
}

func (cont *UContainer) StartContainer(ctx context.Context, cli *client.Client) bool {
	err := cli.ContainerStart(ctx, cont.CID, container.StartOptions{})
	if err != nil {
		return false
	} else {
		return true
	}
}

func (cont *UContainer) PullImage(ctx context.Context, cli *client.Client, cfg *config.Config) {

	out, err := cli.ImagePull(ctx, cfg.ImageName, image.PullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)

}

func (cont *UContainer) CreateContainer(ctx context.Context, zl *zerolog.Logger, cli *client.Client, cfg *config.Config) bool {
	labels := map[string]string{
		"traefik.enable": "true",
		"traefik.http.routers." + cont.Name + ".entrypoints":               "web",
		"traefik.http.services." + cont.Name + ".loadbalancer.server.port": cfg.LBport,
		"traefik.docker.network":                                           cfg.Network,
		"traefik.http.routers." + cont.Name + ".rule":                      "HeaderRegexp(`Cookie`, `.*" + cfg.CookieName + "=" + cont.Route + ".*`)",
	}

	envVars := []string{
		"LANG=ru_RU.UTF-8",
		"KEEP_APP_RUNNING=1",
		"DARK_MODE=1",
		"FF_OPEN_URL=ya.ru",
		"FF_KIOSK=1",
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:  cfg.ImageName,
		Labels: labels,
		Env:    envVars,
	}, nil, &network.NetworkingConfig{
		EndpointsConfig: cfg.EndpointsConfig,
	}, nil, cont.Name)

	if err != nil {
		zl.Err(err).Msg("Failed to create container")
		return false
	}

	cont.CID = resp.ID

	zl.Info().Str("ContainerID", resp.ID).Msg("Container created successfully")

	return true
}
