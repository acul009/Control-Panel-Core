package docker

import (
	"context"
	"controlPanel/scheduling"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	dockerErrors "github.com/docker/docker/errdefs"
)

type Docker struct {
	cli           *client.Client
	ctx           context.Context
	schedulerName string
}

func CreateDockerScheduler(schedulerName string) (*Docker, error) {
	var cli *client.Client
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, fmt.Errorf("Error creating docker scheduler: %w", err)
	}

	return &Docker{
		cli:           cli,
		schedulerName: schedulerName,
		ctx:           context.Background(),
	}, nil
}

func (docker *Docker) Schedule(deployment scheduling.Deployment) error {
	var err error = docker.createContainer(deployment, true)
	switch err.(type) {

	case dockerErrors.ErrConflict:
		docker.deleteContainer(deployment.Selector)
		err = docker.createContainer(deployment, false)
	}

	if err != nil {
		err = fmt.Errorf("Error scheduling deployment: %w", err)
	}

	err = docker.startContainer(deployment.Selector)

	if err != nil {
		err = fmt.Errorf("Error starting deployment: %w", err)
	}

	return err
}

func (docker *Docker) createContainer(deployment scheduling.Deployment, pullImage bool) error {
	var err error
	if pullImage {
		err = docker.pullImage(deployment.Image)
	}

	if err != nil {
		return fmt.Errorf("Error requesting image: %w", err)
	}

	_, err = docker.cli.ContainerCreate(docker.ctx, &container.Config{
		Image: deployment.Image,
		Labels: map[string]string{
			"manager": docker.schedulerName,
			"module":  deployment.Module,
			"name":    deployment.Name,
		},
	},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Tmpfs: map[string]string{
				"/tmp": "rw",
			},
		},
		nil, nil,
		deployment.GetIdentifier(docker.schedulerName),
	)

	return err
}

func (docker *Docker) pullImage(image string) error {
	reader, err := docker.cli.ImagePull(docker.ctx, image, types.ImagePullOptions{})
	defer reader.Close()
	var readerError error = nil
	var buf []byte = make([]byte, 1024)
	for ; readerError != io.EOF; _, readerError = reader.Read(buf) {
	}

	if err != nil {
		return fmt.Errorf("Error pulling image: %w", err)
	}
	return nil
}

func (docker *Docker) deleteContainer(selector scheduling.Selector) error {
	err := docker.cli.ContainerRemove(docker.ctx, selector.GetIdentifier(docker.schedulerName), types.ContainerRemoveOptions{
		Force: true,
	})

	if err != nil {
		return fmt.Errorf("Error deleting container: %w", err)
	}

	return nil
}

func (docker *Docker) startContainer(selector scheduling.Selector) error {
	err := docker.cli.ContainerStart(docker.ctx, selector.GetIdentifier(docker.schedulerName), types.ContainerStartOptions{})

	if err != nil {
		return fmt.Errorf("Error starting container: %w", err)
	}

	return nil
}

func (docker *Docker) Unschedule(selector scheduling.Selector) error {
	err := docker.deleteContainer(selector)
	if err != nil {
		return fmt.Errorf("Error unscheduling deployment: %w", err)
	}
	return nil
}

func (docker *Docker) ListDeployments() ([]scheduling.Deployment, error) {
	var containers []types.Container
	var err error

	containers, err = docker.cli.ContainerList(docker.ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.KeyValuePair{
				Key:   "label",
				Value: "manager=" + docker.schedulerName,
			},
		),
	})

	if err != nil {
		return make([]scheduling.Deployment, 0), fmt.Errorf("Error loading Deployments: %w", err)
	}

	var deployments []scheduling.Deployment = make([]scheduling.Deployment, 0, len(containers))

	for _, container := range containers {

		if container.Labels["manager"] != docker.schedulerName {
			continue
		}

		deployments = append(deployments, containerToDeployment(container))
	}
	return deployments, nil
}

func containerToDeployment(container types.Container) scheduling.Deployment {
	var volumes []string = make([]string, 0, len(container.Mounts))
	for _, volume := range container.Mounts {
		volumes = append(volumes, volume.Destination)
	}

	return scheduling.Deployment{
		Selector: scheduling.Selector{
			Name:   container.Labels["name"],
			Module: container.Labels["module"],
		},
		Image:   container.Image,
		Volumes: volumes,
	}
}

func (docker *Docker) getDeployment(selector scheduling.Selector) (scheduling.Deployment, error) {

	containerList, err := docker.cli.ContainerList(docker.ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.KeyValuePair{
				Key:   "name",
				Value: selector.GetIdentifier(docker.schedulerName),
			},
		),
	})

	if err != nil {
		return scheduling.Deployment{}, fmt.Errorf("Error getting deployment: %w", err)
	}

	if len(containerList) == 0 {
		return scheduling.Deployment{}, fmt.Errorf("No deployment found")
	}

	return containerToDeployment(containerList[0]), nil
}
