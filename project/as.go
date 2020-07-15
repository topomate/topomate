package project

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

type VPNCustomer struct {
	Router *Router
	Parent *Router
}

type VPN struct {
	VRF       string
	Customers []VPNCustomer
	Neighbors map[string]bool
}

// AutonomousSystem represents an AS in a Project
type AutonomousSystem struct {
	ASN             int
	IGP             string
	RedistributeIGP bool
	MPLS            bool
	Network         Net
	LoStart         net.IPNet
	Routers         []*Router
	Links           []Link
	VPN             []VPN
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

func (a AutonomousSystem) getRouter(n interface{}) *Router {
	var idx int
	var err error
	switch n.(type) {
	case int:
		idx = n.(int)
		break
	case string:
		idx, err = strconv.Atoi(n.(string))
		if err != nil {
			utils.Fatalln(err)
		}
		break
	default:
		utils.Fatalln("getRouter: index type mismtach")
	}
	nbr := len(a.Routers)

	if idx-1 >= nbr {
		utils.Fatalf("getRouter: invalid router number %d (has range from %d to %d)\n", idx-1, 1, nbr)
	}

	return a.Routers[idx-1]
}

// TotalContainres returns the total number of containers needed for the AS
// (= P + PE + CE)
func (a *AutonomousSystem) TotalContainers() int {
	res := len(a.Routers)
	for _, v := range a.VPN {
		res += len(v.Customers)
	}
	return res
}

func (a *AutonomousSystem) GetMatchingLink(first, second *NetInterface) *NetInterface {
	if first != nil && second != nil {
		return nil
	}
	if first != nil {
		for _, v := range a.Links {
			if v.First.Interface == first {
				return v.Second.Interface
			}
		}
	} else {
		for _, v := range a.Links {
			if v.Second.Interface == second {
				return v.First.Interface
			}
		}
	}
	return nil
}

// SetupLinks generates the L2 configuration based on provided config
func (a *AutonomousSystem) SetupLinks(cfg config.InternalLinks) {
	switch kind := strings.ToLower(cfg.Kind); kind {
	case "manual":
		a.Links = a.SetupManual(cfg)
	case "ring":
		a.Links = a.SetupRing(cfg)
	case "full-mesh":
		a.Links = a.SetupFullMesh(cfg)
	default:
		fmt.Println("Not implemented")
	}
}

// ReserveSubnets generates IPv4 addressing for internal links in an AS
func (a *AutonomousSystem) ReserveSubnets(prefixLen int) {
	if prefixLen == 0 { // do not set subnets
		return
	}
	m, _ := a.Network.IPNet.Mask.Size()
	if prefixLen <= m {
		utils.Fatalf("AS%d subnets reservation error: prefixlen too large (%d)\n", a.ASN, prefixLen)
	}

	n, _ := cidr.Subnet(a.Network.IPNet, prefixLen-m, 0)
	addrCnt := cidr.AddressCount(n) - 2 // number of hosts available
	assigned := uint64(0)
	// ip := n.IP

	for _, v := range a.Links {
		// ip = cidr.Inc(ip)
		n.IP = cidr.Inc(n.IP)
		v.First.Interface.IP = *n
		// ip = cidr.Inc(ip)
		n.IP = cidr.Inc(n.IP)
		v.Second.Interface.IP = *n
		assigned += 2

		// check if we need to get next subnet
		if assigned+2 > addrCnt {
			n, _ = cidr.NextSubnet(n, prefixLen)
			assigned = 0
			// ip = n.IP
		}
	}
	a.Network.NextAvailable = &net.IPNet{
		IP:   cidr.Inc(n.IP),
		Mask: n.Mask,
	}
}

func (a *AutonomousSystem) linkRouters() {
	for _, lnk := range a.Links {
		first := lnk.First
		second := lnk.Second

		// Get the interface IP address without mask for BGP configuration
		firstID := first.Interface.IP.IP.String()
		secondID := second.Interface.IP.IP.String()

		// Replace it by the loopback address if present
		if len(first.Router.Loopback) > 0 {
			firstID = first.Router.Loopback[0].IP.String()
		}
		if len(second.Router.Loopback) > 0 {
			secondID = second.Router.Loopback[0].IP.String()
		}

		// Add a reference to the interface to the router so it can access its properties
		first.Router.Links =
			append(first.Router.Links, first.Interface)

		// Add an entry in the neighbors table
		first.Router.Neighbors[secondID] = &BGPNbr{
			RemoteAS:     a.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  true,
			AF:           AddressFamily{IPv4: true},
		}

		// Do the same thing for the second part of the link
		second.Router.Links =
			append(second.Router.Links, second.Interface)
		second.Router.Neighbors[firstID] = &BGPNbr{
			RemoteAS:     a.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  true,
			AF:           AddressFamily{IPv4: true},
		}
	}
}

func (a *AutonomousSystem) linkVPN() {
	for _, lnk := range a.Links {

		first := lnk.First
		second := lnk.Second

		// Replace it by the loopback address if present
		firstID := first.Router.LoID()
		secondID := second.Router.LoID()

		// Add neighbors for VPN
		for _, vpn := range a.VPN {
			if _, ok := vpn.Neighbors[firstID]; ok {
				for id := range vpn.Neighbors {
					if id == firstID {
						continue
					}
					nbr, ok := first.Router.Neighbors[id]
					if ok {
						nbr.AF.VPNv4 = true
					} else {
						first.Router.Neighbors[id] = &BGPNbr{
							RemoteAS:     a.ASN,
							UpdateSource: "lo",
							ConnCheck:    false,
							NextHopSelf:  false,
							AF:           AddressFamily{VPNv4: true},
						}
					}
				}
			}
			// Check if current router is present
			if _, ok := vpn.Neighbors[secondID]; ok {
				for id := range vpn.Neighbors {
					if id == secondID {
						continue
					}
					nbr, ok := second.Router.Neighbors[id]
					if ok {
						nbr.AF.VPNv4 = true
					} else {
						second.Router.Neighbors[id] = &BGPNbr{
							RemoteAS:     a.ASN,
							UpdateSource: "lo",
							ConnCheck:    false,
							NextHopSelf:  false,
							AF:           AddressFamily{VPNv4: true},
						}
					}
				}
			}
		}
	}
}
