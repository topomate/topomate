package config

import (
	"fmt"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"

	"github.com/rahveiz/topomate/link"
	"github.com/rahveiz/topomate/utils"
)

// AutonomousSystem represents an AS in a Project
type AutonomousSystem struct {
	ASN     int
	IGP     string
	Network Net
	Routers []*Router
	Links   []Link
}

func (a *AutonomousSystem) getContainerName(n interface{}) string {
	var name string
	switch n.(type) {
	case int:
		name = fmt.Sprintf("AS%d-R%d", a.ASN, n.(int))
		break
	case string:
		name = fmt.Sprintf("AS%d-R%s", a.ASN, n.(string))
		break
	default:
		utils.Fatalln("getContainerName: n type mismatch")
	}
	return name
}

// SetupLinks generates the L2 configuration based on provided config
func (a *AutonomousSystem) SetupLinks(cfg LinkModule) {
	switch kind := strings.ToLower(cfg.Kind); kind {
	case "manual":
		a.Links = cfg.SetupManual()
	case "ring":
		a.Links = cfg.SetupRing(len(a.Routers))
	case "full-mesh":
		a.Links = cfg.SetupFullMesh(len(a.Routers))
	default:
		fmt.Println("Not implemented")
	}
}

// ApplyLinks applies the L2 configuration using OVS
func (a *AutonomousSystem) ApplyLinks() {
	for _, v := range a.Links {
		brName := v.BrName(a.ASN)
		ifa, ifb := v.IfNames()
		link.CreateBridge(brName)
		link.AddPortToContainer(brName, ifa, a.getContainerName(v.First.RouterIndex))
		link.AddPortToContainer(brName, ifb, a.getContainerName(v.Second.RouterIndex))
	}
}

// RemoveLinks removes the L2 configuration of an AS
func (a *AutonomousSystem) RemoveLinks() {
	for _, v := range a.Links {
		link.DeleteBridge(v.BrName(a.ASN))
	}
}

func (a *AutonomousSystem) ReserveSubnets(prefixLen int) {
	if prefixLen == 0 { // do not set subnets
		return
	}
	m, _ := a.Network.IPNet.Mask.Size()
	if prefixLen <= m {
		utils.Fatalf("AS%d subnets reservation error: prefixlen too large", a.ASN)
	}

	n, _ := cidr.Subnet(a.Network.IPNet, prefixLen-m, 0)
	addrCnt := cidr.AddressCount(n) - 2 // number of hosts available
	assigned := uint64(0)
	ip := n.IP

	for _, v := range a.Links {
		ip = cidr.Inc(ip)
		v.First.IP = ip
		ip = cidr.Inc(ip)
		v.Second.IP = ip
		assigned += 2

		// check if we need to get next subnet
		if assigned+2 > addrCnt {
			n, _ = cidr.NextSubnet(n, prefixLen)
			assigned = 0
			ip = n.IP
		}

	}
}

func (a *AutonomousSystem) linkRouters() {
	for _, lnk := range a.Links {
		first := lnk.First
		a.Routers[first.RouterIndex-1].Links =
			append(a.Routers[first.RouterIndex-1].Links, first)
		second := lnk.Second
		a.Routers[second.RouterIndex-1].Links =
			append(a.Routers[second.RouterIndex-1].Links, second)
	}
}

// func (a *AutonomousSystem) SetIPs() {
// 	for _, v := range a.Links {
// 		ifa, _ := v.IfNames()

// 		cmdArgs := []string{
// 			"exec",
// 			"",
// 			"vtysh",
// 			"-c", "conf t",
// 			"-c", "",
// 			"-c", "",
// 		}

// 		// First
// 		cmdArgs[1] = a.Routers[v.First.RouterIndex-1].ContainerName
// 		cmdArgs[6] = fmt.Sprintf("interface %s", ifa)
// 		cmdArgs[8] = fmt.Sprintf("ip address %s/%d", v.First.IP.String(), v.SubnetLength)
// 		// fmt.Println(exec.Command("docker", cmdArgs...).String())
// 		// res, err := exec.Command("docker", "exec", a.Routers[v.First.RouterIndex-1].ContainerName, "vtysh", "-c", "conf t").CombinedOutput()
// 		res, err := exec.Command(
// 			"docker",
// 			cmdArgs...,
// 		).CombinedOutput()

// 		fmt.Println(string(res))
// 		if err != nil {
// 			utils.Fatalln(err)
// 		}

// 		// Second
// 		// cmdArgs[1] = a.Routers[v.Second.RouterIndex-1].ContainerName
// 		// cmdArgs[5] = fmt.Sprintf("'interface %s'", ifb)
// 		// cmdArgs[7] = fmt.Sprintf("'ip address %s/%d'", v.Second.IP.String(), v.SubnetLength)
// 		// _, err = exec.Command(
// 		// 	"docker",
// 		// 	cmdArgs...,
// 		// ).CombinedOutput()
// 		// if err != nil {
// 		// 	utils.Fatalln(err)
// 		// }
// 	}
// }
