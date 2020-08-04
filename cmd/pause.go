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
