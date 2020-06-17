package config

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/utils"
)

type Router struct {
	Hostname      string
	ContainerName string
	Links         []*NetLink
}

func (r *Router) StartContainer(wg *sync.WaitGroup) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	// Check if container already exists
	var containerID string
	flt := filters.NewArgs(filters.Arg("name", r.ContainerName))
	li, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: flt,
	})
	if len(li) == 0 { // container does not exist yet
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:           "topomate-router",
			NetworkDisabled: true, // docker networking disabled as we use OVS
		}, &container.HostConfig{
			CapAdd: []string{"SYS_ADMIN", "NET_ADMIN"},
		}, nil, nil, r.ContainerName)
		utils.Check(err)
		containerID = resp.ID
	} else { // container exists
		containerID = li[0].ID
	}

	// Start container
	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

}

func (r *Router) StopContainer(wg *sync.WaitGroup) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	if err := cli.ContainerStop(ctx, r.ContainerName, nil); err != nil {
		panic(err)
	}
}
