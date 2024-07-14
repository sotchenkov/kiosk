package main

// info: https://ofstack.com/Server/42749/practice-of-using-golang-to-play-docker-api.html
// info: https://gin-gonic.com/docs/examples/param-in-path/

// WIP: добавить переменные окружения
// WIP: больше контроля над запускаемым контейнером. Добавить проверки состояния и пересоздание контенера
import (
	"context"
	"log"
	"time"

	// "go/types"
	"io"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func main() {
	// Новый роутер
	router := gin.Default()
	// Контекст для gin
	ctxD := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// router.GET("/demo", func(ctx *gin.Context) {
	// 	ctx.IndentedJSON(200, gin.H{
	// 		"data": true,
	// 	})
	// })

	// Роутер берет из урла имя контенера
	// После чего создается новый контейнер
	// с параметрами
	router.GET("/:name", func(ctx *gin.Context) {
		n := ctx.Param("name")
		st := runContainer(ctxD, cli, n)
		//ctx.IndentedJSON(200, gin.H{
		//	"state": st,
		//})
		if st {
			// Задержка на запуск контейнера. Заменить на проверку состояния
			time.Sleep(time.Second * 1)
			// Редирект на УРЛ с контейнером
			ctx.Redirect(301, "http://127.0.0.1/"+n+"link")
		} else {
			ctx.IndentedJSON(200, gin.H{
				"err": true,
			})
		}
	})

	// тестовый роутер для остановки контенера
	router.GET("/:name/:ops", func(ctx *gin.Context) {
		n := ctx.Param("name")
		s := ctx.Param("ops")
		st := stopContainer(ctxD, cli, n, s)
		ctx.IndentedJSON(200, gin.H{
			"state": st,
		})
	})

	// Запуск сервера
	router.Run(":8099")
}

// Проверить существование контейнера
// При отсутсвии - создать.
// Если существует - запустить
// Возращает true если успешно
func runContainer(ctx context.Context, cli *client.Client, name string) bool {

	containes, err := cli.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		log.Fatalln(err)
	}

	var contID string
	exist := false

	for _, c := range containes {
		for _, n := range c.Names {
			// log.Println(n)
			if n == "/"+name {
				log.Println("Found: ", n, " like a ", "/"+name)
				exist = true
				contID = c.ID
				break
			}
		}
	}

	if !exist {

		imageName := "nginx:1.16.0-alpine"

		out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			panic(err)
		}
		io.Copy(os.Stdout, out)

		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image: imageName,
			Labels: map[string]string{
				"traefik.enable": "true",
				"traefik.http.routers." + name + ".entrypoints":               "web",
				"traefik.http.services." + name + ".loadbalancer.server.port": "80",
				"traefik.docker.network":                                      "kiosk-int",
				"traefik.http.routers." + name + ".rule":                      "Path(`/" + name + "link`)",
			},
		}, nil, &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"kiosk-gin-proxy_kiosk-int": &network.EndpointSettings{
					NetworkID: "kiosk-gin-proxy_kiosk-int",
				},
			},
		}, nil, name)
		if err != nil {
			panic(err)
		}

		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			panic(err)
		}
	} else {
		if err := cli.ContainerStart(ctx, contID, container.StartOptions{}); err != nil {
			panic(err)
		}
	}
	return true
}

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
