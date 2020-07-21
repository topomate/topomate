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
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/internal/ovsdocker"
	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"
)

// resumeCmd represents the resume command
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the current project",
	Long: `Resume the current project by starting the containers and reapplying
the links.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resume called")
		if len(args) > 0 {
			resumeContainers(args[0])
		} else {
			resumeContainers("")
		}

	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// resumeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// resumeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func resumeContainers(name string) {
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

	// Start container(s)

	// If container name is specified, start the container
	if name != "" {
		d := ovsdocker.New(name)
		if err := cli.ContainerStart(ctx, name, types.ContainerStartOptions{}); err != nil {
			panic(err)
		}
		for _, v := range m[name] {
			d.Portname = strings.TrimSuffix(v.HostIface, "_l")
			d.AddPort(v.Bridge, v.ContainerIface, v.Settings, nil, true)
		}
	} else { // Name not specified, start all the containers
		wg := sync.WaitGroup{}
		for cName, lks := range m {
			wg.Add(1)
			go func(w *sync.WaitGroup, c *context.Context, name string, links []ovsdocker.OVSInterface) {
				if err := cli.ContainerStart(*c, name, types.ContainerStartOptions{}); err != nil {
					panic(err)
				}
				d := ovsdocker.New(name)
				for _, v := range links {
					d.Portname = strings.TrimSuffix(v.HostIface, "_l")
					d.AddPort(v.Bridge, v.ContainerIface, v.Settings, nil, true)
				}
				w.Done()
			}(&wg, &ctx, cName, lks)
		}
		wg.Wait()
	}

}
