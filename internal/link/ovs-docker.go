package link

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/distribution/uuid"

	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/utils"
)

type IPClient struct {
	PID      int
	Portname string
}

func (n *IPClient) PidStr() string {
	pids := strconv.Itoa(n.PID)
	return pids
}

func (n *IPClient) NetNS(args ...string) *exec.Cmd {
	cmdArgs := []string{"ip", "netns", "exec", n.PidStr()}
	cmdArgs = append(cmdArgs, args...)
	return utils.ExecSudo(cmdArgs...)
}

func (n *IPClient) Link(args ...string) *exec.Cmd {
	cmdArgs := []string{"ip", "link"}
	cmdArgs = append(cmdArgs, args...)
	return utils.ExecSudo(cmdArgs...)
}

func createNetNS(pid int) {
	out, err := utils.ExecSudo("mkdir", "-p", "/var/run/netns").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln("createNetNS:", err)
	}

	varPath := fmt.Sprintf("/var/run/netns/%d", pid)
	procPath := fmt.Sprintf("/proc/%d/ns/net", pid)

	if _, err = os.Stat(varPath); err != nil {
		if os.IsNotExist(err) {
			// os.Symlink(procPath, varPath)
			utils.ExecSudo("ln", "-s", procPath, varPath).CombinedOutput()
		} else {
			utils.Fatalln("createNetNS:", err)
		}
	}

}

func deleteNetNS(pid int) {
	// if err := os.Remove(fmt.Sprintf("/var/run/netns/%d", pid)); err != nil {
	// 	utils.Fatalln("deleteNetNS:", err)
	// }
	varPath := fmt.Sprintf("/var/run/netns/%d", pid)
	utils.ExecSudo("rm", "-f", varPath).CombinedOutput()
}

func findInterface(containerName, ifName string) *exec.Cmd {
	id := fmt.Sprintf("external_ids:container_id=%s", containerName)
	iface := fmt.Sprintf("external_ids:container_iface=%s", ifName)

	return utils.ExecSudo(
		"ovs-vsctl",
		"--data=bare",
		"--no-heading",
		"--columns=name",
		"find",
		"interface",
		id,
		iface,
	)
}

func getPort(containerName, ifName string) string {
	out, err := findInterface(containerName, ifName).CombinedOutput()
	if err != nil {
		utils.Fatalln("getPort:", err)
	}
	return string(out)
}

func getPID(containerName string) int {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	res, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		utils.Fatalln(err)
	}

	return res.State.Pid
}

func AddPort(brName, ifName, containerName string) {
	if getPort(containerName, ifName) != "" {
		utils.Fatalln("addPort: port already exists")
	}

	id := uuid.Generate().String()
	portname := strings.Replace(id, "-", "", -1)[0:13]
	c := IPClient{
		PID:      getPID(containerName),
		Portname: portname,
	}
	portnameC := c.Portname + "_c"
	portnameL := c.Portname + "_l"

	createNetNS(c.PID)
	defer deleteNetNS(c.PID)
	// fmt.Println(portname)
	out, err := c.Link(
		"add", portnameL,
		"type", "veth", "peer", "name", portnameC).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln("ip link", err)
	}

	vsArgs := []string{"ovs-vsctl",
		"--may-exist", "add-port", brName, portnameL, "--",
		"set", "interface", portnameL, "external_ids:container_id=" + containerName,
		"external_ids:container_iface=" + portnameL,
	}

	out, err = utils.ExecSudo(vsArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		c.Link("delete", portnameL).Run()
		utils.Fatalln("add port:", err)
	}

	out, err = c.Link("set", portnameL, "up").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln("ip link set L", err)
	}
	out, err = c.Link("set", portnameC, "netns", c.PidStr()).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln("ip link set C", err)
	}

	fmt.Println(c.NetNS("ip", "link", "set", "dev", portnameC, "name", ifName).String())

	out, err = c.NetNS("ip", "link", "set", "dev", portnameC, "name", ifName).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln("ip netns", err)
	}
	out, err = c.NetNS("ip", "link", "set", ifName, "up").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		utils.Fatalln(err)
	}

}
