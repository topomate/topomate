package project

import (
	"context"
	"fmt"
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

type AddressFamily struct {
	IPv4  bool
	IPv6  bool
	VPNv4 bool
	VPNv6 bool
}

// BGPNbr represents a neighbor configuration for a given router
type BGPNbr struct {
	RemoteAS     int
	UpdateSource string
	ConnCheck    bool
	NextHopSelf  bool
	IfName       string
	RouteMapsIn  []string
	RouteMapsOut []string
	AF           AddressFamily
	RRClient     bool
	RSClient     bool
}

// Router contains informations needed to configure a router.
// It contains elements relative to the container and to the FRR configuration.
type Router struct {
	ID            int
	Hostname      string
	ContainerName string
	Loopback      []net.IPNet
	Links         []*NetInterface
	Neighbors     map[string]*BGPNbr
	NextInterface int
}

func (r *Router) LoID() string {
	if len(r.Loopback) == 0 {
		return ""
	}
	return r.Loopback[0].IP.String()
}

// StartContainer starts the container for the router. If configPath is set,
// it also copies the configuration file from the configured directory to
// the container
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
	if err != nil {
		utils.Fatalln(err)
	}
	if len(li) == 0 { // container does not exist yet
		hostCfg := &container.HostConfig{
			CapAdd: []string{"SYS_ADMIN", "NET_ADMIN"},
		}
		// if configPath != "" {
		// 	hostCfg.Mounts = []mount.Mount{
		// 		{
		// 			Type:   mount.TypeBind,
		// 			Source: configPath,
		// 			Target: "/etc/frr/frr.conf",
		// 		},
		// 	}
		// }
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:           config.DockerRouterImage,
			Hostname:        r.Hostname,
			NetworkDisabled: true, // docker networking disabled as we use OVS
		}, hostCfg, nil, nil, r.ContainerName)
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

	if config.VFlag {
		fmt.Println(r.ContainerName, "started.")
	}

}

// StopContainer stops the router container
func (r *Router) StopContainer(wg *sync.WaitGroup, configPath string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	if configPath != "" {
		r.SaveConfig(configPath)
	}

	if err := cli.ContainerStop(ctx, r.ContainerName, nil); err != nil {
		panic(err)
	}
}

// CopyConfig copies the configuration file configPath to the configuration
// directory in the container file system.
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

func (r *Router) SaveConfig(configPath string) {
	_, err := exec.Command(
		"docker",
		"cp",
		r.ContainerName+":/etc/frr/frr.conf",
		configPath,
	).CombinedOutput()
	if err != nil {
		utils.Fatalln(err)
	}
}
