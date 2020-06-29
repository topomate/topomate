package ovsdocker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/distribution/uuid"

	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/utils"
)

const MPLSMAXLabels = 65535

type PortSettings struct {
	MPLS  bool
	MTU   int
	Speed int
}

type OVSDockerClient struct {
	PID           int
	Portname      string
	ContainerName string
	varPath       string
	procPath      string
}

func DefaultParams() PortSettings {
	return PortSettings{
		MPLS:  false,
		MTU:   1500,
		Speed: 10000,
	}
}

func (c *OVSDockerClient) pidToStr() string {
	return strconv.Itoa(c.PID)
}

func (c *OVSDockerClient) createNetNSLink() {
	var stderr bytes.Buffer
	cmd := utils.ExecSudo("mkdir", "-p", "/var/run/netns")
	cmd.Stderr = &stderr
	err := cmd.Run()
	fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
	// stderr.Reset()
	if err != nil {
		utils.Fatalln("createNetNS:", err)
	}

	if _, err = os.Stat(c.varPath); err != nil {
		if os.IsNotExist(err) {
			// os.Symlink(procPath, varPath)
			cmd = utils.ExecSudo("ln", "-s", c.procPath, c.varPath)
			cmd.Stderr = &stderr
			_err := cmd.Run()
			fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
			if _err != nil {
				utils.Fatalln("createNetNS:", err)
			}
		} else {
			utils.Fatalln("createNetNS:", err)
		}
	}
}

func (c *OVSDockerClient) deleteNetNSLink() {
	var stderr bytes.Buffer
	cmd := utils.ExecSudo("rm", "-f", c.varPath)
	cmd.Stderr = &stderr
	err := cmd.Run()
	fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
	// stderr.Reset()
	if err != nil {
		utils.Fatalln("deleteNetNS:", err)
	}
}

// PortExists checks if an interface with name ifName already exists in the container (from OVS)
func (c *OVSDockerClient) PortExists(ifName string) bool {
	var stdout bytes.Buffer
	cmd := findInterface(c.ContainerName, ifName)
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		utils.Fatalln("PortExists: ", err)
	}
	return stdout.Len() > 0
}

// New returns an OVSDockerClient based on the container name.
// It fetches the matching PID and generates an UUID for future use
func New(containerName string) *OVSDockerClient {
	c := &OVSDockerClient{
		PID:           getPID(containerName),
		ContainerName: containerName,
	}
	id := uuid.Generate().String()
	portname := strings.Replace(id, "-", "", -1)[0:13]
	c.Portname = portname
	c.procPath = fmt.Sprintf("/proc/%d/ns/net", c.PID)
	c.varPath = fmt.Sprintf("/var/run/netns/%d", c.PID)

	return c
}

// PortnameHost returns the portname suffixed by "_l"
func (c *OVSDockerClient) PortnameHost() string {
	return c.Portname + "_l"
}

// PortnameContainer returns the portname suffixed by "_c"
func (c *OVSDockerClient) PortnameContainer() string {
	return c.Portname + "_c"
}

// IfNames returns both portnames (host and container)
func (c *OVSDockerClient) IfNames() (string, string) {
	return c.PortnameHost(), c.PortnameContainer()
}

func findInterface(containerName, ifName string) *exec.Cmd {
	id := "external_ids:container_id=" + containerName
	iface := "external_ids:container_iface=" + ifName

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

// AddPort adds a port to the container, and links it with an OVS bridge
func (c *OVSDockerClient) AddPort(brName, ifName string, settings PortSettings) error {
	if c.PortExists(ifName) {
		return fmt.Errorf("AddPort: interface %s already exists in container %s", ifName, c.ContainerName)
	}

	c.createNetNSLink()
	defer c.deleteNetNSLink()

	// Create VEth pair
	if err := c.createVEth(); err != nil {
		return err
	}

	// Add the host end of the veth to an OVS bridge
	if err := c.addToBridge(brName, ifName, settings.Speed); err != nil {
		return err
	}

	portHost, portCont := c.IfNames()

	// Activate host side
	if err := c.ExecLink("set", portHost, "up"); err != nil {
		return err
	}

	// Move container side into container
	if err := c.ExecLink("set", portCont, "netns", c.pidToStr()); err != nil {
		return err
	}

	// Change its name
	if err := c.ExecNS("ip", "link", "set", "dev", portCont, "name", ifName); err != nil {
		return err
	}

	// Activate container side
	if err := c.ExecNS("ip", "link", "set", ifName, "up"); err != nil {
		return err
	}

	if settings.MPLS {
		if err := c.ExecNS("sysctl", "-w", "net.mpls.conf."+ifName+".input=1"); err != nil {
			return err
		}

		if err := c.ExecNS("sysctl", "-w", "net.mpls.conf.platform_labels="+strconv.Itoa(MPLSMAXLabels)); err != nil {
			return err
		}
	}

	return nil
}

// ExecNS is a wrapper around the "ip netns exec <PID>" command (with PID auto-filled)
func (c *OVSDockerClient) ExecNS(args ...string) error {
	var stderr bytes.Buffer
	cmdArgs := []string{"ip", "netns", "exec", c.pidToStr()}
	cmdArgs = append(cmdArgs, args...)
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s\n%s", string(stderr.Bytes()), err)
	}
	return nil
}

// ExecLink is a wrapper around the "ip link" command
func (c *OVSDockerClient) ExecLink(args ...string) error {
	var stderr bytes.Buffer
	cmdArgs := []string{"ip", "link"}
	cmdArgs = append(cmdArgs, args...)
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s\n%s", string(stderr.Bytes()), err)
	}
	return nil
}

func (c *OVSDockerClient) createVEth() error {
	host, cont := c.IfNames()
	return c.ExecLink("add", host, "type", "veth", "peer", "name", cont)
}

func (c *OVSDockerClient) addToBridge(brName, ifName string, speed int) error {
	var stderr bytes.Buffer
	host := c.PortnameHost()
	cmdArgs := []string{"ovs-vsctl",
		"--may-exist", "add-port", brName, host, "--",
		"set", "interface", host,
		"external_ids:container_id=" + c.ContainerName,
		"external_ids:container_iface=" + host,
		"ingress_policing_rate=" + strconv.Itoa(speed*1000),
	}
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New(string(stderr.Bytes()))
	}
	return nil
}

func getPID(containerName string) int {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		utils.Fatalln("ovsdocker (getPID):", err)
	}

	res, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		utils.Fatalln("ovsdocker (getPID):", err)
	}

	return res.State.Pid
}
