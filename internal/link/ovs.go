package link

import (
	"fmt"
	"log"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/rahveiz/topomate/internal/ovsdocker"
	"github.com/rahveiz/topomate/utils"
)

func CreateBridge(name string) {

	c := ovs.New(ovs.Sudo())

	if err := c.VSwitch.AddBridge(name); err != nil {
		log.Fatalf("failed to add bridge: %v", err)
	}

}

func DeleteBridge(name string) {
	c := ovs.New(ovs.Sudo())

	if err := c.VSwitch.DeleteBridge(name); err != nil {
		log.Fatalf("failed to delete bridge: %v", err)
	}
}

// AddPortToContainer links a container to an OVS bridge using
// the ovs-docker script
func AddPortToContainer(brName, ifName, containerName string) {
	c := ovsdocker.New(containerName)
	if err := c.AddPort(brName, ifName, ovsdocker.DefaultParams()); err != nil {
		utils.Fatalln(err)
	}
}

// DelPortFromContainer removes an OVS port from a container
func DelPortFromContainer(brName, ifName, containerName string) {
	out, err := utils.ExecSudo(
		"ovs-docker",
		"del-port",
		brName,
		ifName,
		containerName,
	).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		log.Fatalf("error using ovs-docker: %s\n", err)
	}
}

// ClearPortsFromContainer removes all OVS ports from a container
func ClearPortsFromContainer(brName, containerName string) {
	out, err := utils.ExecSudo(
		"ovs-docker",
		"del-ports",
		brName,
		containerName,
	).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		log.Fatalf("error using ovs-docker: %s\n", err)
	}
}
