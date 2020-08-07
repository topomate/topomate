package project

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

type Host struct {
	Hostname      string
	ContainerName string
	DockerImage   string
	Command       []string
	Files         []HostFile
	Links         []*NetInterface
	NextInterface int
}

type HostFile struct {
	HostPath      string
	ContainerPath string
}

type HostLinkItem struct {
	Host      *Host
	Interface *NetInterface
}

type HostLink struct {
	Router *LinkItem
	Host   *HostLinkItem
}

func NewHostLinkItem(host *Host) *HostLinkItem {
	ifName := fmt.Sprintf("eth%d", host.NextInterface)
	host.NextInterface++
	return &HostLinkItem{
		Host: host,
		Interface: &NetInterface{
			IfName: ifName,
			IP:     net.IPNet{},
			Speed:  10000,
		},
	}
}

// StartContainer starts the container
func (host *Host) StartContainer(wg *sync.WaitGroup) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	// Check if container already exists
	var containerID string
	flt := filters.NewArgs(filters.Arg("name", host.ContainerName))
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
		image := host.DockerImage
		// -ssh.bind :8083
		contCfg := &container.Config{
			Image:           image,
			Hostname:        host.Hostname,
			NetworkDisabled: true,
			Cmd:             host.Command,
		}
		resp, err := cli.ContainerCreate(ctx,
			contCfg, hostCfg, nil, nil, host.ContainerName)
		utils.Check(err)
		containerID = resp.ID
	} else { // container exists
		containerID = li[0].ID
	}

	// Copy files
	host.CopyFiles()

	// Start container
	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	if config.VFlag {
		fmt.Println(host.ContainerName, "started.")
	}
}

func (host *Host) StopContainer(wg *sync.WaitGroup) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	if err := cli.ContainerStop(ctx, host.ContainerName, nil); err != nil {
		panic(err)
	}
}

func (host *Host) CopyFiles() {
	for _, f := range host.Files {
		out, err := exec.Command(
			"docker",
			"cp",
			f.HostPath,
			host.ContainerName+":"+f.ContainerPath,
		).CombinedOutput()
		if err != nil {
			utils.Fatalln(string(out), err)
		}
	}
}
