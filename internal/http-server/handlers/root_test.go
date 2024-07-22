package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"kiosk/internal/config"
	"kiosk/internal/lib/docker"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Функция для настройки конфигурации, клиента Docker, логгера и Gin Engine
func setupTestEnvironment() (context.Context, *zerolog.Logger, *config.Config, *docker.DockerClient, *gin.Engine) {
	cfg := &config.Config{
		CookieName:     "test_cookie",
		RedirectURL:    "http://127.0.0.1",
		ControllerHost: "localhost",
		ImageName:      "jlesage/firefox",
		ListenPort:     "8099",
		LBport:         "5800",
	}

	dockerCLI := docker.NewCLI()

	zl := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)

	ctx := context.Background()

	r := gin.Default()

	return ctx, &zl, cfg, dockerCLI, r
}

// Функция для извлечения значения куки из ответа
func getCookieValue(response *httptest.ResponseRecorder, cookieName string) string {
	cookies := response.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			return cookie.Value
		}
	}
	return ""
}

// Функция для удаления контейнера
func removeContainer(ctx context.Context, dockerCLI *docker.DockerClient, containerName string) {
	// Остановка контейнера
	if err := dockerCLI.Client.ContainerStop(ctx, containerName, container.StopOptions{}); err != nil {
		panic("Failed to stop container: " + err.Error())
	}

	// Удаление контейнера
	if err := dockerCLI.Client.ContainerRemove(ctx, containerName, container.RemoveOptions{}); err != nil {
		panic("Failed to remove container: " + err.Error())
	}
}

// Тест для проверки создания контейнера, когда кука отсутствует
func TestCreateContainerWhenCookieNotPresent(t *testing.T) {
	// Настройка тестовой среды
	ctx, zl, cfg, dockerCLI, r := setupTestEnvironment()
	r.GET("/", RootHandler(ctx, zl, cfg, dockerCLI))

	// Создание запроса без куки
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Запуск запроса и запись ответа
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверка, что ответ имеет статус редиректа
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)

	// Извлечение куки из ответа
	cookieValue := getCookieValue(w, cfg.CookieName)
	assert.NotEmpty(t, cookieValue, "Cookie should be set")

	// Проверка, что контейнер существует
	containerName := cookieValue[0:15]
	containerExists, err := dockerCLI.Client.ContainerInspect(ctx, containerName)
	if err != nil {
		t.Fatalf("Failed to inspect container: %v", err)
	}
	assert.True(t, containerExists.ID != "", "Container should exist")

	// Удаление контейнера после теста
	removeContainer(ctx, dockerCLI, containerName)
}

// Тест для проверки поведения с установленной кукой
func TestRequestWithCookie(t *testing.T) {
	// Настройка тестовой среды
	ctx, zl, cfg, dockerCLI, r := setupTestEnvironment()
	r.GET("/", RootHandler(ctx, zl, cfg, dockerCLI))

	// Создание первоначального запроса для установки куки
	initialReq, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Запуск первоначального запроса и запись ответа
	initialResp := httptest.NewRecorder()
	r.ServeHTTP(initialResp, initialReq)

	// Извлечение куки из первоначального ответа
	cookieValue := getCookieValue(initialResp, cfg.CookieName)
	if cookieValue == "" {
		t.Fatal("Initial request did not set the cookie")
	}

	// Создание нового запроса с установленной кукой
	reqWithCookie, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqWithCookie.AddCookie(&http.Cookie{
		Name:  cfg.CookieName,
		Value: cookieValue,
	})

	// Запуск запроса и запись ответа
	w := httptest.NewRecorder()
	r.ServeHTTP(w, reqWithCookie)

	// Проверка, что ответ имеет статус редиректа
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)

	// Проверка, что редирект происходит на ожидаемый URL
	expectedRedirectURL := cfg.RedirectURL
	assert.Equal(t, expectedRedirectURL, w.Result().Header.Get("Location"), "Expected redirect URL did not match")

	// Удаление контейнера после теста
	removeContainer(ctx, dockerCLI, cookieValue[0:15])
}
