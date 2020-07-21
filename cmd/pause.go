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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/internal/ovsdocker"
	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"
)

// pauseCmd represents the pause command
var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current project",
	Long: `Pause the current project by stopping the containers and removing
the veth pairs. OVS bridges will be kept.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("pause called")
		if len(args) > 0 {
			pauseContainers(args[0])
		} else {
			pauseContainers("")
		}
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pauseCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pauseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func pauseContainers(name string) {
	content, err := ioutil.ReadFile(utils.GetDirectoryFromKey("MainDir", "") + "/links.json")
	if err != nil {
		utils.Fatalln(err)
	}
	m := ovsdocker.OVSBulk{}
	err = json.Unmarshal(content, &m)
	if err != nil {
		utils.Fatalln(err)
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)
	c := ovs.New(ovs.Sudo())

	// Stop container(s)

	// If container name is specified, stop the container
	if name != "" {
		if err := cli.ContainerStop(ctx, name, nil); err != nil {
			panic(err)
		}
		for _, v := range m[name] {
			if err := c.VSwitch.DeletePort(v.Bridge, v.HostIface); err != nil {
				utils.Fatalln(err)
			}
			// d.Portname = strings.TrimSuffix(v.HostIface, "_l")
			// d.AddPort(v.Bridge, v.ContainerIface, v.Settings, nil, true)
		}
	} else { // Name not specified, stop all the containers
		wg := sync.WaitGroup{}
		for cName, lks := range m {
			wg.Add(1)
			go func(w *sync.WaitGroup, _c *ovs.Client, _ctx *context.Context,
				name string, links []ovsdocker.OVSInterface) {
				if err := cli.ContainerStop(*_ctx, name, nil); err != nil {
					panic(err)
				}
				for _, v := range links {
					if err := _c.VSwitch.DeletePort(v.Bridge, v.HostIface); err != nil {
						utils.Fatalln(err)
					}
				}
				w.Done()
			}(&wg, c, &ctx, cName, lks)
		}
		wg.Wait()
	}

}
