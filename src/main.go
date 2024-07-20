package main

// info: https://ofstack.com/Server/42749/practice-of-using-golang-to-play-docker-api.html
// info: https://gin-gonic.com/docs/examples/param-in-path/

// WIP: Исключить вероятность создания контейнеров с одиниковыми именами
import (
	"context"

	"math/rand"
	"time"

	// "go/types"
	"io"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Переменные окружения
type ENV struct {
	ListenPort      string
	ImageName       string
	RedirectURL     string
	EndpointsConfig map[string]*network.EndpointSettings
	Environment     map[string]string
	Network         string
	RedirectPrefix  string
	LBport          string
	CookieName      string
	ControllerHost  string
}

// Параметры контейнера
type UContainer struct {
	Name    string
	Route   string
	ISExist bool
	CState  string
	CStatus string
	CID     string
}

func init() {
	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		log.Debug().
			Str("controler", "Env file not found").Send()
	}
}

func main() {

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	conf := func() ENV {
		var c ENV

		c.ListenPort, _ = os.LookupEnv("LISTENPORT")
		c.ImageName, _ = os.LookupEnv("IMAGENAME")

		c.Network, _ = os.LookupEnv("DOCKERNETWORK")

		c.EndpointsConfig = map[string]*network.EndpointSettings{
			c.Network: &network.EndpointSettings{
				NetworkID: c.Network,
			},
		}

		c.RedirectURL, _ = os.LookupEnv("REDIRECTURL")
		c.RedirectPrefix, _ = os.LookupEnv("REDIRECTPREFIX")
		c.LBport, _ = os.LookupEnv("LBPORT")
		c.CookieName, _ = os.LookupEnv("COOKIENAME")
		c.ControllerHost, _ = os.LookupEnv("CONTROLLERHOST")
		return c

	}()

	// Новый роутер
	router := gin.Default()
	// Контекст для gin
	ctxD := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// Роутер берет из параметров урла имя контенера
	// После чего создается новый контейнер
	// с параметрами
	router.GET("/", func(ctx *gin.Context) {
		var user_cont UContainer
		clientIP := ctx.ClientIP()

		user_cont.Route, err = ctx.Cookie(conf.CookieName)
		if err != nil || user_cont.Route == "" {
			log.Debug().
				Str("user", "cookie not found").
				Str("client", clientIP).
				Send()
			user_cont.Route = RandomRoute()
			log.Debug().
				Str("user", "new cookie").
				Str("cookie", user_cont.Route).
				Str("client", clientIP).
				Send()
		}
		user_cont.Name = user_cont.Route[0:15]
		log.Info().
			Str("route", user_cont.Route).
			Str("container_name", user_cont.Name).
			Str("client", clientIP).
			Msg("Trying to follow a new route")

		//user_cont.Name = ctx.Param("name")
		//user_cont.Name = ctx.Query("name")

		if user_cont.Exist(ctxD, cli) {
			log.Debug().
				Str("container", user_cont.CState).
				Str("route", user_cont.Route).
				Str("client", clientIP).
				Msg(user_cont.Route + "; State - " + user_cont.CState + "; Status - " + user_cont.CStatus + "; name - " + user_cont.Name)

		} else {
			log.Debug().
				Str("container", "not found").
				Str("client", clientIP).
				Msg("Container " + user_cont.Name + " not found")
		}

		//st := runContainer(ctxD, cli, n, conf)

		switch user_cont.CState {
		// Редиректим пользователя по маршруту сразу если контейнер работаем
		case "running":
			log.Debug().
				Str("container", user_cont.CState).
				Str("route", user_cont.Route).
				Str("client", clientIP).
				Msg("Container " + user_cont.Name + " is running. Redirect")

			ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", conf.ControllerHost, false, true)
			ctx.Redirect(307, conf.RedirectURL)
		case "exited":
			// Если контейнер остановлен - зупустить, дождаться запуска и сделать редирект
			log.Debug().
				Str("container", user_cont.CState).
				Str("route", user_cont.Route).
				Str("client", clientIP).
				Msg("Container " + user_cont.Name + " stopped. Attempt to launch")

			if user_cont.StartContainer(ctxD, cli) {
				ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", conf.ControllerHost, false, true)
				// WIP: убрать таймеры, передалать на канал
				time.Sleep(time.Second * 5)
				ctx.Redirect(307, conf.RedirectURL)
			} else {
				ctx.IndentedJSON(200, gin.H{
					"running": false,
				})
			}

		default:
			log.Debug().
				Str("contanier", "not found").
				Str("route", user_cont.Route).
				Str("client", clientIP).
				Msg("Container not found. Attempt to create")

			if !user_cont.ISExist {
				user_cont.PullImage(ctxD, cli, conf)

				if user_cont.CreateContainer(ctxD, cli, conf) {

					log.Debug().
						Str("container", "created").
						Str("route", user_cont.Route).
						Str("client", clientIP).
						Msg("Create new container with name " + user_cont.Name)

					ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", conf.ControllerHost, false, true)
					time.Sleep(time.Second * 7)
					ctx.Redirect(307, conf.RedirectURL)
				} else {

					log.Warn().
						Str("client", clientIP).
						Msg("Error to create container")

					ctx.SetCookie(conf.CookieName, "-", 3600, "/", conf.ControllerHost, false, true)
					ctx.IndentedJSON(200, gin.H{
						"err": true,
					})
				}
			}

		}
	})

	// тестовый роутер для чистки кук
	router.GET("/clean", func(ctx *gin.Context) {
		ctx.SetCookie(conf.CookieName, "", 3600, "/", conf.ControllerHost, false, true)
		ctx.IndentedJSON(200, gin.H{
			"state": "clean",
		})
		log.Debug().Msg(ctx.ClientIP() + " - cookie cleaned")
	})

	// Запуск сервера
	router.Run(":" + conf.ListenPort)
}

