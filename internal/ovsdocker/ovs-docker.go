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
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

const MPLSMAXLabels = 65535

type PortSettings struct {
	MTU    int
	Speed  int
	OFPort int
	VRF    string
}

type OVSDockerClient struct {
	PID           int
	Portname      string
	ContainerName string
	varPath       string
	procPath      string
}

type OVSInterface struct {
	HostIface      string `json:"h_if"`
	Bridge         string `json:"br"`
	ContainerIface string `json:"c_if"`
	Settings       PortSettings
}

type OVSBulk map[string][]OVSInterface

func DefaultParams() PortSettings {
	return PortSettings{
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
	// stderr.Reset()
	if err != nil {
		fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
		utils.Fatalln("createNetNS:", err)
	}

	if _, err = os.Stat(c.varPath); err != nil {
		if os.IsNotExist(err) {
			// os.Symlink(procPath, varPath)
			cmd = utils.ExecSudo("ln", "-s", c.procPath, c.varPath)
			cmd.Stderr = &stderr
			_err := cmd.Run()
			if _err != nil {
				fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
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
	// stderr.Reset()
	if err != nil {
		fmt.Fprintln(os.Stderr, string(stderr.Bytes()))
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

// FindPort checks if an interface ifName exists within the container,
// and returns the corresponding interface on the host side
func (c *OVSDockerClient) FindPort(ifName string) (string, bool) {
	var stdout bytes.Buffer
	cmd := findInterface(c.ContainerName, ifName)
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		utils.Fatalln("FindPort: ", err)
	}
	if stdout.Len() > 0 {
		return strings.TrimSuffix(string(stdout.Bytes()), "\n"), true
	}
	return "", false
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

func GetOFPort(containerName, ifName string) (string, bool) {
	var stdout, stderr bytes.Buffer
	cmd := findInterface(containerName, ifName)
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		utils.Fatalln("GetOFPort: ", err)
	}
	if stdout.Len() == 0 {
		return "", false
	}
	ifID := strings.TrimSuffix(string(stdout.Bytes()), "\n")
	stdout.Reset()
	cmd = utils.ExecSudo(
		"ovs-vsctl",
		"get", "Interface",
		ifID, "ofport",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		utils.Fatalln("GetOFPort:", string(stderr.Bytes()), err)
	}
	return strings.TrimSuffix(string(stdout.Bytes()), "\n"), true

}

// AddPort adds a port to the container, and links it with an OVS bridge.
// If hostIf in not nil, it fills the struct fields. If bridge is set to false,
// the host part is not added to the OVS bridge
func (c *OVSDockerClient) AddPort(brName, ifName string, settings PortSettings, hostIf *OVSInterface, bridge bool) error {
	if _, ok := c.FindPort(ifName); ok {
		return fmt.Errorf("AddPort: interface %s already exists in container %s", ifName, c.ContainerName)
	}

	c.createNetNSLink()
	defer c.deleteNetNSLink()

	// Create VEth pair
	if err := c.createVEth(); err != nil {
		return err
	}

	portHost, portCont := c.IfNames()

	if bridge {
		// Add the host end of the veth to an OVS bridge
		if err := c.addToBridge(brName, ifName, settings.Speed, settings.OFPort); err != nil {
			return err
		}
	}
	if hostIf != nil {
		*hostIf = OVSInterface{
			HostIface:      portHost,
			ContainerIface: ifName,
			Bridge:         brName,
			Settings:       settings,
		}
	}

	// Activate host side
	if err := ExecLink("set", portHost, "up"); err != nil {
		return err
	}

	// Move container side into container
	if err := ExecLink("set", portCont, "netns", c.pidToStr()); err != nil {
		return err
	}

	// Change its name
	if err := c.ExecNS("ip", "link", "set", "dev", portCont, "name", ifName); err != nil {
		return err
	}

	// Activate container side and enable MPLS
	if err := c.ExecNS("ip", "link", "set", ifName, "up"); err != nil {
		return err
	}

	if err := c.ExecNS("sysctl", "-w", "net.mpls.conf."+ifName+".input=1"); err != nil {
		return err
	}

	if err := c.ExecNS("sysctl", "-w", "net.mpls.platform_labels="+strconv.Itoa(MPLSMAXLabels)); err != nil {
		return err
	}

	if err := c.ExecNS("sysctl", "-w", "net.ipv4.tcp_l3mdev_accept=1"); err != nil {
		return err
	}
	if err := c.ExecNS("sysctl", "-w", "net.ipv4.udp_l3mdev_accept=1"); err != nil {
		return err
	}

	// Add a VRF in needed
	if settings.VRF != "" {
		c.ExecNS("ip", "link", "add", settings.VRF, "type", "vrf", "table", "100")
		c.ExecNS("ip", "link", "set", settings.VRF, "up")

		if err := c.ExecNS("ip", "link", "set", ifName, "vrf", settings.VRF); err != nil {
			return err
		}
	}

	return nil
}

// DeletePort deletes a port from a container
func (c *OVSDockerClient) DeletePort(ifName string) {
	port, ok := c.FindPort(ifName)
	if !ok {
		return
	}
	err := utils.ExecSudo("ovs-vsctl", "if-exists", "del-port", port).Run()
	if err != nil {
		utils.Fatalln("DeletePort:", err)
	}

	if err := ExecLink("delete", port); err != nil {
		utils.Fatalln(err)
	}
}

// ExecNS is a wrapper around the "ip netns exec <PID>" command (with PID auto-filled)
func (c *OVSDockerClient) ExecNS(args ...string) error {
	var stderr bytes.Buffer
	cmdArgs := []string{"ip", "netns", "exec", c.pidToStr()}
	cmdArgs = append(cmdArgs, args...)
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	if config.VFlag {
		fmt.Println(cmd.String())
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ExecNS: %s\n%s%s", cmd.String(), string(stderr.Bytes()), err)
	}
	return nil
}

// ExecLink is a wrapper around the "ip link" command
func ExecLink(args ...string) error {
	var stderr bytes.Buffer
	cmdArgs := []string{"ip", "link"}
	cmdArgs = append(cmdArgs, args...)
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	if config.VFlag {
		fmt.Println(cmd.String())
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ExecLink: %s\n%s%s", cmd.String(), string(stderr.Bytes()), err)
	}
	return nil
}

func (c *OVSDockerClient) createVEth() error {
	host, cont := c.IfNames()
	return ExecLink("add", host, "type", "veth", "peer", "name", cont)
}

func (c *OVSDockerClient) addToBridge(brName, ifName string, speed int, ofport int) error {
	var stderr bytes.Buffer
	host := c.PortnameHost()
	cmdArgs := []string{"ovs-vsctl",
		"--may-exist", "add-port", brName, host, "--",
		"set", "interface", host,
		"external_ids:container_id=" + c.ContainerName,
		"external_ids:container_iface=" + ifName,
		"ingress_policing_rate=" + strconv.Itoa(speed*1000),
	}
	if ofport > 0 {
		cmdArgs = append(cmdArgs, "ofport_request="+strconv.Itoa(ofport))
	}
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New(string(stderr.Bytes()))
	}
	return nil
}

func AddToBridgeBulk(elements map[string][]OVSInterface) error {
	var stderr bytes.Buffer
	size := 0
	for _, v := range elements {
		size += len(v)
	}

	cmdArgs := make([]string, 1, 16*size)
	cmdArgs[0] = "ovs-vsctl"
	for k, v := range elements {
		for _, e := range v {
			cmdArgs = append(cmdArgs,
				"--", "add-port", e.Bridge, e.HostIface,
				"--", "set", "interface", e.HostIface,
				"external_ids:container_id="+k,
				"external_ids:container_iface="+e.ContainerIface,
				"ingress_policing_rate="+strconv.Itoa(e.Settings.Speed*1000),
				"ofport_request="+strconv.Itoa(e.Settings.OFPort),
			)
		}
	}
	cmd := utils.ExecSudo(cmdArgs...)
	cmd.Stderr = &stderr
	if config.VFlag {
		fmt.Println(cmd.String())
	}
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
