/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/rahveiz/topomate/internal/ovsdocker"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Removes elements created by topomate (interfaces, containers).",
	Run: func(cmd *cobra.Command, args []string) {
		cleanContainers()
		cleanOVS()
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func cleanContainers() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println("Stopping and removing containers...")
	var wg sync.WaitGroup
	for _, container := range containers {
		if container.Image == config.DockerRSImage || container.Image == config.DockerRouterImage {
			wg.Add(1)
			go func(w *sync.WaitGroup, id string) {
				if err := cli.ContainerStop(ctx, id, nil); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				if err := cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{}); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				w.Done()
			}(&wg, container.ID)
		}

	}
	wg.Wait()
	fmt.Println("Done.")
}

func cleanOVS() {
	cli := ovs.New(ovs.Sudo())
	bridges, err := cli.VSwitch.ListBridges()
	if err != nil {
		utils.Fatalln(err)
	}
	for _, br := range bridges {
		ports, err := cli.VSwitch.ListPorts(br)
		if err != nil {
			utils.Fatalln(err)
		}
		for _, p := range ports {
			ovsdocker.ExecLink("del", "dev", p)
		}
		cli.VSwitch.DeleteBridge(br)
	}
}