// Проверить существование контейнера
// При отсутсвии - создать.
// Если существует - запустить
// Возращает true если успешно
// func runContainer(ctx context.Context, cli *client.Client, name string, conf ENV) bool {

// 	if !exist {

// 		//imageName := "nginx:1.16.0-alpine"

// Тест - остановка контейнера. Возвращает true если успешно
func stopContainer(ctx context.Context, cli *client.Client, name string, ops string) bool {
	containes, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		log.Err(err)
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

		err = cli.ContainerStop(ctx, contID, container.StopOptions{})
		if err != nil {
			return false
		} else {
			return true
		}
	} else {
		return false
	}

}

func (cont *UContainer) Exist(ctx context.Context, cli *client.Client) bool {

	containes, err := cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		log.Err(err)
	}
	// Итерация по списку контенейров для поиска айдишника контейнера
	for _, c := range containes {
		// При балансировке может вернуться несколько имен. Берем первое
		for _, n := range c.Names {
			// log.Println(n)
			if n == "/"+cont.Name {
				log.Debug().
					Str("search", "/"+cont.Name).
					Str("found", n).
					Msg("Search for created containers. " + "Found: " + n + " like a " + "/" + cont.Name)
				cont.ISExist = true
				//cont.Route = c.ID
				cont.CState = c.State
				cont.CStatus = c.Status
				cont.CID = c.ID
				break
			}
		}
	}

	return cont.ISExist

}

func (cont *UContainer) PullImage(ctx context.Context, cli *client.Client, conf ENV) {

	out, err := cli.ImagePull(ctx, conf.ImageName, image.PullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, out)

}

func (cont *UContainer) CreateContainer(ctx context.Context, cli *client.Client, conf ENV) bool {
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: conf.ImageName,
		Labels: map[string]string{
			"traefik.enable": "true",
			"traefik.http.routers." + cont.Name + ".entrypoints": "web",
			//"traefik.http.services." + name + ".loadbalancer.server.port": "80",
			"traefik.http.services." + cont.Name + ".loadbalancer.server.port": conf.LBport,
			//	"traefik.docker.network":                                      "kiosk-int",
			"traefik.docker.network": conf.Network,

			//"traefik.http.routers." + name + ".rule": "PathPrefix(`/" + name + conf.RedirectPrefix + "`)",
			// Роут на основе префикса
			//"traefik.http.routers." + cont.Name + ".rule": "PathPrefix(`/`)",
			// Роут на основе кук
			"traefik.http.routers." + cont.Name + ".rule": "HeaderRegexp(`Cookie`, `.*" + conf.CookieName + "=" + cont.Route + ".*`)",
		},
		Env: []string{
			"LANG=ru_RU.UTF-8",
			"KEEP_APP_RUNNING=1",
			"DARK_MODE=1",
			"FF_OPEN_URL=ya.ru",
			"FF_KIOSK=1",
		},
		// ExposedPorts: nat.PortSet{
		// 	"5800": struct{}{},
		// },
	},
		// &container.HostConfig{
		// 	PortBindings: nat.PortMap{
		// 		nat.Port("5800"): []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "5800"}},
		// 	},
		// },
		nil,
		&network.NetworkingConfig{
			// Определение подключения к сети
			EndpointsConfig: conf.EndpointsConfig,
		}, nil, cont.Name)

	if err != nil {
		log.Err(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Err(err)
	} else {
		if err := cli.ContainerStart(ctx, cont.Name, container.StartOptions{}); err != nil {
			log.Err(err)
		}
	}

	return cont.Exist(ctx, cli)

}

func RandomRoute() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	s := make([]rune, 64)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func (cont *UContainer) StartContainer(ctx context.Context, cli *client.Client) bool {
	err := cli.ContainerStart(ctx, cont.CID, container.StartOptions{})
	if err != nil {
		return false
	} else {
		return true
	}
}
