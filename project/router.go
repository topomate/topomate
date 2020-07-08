package project

import (
	"context"
	"net"
	"os/exec"
	"sync"

	"github.com/rahveiz/topomate/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/utils"
)

type BGPNbr struct {
	RemoteAS     int
	UpdateSource string
	ConnCheck    bool
	NextHopSelf  bool
	IfName       string
	RouteMapsIn  []string
	RouteMapsOut []string
}

type Router struct {
	ID            int
	Hostname      string
	ContainerName string
	Loopback      []net.IPNet
	Links         []*NetInterface
	Neighbors     map[string]BGPNbr
	NextInterface int
}

func (r *Router) StartContainer(wg *sync.WaitGroup, configPath string) {
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
			Image:           config.DockerRouterImage,
			Hostname:        r.Hostname,
			NetworkDisabled: true, // docker networking disabled as we use OVS
		}, &container.HostConfig{
			CapAdd: []string{"SYS_ADMIN", "NET_ADMIN"},
		}, nil, nil, r.ContainerName)
		utils.Check(err)
		containerID = resp.ID
	} else { // container exists
		containerID = li[0].ID
	}

	// If configPath is set, copy the configuration into the container
	if configPath != "" {
		r.CopyConfig(configPath)
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

func (r *Router) CopyConfig(configPath string) {
	_, err := exec.Command(
		"docker",
		"cp",
		configPath,
		r.ContainerName+":/etc/frr/frr.conf",
	).CombinedOutput()
	if err != nil {
		utils.Fatalln(err)
	}
}
