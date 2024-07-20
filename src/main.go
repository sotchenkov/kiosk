package main

// info: https://ofstack.com/Server/42749/practice-of-using-golang-to-play-docker-api.html
// info: https://gin-gonic.com/docs/examples/param-in-path/

// WIP: добавить переменные окружения
// WIP: больше контроля над запускаемым контейнером. Добавить проверки состояния и пересоздание контенера
import (
	"context"
	"log"
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
		log.Print("No .env file found")
	}
}

func main() {

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

		user_cont.Route, err = ctx.Cookie(conf.CookieName)
		if err != nil || user_cont.Route == "" {
			log.Println("Cookie no set")
			user_cont.Route = RandomRoute()
			log.Println("Set ", user_cont.Route)
		}
		user_cont.Name = user_cont.Route[0:15]
		log.Println("Cookie: ", user_cont.Route, " Name - ", user_cont.Name)

		//user_cont.Name = ctx.Param("name")
		//user_cont.Name = ctx.Query("name")

		if user_cont.Exist(ctxD, cli) {
			log.Println(user_cont.Route, " State - ", user_cont.CState, " Status - ", user_cont.CStatus, "name - ", user_cont.Name)

		} else {
			log.Println("Container " + user_cont.Name + " not found")
		}

		//st := runContainer(ctxD, cli, n, conf)

		switch user_cont.CState {
		// Редиректим пользователя по маршруту сразу если контейнер работаем
		case "running":
			ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", "127.0.0.1", false, true)
			ctx.Redirect(307, conf.RedirectURL)
		case "exited":
			// Если контейнер остановлен - зупустить, дождаться запуска и сделать редирект
			log.Println("conteiner stopped")
			if user_cont.StartContaner(ctxD, cli) {
				ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", "127.0.0.1", false, true)
				// WIP: убрать таймеры, передалать на канал
				time.Sleep(time.Second * 5)
				ctx.Redirect(307, conf.RedirectURL)
			} else {
				ctx.IndentedJSON(200, gin.H{
					"running": false,
				})
			}

		default:

			if !user_cont.ISExist {
				user_cont.PullImage(ctxD, cli, conf)

				if user_cont.CreateContaner(ctxD, cli, conf) {
					log.Println("Create container - ", user_cont.Name, "route -", user_cont.Route)
					ctx.SetCookie(conf.CookieName, user_cont.Route, 3600, "/", "127.0.0.1", false, true)
					time.Sleep(time.Second * 7)
					ctx.Redirect(307, conf.RedirectURL)
				} else {
					log.Println("Error to create container")
					ctx.SetCookie(conf.CookieName, "-", 3600, "/", "127.0.0.1", false, true)
					ctx.IndentedJSON(200, gin.H{
						"err": true,
					})
				}
			}

		}
	})

	// тестовый роутер для остановки контенера
	router.GET("/clean", func(ctx *gin.Context) {
		ctx.SetCookie(conf.CookieName, "", 3600, "/", "127.0.0.1", false, true)
		ctx.IndentedJSON(200, gin.H{
			"state": "clean",
		})
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
		log.Fatalln(err)
	}

	var contID string

	for _, c := range containes {
		for _, n := range c.Names {
			log.Println(n)
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
		log.Fatalln(err)
	}
	// Итерация по списку контенейров для поиска айдишника контейнера
	for _, c := range containes {
		// При балансировке может вернуться несколько имен. Берем первое
		for _, n := range c.Names {
			// log.Println(n)
			if n == "/"+cont.Name {
				log.Println("Found: ", n, " like a ", "/"+cont.Name)
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

func (cont *UContainer) CreateContaner(ctx context.Context, cli *client.Client, conf ENV) bool {
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
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	} else {
		if err := cli.ContainerStart(ctx, cont.Name, container.StartOptions{}); err != nil {
			panic(err)
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

func (cont *UContainer) StartContaner(ctx context.Context, cli *client.Client) bool {
	err := cli.ContainerStart(ctx, cont.CID, container.StartOptions{})
	if err != nil {
		return false
	} else {
		return true
	}
}
