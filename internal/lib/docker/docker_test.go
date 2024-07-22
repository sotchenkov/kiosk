package docker

import (
	"context"
	"kiosk/internal/config"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DockerTestSuite struct {
	suite.Suite
	cli           *DockerClient
	imageName     string
	containerName string
}

func (suite *DockerTestSuite) SetupSuite() {
	suite.cli = NewCLI()
	suite.imageName = "nginx:alpine"
	suite.containerName = "test-container"

	// Pull image with more detailed logging
	suite.T().Logf("Attempting to pull image: %s", suite.imageName)
	_, err := suite.cli.Client.ImagePull(context.Background(), suite.imageName, image.PullOptions{})
	if err != nil {
		suite.T().Fatal("Error pulling image:", err)
	}

	// Allow some time for the image to be pulled and recognized
	time.Sleep(15 * time.Second)

	// Verify the image exists locally
	_, _, err = suite.cli.Client.ImageInspectWithRaw(context.Background(), suite.imageName)
	if err != nil {
		suite.T().Fatal("Image not found locally after pulling:", err)
	} else {
		suite.T().Logf("Image %s found locally", suite.imageName)
	}
}

func (suite *DockerTestSuite) TestExist() {
	_, _, err := suite.cli.Client.ImageInspectWithRaw(context.Background(), suite.imageName)
	if err != nil {
		suite.T().Fatal("Image not found locally before creating container:", err)
	}

	_, err = suite.cli.Client.ContainerCreate(context.Background(), &container.Config{
		Image: suite.imageName,
	}, nil, nil, nil, suite.containerName)
	if err != nil {
		suite.T().Fatal("Error creating container:", err)
	}
	defer suite.cli.Client.ContainerRemove(context.Background(), suite.containerName, container.RemoveOptions{Force: true})

	uContainer := UContainer{Name: suite.containerName}
	exists := uContainer.Exist(context.Background(), &zerolog.Logger{}, suite.cli.Client)
	assert.True(suite.T(), exists)
	assert.Equal(suite.T(), suite.containerName, uContainer.Name)
}

func (suite *DockerTestSuite) TestStopContainer() {
	resp, err := suite.cli.Client.ContainerCreate(context.Background(), &container.Config{
		Image: suite.imageName,
	}, nil, nil, nil, suite.containerName)
	if err != nil {
		suite.T().Fatal(err)
	}
	defer suite.cli.Client.ContainerRemove(context.Background(), suite.containerName, container.RemoveOptions{Force: true})

	err = suite.cli.Client.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
	if err != nil {
		suite.T().Fatal(err)
	}

	uContainer := UContainer{}
	stopped := uContainer.StopContainer(context.Background(), &zerolog.Logger{}, suite.cli.Client, suite.containerName, "stop")
	assert.True(suite.T(), stopped)

	containerJSON, err := suite.cli.Client.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "exited", containerJSON.State.Status)
}

func (suite *DockerTestSuite) TestStartContainer() {
	resp, err := suite.cli.Client.ContainerCreate(context.Background(), &container.Config{
		Image: suite.imageName,
	}, nil, nil, nil, suite.containerName)
	if err != nil {
		suite.T().Fatal(err)
	}
	defer suite.cli.Client.ContainerRemove(context.Background(), suite.containerName, container.RemoveOptions{Force: true})

	err = suite.cli.Client.ContainerStop(context.Background(), resp.ID, container.StopOptions{})
	if err != nil {
		suite.T().Fatal(err)
	}

	uContainer := UContainer{CID: resp.ID}
	started := uContainer.StartContainer(context.Background(), suite.cli.Client)
	assert.True(suite.T(), started)

	containerJSON, err := suite.cli.Client.ContainerInspect(context.Background(), resp.ID)
	if err != nil {
		suite.T().Fatal(err)
	}
	assert.Equal(suite.T(), "running", containerJSON.State.Status)
}

func (suite *DockerTestSuite) TestCreateContainer() {
	logger := zerolog.New(nil)

	config := &config.Config{
		ImageName: suite.imageName,
		LBport:    "8080",
		Network:   "bridge",
	}

	uContainer := UContainer{Name: suite.containerName}
	created := uContainer.CreateContainer(context.Background(), &logger, suite.cli.Client, config)
	assert.True(suite.T(), created)
	assert.NotEmpty(suite.T(), uContainer.CID)

	defer suite.cli.Client.ContainerRemove(context.Background(), suite.containerName, container.RemoveOptions{Force: true})
}

func TestDockerTestSuite(t *testing.T) {
	suite.Run(t, new(DockerTestSuite))
}
