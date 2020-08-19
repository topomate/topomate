package project

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

const (
	IGPUndef = iota
	IGPOSPF  = iota
	IGPISIS  = iota
)

// type iBGPYaml struct {
// 	RR []struct {
// 		Router  int   `yaml:"router"`
// 		Clients []int `yaml:"clients,flow"`
// 	} `yaml:"route_reflectors"`
// 	Cliques [][]int `yaml:"cliques,flow"`
// }

type VPNCustomer struct {
	Router *Router
	Parent *Router
	Hub    bool
}

type VPN struct {
	VRF          string
	Customers    []VPNCustomer
	Neighbors    map[string]bool
	SpokeSubnets []net.IPNet
}
type ospfAttributes struct {
	Area int
	Stub bool
}

// AutonomousSystem represents an AS in a Project
type AutonomousSystem struct {
	ASN       int
	IGP       string
	MPLS      bool
	Network   Net
	LoStart   net.IPNet
	Routers   []*Router
	Hosts     []*Host
	Links     []Link
	HostLinks []HostLink
	VPN       []VPN
	BGP       struct {
		Disabled        bool
		RedistributeIGP bool
	}
	OSPF struct {
		Stubs []int
	}
	RPKI struct {
		Servers []string
	}
}

func (vpn *VPN) IsHubAndSpoke() bool {
	return vpn.SpokeSubnets != nil || len(vpn.SpokeSubnets) > 0
}

func (a *AutonomousSystem) IsOSPFStub(area int) bool {
	for _, e := range a.OSPF.Stubs {
		if e == area {
			return true
		}
	}
	return false
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

// TotalContainres returns the total number of router containers needed for the AS
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
		break
	case "ring":
		a.Links = a.SetupRing(cfg)
		break
	case "full-mesh":
		a.Links = a.SetupFullMesh(cfg)
		break
	default:
		break
	}
}

// ReserveSubnets generates IPv4 addressing for internal links in an AS
func (a *AutonomousSystem) ReserveSubnets() {
	if !a.Network.AutoAddress { // do not set subnets
		return
	}
	for _, v := range a.Links {
		a, b := a.Network.NextLinkIPs()
		v.First.Interface.IP = a
		v.Second.Interface.IP = b
	}
}

func (a *AutonomousSystem) linkRouters(ibgp bool) {
	af := AddressFamily{IPv4: true}
	if !a.Network.Is4() {
		af = AddressFamily{IPv6: true}
	}
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

		// Add IGP configuration elements

		if a.IGPType() == IGPISIS {
			sameArea := second.Router.IGP.ISIS.Area == first.Router.IGP.ISIS.Area
			// If the router is only L1 or L2, circuit is the same
			if first.Router.IGP.ISIS.Level != 3 {
				first.Interface.IGP.ISIS.Circuit = first.Router.IGP.ISIS.Level
			} else { // If the router is L1-L2, we decide based on the opposite router
				// Same area: L1-L2
				if sameArea {
					first.Interface.IGP.ISIS.Circuit = second.Interface.IGP.ISIS.Circuit
				} else { // Different areas: L2
					first.Interface.IGP.ISIS.Circuit = 2
				}
			}
			if second.Router.IGP.ISIS.Level != 3 {
				second.Interface.IGP.ISIS.Circuit = second.Router.IGP.ISIS.Level
			} else {
				if sameArea {
					second.Interface.IGP.ISIS.Circuit = first.Interface.IGP.ISIS.Circuit
				} else {
					second.Interface.IGP.ISIS.Circuit = 2
				}
			}
		}

		// Add a reference to the interface to the router so it can access its properties
		first.Router.Links =
			append(first.Router.Links, first.Interface)

		second.Router.Links =
			append(second.Router.Links, second.Interface)

		// Add an entry in the neighbors table if no ibgp configuration is specified
		if ibgp {
			first.Router.Neighbors[secondID] = &BGPNbr{
				RemoteAS:     a.ASN,
				UpdateSource: "lo",
				ConnCheck:    false,
				NextHopSelf:  true,
				AF:           af,
			}
			second.Router.Neighbors[firstID] = &BGPNbr{
				RemoteAS:     a.ASN,
				UpdateSource: "lo",
				ConnCheck:    false,
				NextHopSelf:  true,
				AF:           af,
			}
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

func (a *AutonomousSystem) setupIBGP(ibgpConfig config.IBGPConfig) {
	af := AddressFamily{IPv4: true}
	if !a.Network.Is4() {
		af = AddressFamily{IPv6: true}
	}
	// Setup route reflectors and clients
	for _, r := range ibgpConfig.RR {
		routeReflector := a.getRouter(r.Router)
		for _, c := range r.Clients {
			client := a.getRouter(c)
			id, mask := client.LoInfo()
			routeReflector.Neighbors[id] = &BGPNbr{
				RemoteAS:     a.ASN,
				UpdateSource: "lo",
				RRClient:     true,
				NextHopSelf:  true,
				AF:           af,
				Mask:         mask,
			}

			id, mask = client.LoInfo()
			client.Neighbors[id] = &BGPNbr{
				RemoteAS:     a.ASN,
				UpdateSource: "lo",
				AF:           af,
				Mask:         mask,
			}
		}
	}

	// Setup iBGP cliques
	for _, clique := range ibgpConfig.Cliques {
		// For each router of the clique, add all other routers to its neighbors
		for i := 0; i < len(clique); i++ {
			router := a.getRouter(clique[i])
			for j := 0; j < len(clique); j++ {
				// Skip if i == j (same router)
				if i == j {
					continue
				}
				n := a.getRouter(clique[j])
				id, mask := n.LoInfo()
				router.Neighbors[id] = &BGPNbr{
					RemoteAS:     a.ASN,
					UpdateSource: "lo",
					NextHopSelf:  true,
					AF:           af,
					Mask:         mask,
				}
			}
		}
	}

}

func (a *AutonomousSystem) IGPType() int {
	switch strings.ToUpper(a.IGP) {
	case "OSPF":
		return IGPOSPF
	case "ISIS", "IS-IS":
		return IGPISIS
	default:
		break
	}
	return IGPUndef
}
