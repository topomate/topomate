package link

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/vishvananda/netlink"
)

func CreateBridge(name string) {

	c := ovs.New()

	if err := c.VSwitch.AddBridge(name); err != nil {
		log.Fatalf("failed to add bridge: %v", err)
	}

}

func CreateBridgeWithIp(name, prefix string) {

	// Command must be run as root
	if os.Geteuid() != 0 {
		log.Fatalf("must have root privileges")
	}

	CreateBridge(name)

	br, err := netlink.LinkByName(name)
	if err != nil {
		log.Fatalf("failed to add address to %s: %v", name, err)
	}

	addr, err := netlink.ParseAddr(prefix)
	if err != nil {
		log.Fatalf("failed to parse addr %s: %v", prefix, err)
	}

	if err := netlink.AddrAdd(br, addr); err != nil {
		log.Fatalf("failed to add addr: %v", err)
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
	out, err := exec.Command(
		"ovs-docker",
		"add-port",
		brName,
		ifName,
		containerName,
	).CombinedOutput()
	if err != nil {
		log.Fatalf("error using ovs-docker")
	}
	fmt.Println(string(out))
	fmt.Println("foobar")
}
