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
	"io/ioutil"
	"strings"

	"github.com/digitalocean/go-openvswitch/ovs"

	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/internal/ovsdocker"

	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		restartContainer(args[0])
	},
	Args: cobra.MinimumNArgs(1),
}

func init() {
	rootCmd.AddCommand(restartCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// restartCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// restartCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func restartContainer(name string) {
	content, err := ioutil.ReadFile(utils.GetDirectoryFromKey("MainDir", "") + "/links.json")
	if err != nil {
		utils.Fatalln(err)
	}
	m := ovsdocker.OVSBulk{}
	err = json.Unmarshal(content, &m)
	if err != nil {
		utils.Fatalln(err)
	}

	// Stop container
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)
	if err := cli.ContainerRestart(ctx, name, nil); err != nil {
		panic(err)
	}

	c := ovs.New(ovs.Sudo())
	d := ovsdocker.New(name)
	for _, v := range m[name] {
		c.VSwitch.DeletePort(v.Bridge, v.HostIface)
		d.Portname = strings.TrimSuffix(v.HostIface, "_l")
		d.AddPort(v.Bridge, v.ContainerIface, v.Settings, nil, true)
	}

}
