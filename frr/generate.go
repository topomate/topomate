package frr

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/project"
	"github.com/rahveiz/topomate/utils"
)

func GenerateConfig(p *project.Project) [][]*FRRConfig {
	configs := make([][]*FRRConfig, len(p.AS)+1)
	idx := 0
	for i, as := range p.AS {
		n := as.TotalContainers()
		is4 := as.Network.IPNet.IP.To4() != nil

		configs[idx] = make([]*FRRConfig, n)
		j := 0
		for _, r := range as.Routers {
			c := &FRRConfig{
				Hostname:     r.Hostname,
				Interfaces:   make(map[string]IfConfig, n),
				StaticRoutes: make(staticRoutes, len(r.Links)),
				MPLS:         as.MPLS,
			}

			// Loopback interface
			nbLo := len(r.Loopback)
			if nbLo > 0 {
				ips := make([]net.IPNet, nbLo)
				for idx, ip := range r.Loopback {
					ips[idx] = ip
				}
				c.Interfaces["lo"] = IfConfig{
					IPs:       ips,
					IGPConfig: make([]IGPIfConfig, 0, 5),
				}
			}

			// BGP
			c.BGP = BGPConfig{
				ASN:       i,
				Neighbors: make(map[string]BGPNbr, n),
			}

			if is4 {
				c.BGP.Networks = []string{as.Network.IPNet.String()}
			} else {
				c.BGP.Networks6 = []string{as.Network.IPNet.String()}
			}

			if nbLo > 0 {
				c.BGP.RouterID = r.Loopback[0].IP.String()
			}

			for ip, nbr := range r.Neighbors {
				c.BGP.Neighbors[ip] = BGPNbr(*nbr)
				if nbr.RemoteAS != as.ASN {
					c.StaticRoutes[nbr.IfName] =
						append(c.StaticRoutes[nbr.IfName], ip+"/32")
				}
			}

			// IGP
			igp := strings.ToUpper(as.IGP)
			switch igp {
			case "OSPF":
				// Check if we need to setup OSPFv2 or OSPFv3
				if is4 {
					c.IGP = append(c.IGP, getOSPFConfig(c.BGP.RouterID, 0, RouteRedistribution{
						// Connected: true,
					}))
					c.nextOSPF = 2
				} else {
					c.IGP = append(c.IGP, getOSPF6Config(c.BGP.RouterID))
				}
				if as.RedistributeIGP {
					c.BGP.Redistribute.OSPF = true
				}
				break
			case "IS-IS", "ISIS":
				if as.RedistributeIGP {
					c.BGP.Redistribute.ISIS = true
				}
				// Default level is 2
				lvl := 2
				if r.IGP.ISIS.Level != 0 {
					lvl = r.IGP.ISIS.Level
				}
				c.IGP = append(c.IGP, getISISConfig(r.Loopback[0].IP,
					r.IGP.ISIS.Area, lvl, RouteRedistribution{}))
				break
			default:
				break
			}

			// Interfaces
			for _, iface := range r.Links {
				ifCfg := IfConfig{
					IPs:         []net.IPNet{iface.IP},
					Description: iface.Description,
					Speed:       iface.Speed,
					External:    iface.External,
					IGPConfig:   make([]IGPIfConfig, 0, 5),
				}
				if !iface.External {
					switch igp {
					case "OSPF":
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, OSPFIfConfig{
								V6:        !is4,
								Cost:      iface.Cost,
								ProcessID: 0,
								Area:      0,
							})
					case "ISIS", "IS-IS":
						// Default circuit-type is 2
						circuit := iface.IGP.ISIS.Circuit
						if circuit == 0 {
							circuit = 2
						}
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, ISISIfConfig{
								V6:          !is4,
								ProcessName: isisDefaultProcess,
								Cost:        iface.Cost,
								Passive:     iface.IGP.ISIS.Passive,
								CircuitType: circuit,
							})

						break
					}
				}
				c.Interfaces[iface.IfName] = ifCfg
			}

			// Also add IGP config for loopback interface
			if nbLo > 0 {
				ifCfg := c.Interfaces["lo"]
				switch igp {
				case "OSPF":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, OSPFIfConfig{
							V6:        !is4,
							ProcessID: 0,
							Area:      0,
						})
				case "ISIS", "IS-IS":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, ISISIfConfig{
							V6:          !is4,
							ProcessName: isisDefaultProcess,
							Passive:     true,
						})
					break
				}
				c.Interfaces["lo"] = ifCfg
			}

			configs[idx][j] = c
			configs[idx][j].BGP.VRF = make(map[string]VRFConfig, 5)
			j++
		}

		// VPNS
		for _, vpn := range as.VPN {
			for _, r := range vpn.Customers {
				c := &FRRConfig{
					Hostname:     r.Router.Hostname,
					Interfaces:   make(map[string]IfConfig, n),
					StaticRoutes: make(staticRoutes, len(r.Router.Links)),
				}
				nbLo := len(r.Router.Loopback)
				if nbLo > 0 {
					ips := make([]net.IPNet, nbLo)
					for idx, ip := range r.Router.Loopback {
						ips[idx] = ip
					}
					c.Interfaces["lo"] = IfConfig{
						IPs: ips,
					}
				}

				// IGP
				igp := strings.ToUpper(as.IGP)
				parentCfg := configs[idx][r.Parent.ID-1]

				// if BGPVRF config is not present in parent, add it
				if _, ok := parentCfg.BGP.VRF[vpn.VRF]; !ok {
					parentCfg.BGP.VRF[vpn.VRF] = VRFConfig{
						RD: 1,
						RT: RouteTarget{
							In:  1,
							Out: 1,
						},
						Redistribute: RouteRedistribution{
							//Connected: true,
							OSPF: true,
						},
					}
				}

				// ospfID := parentCfg.nextOSPF
				switch igp {
				case "OSPF":
					// Check if we need to setup OSPFv2 or OSPFv3
					if is4 {
						c.IGP = append(c.IGP, getOSPFConfig(c.BGP.RouterID, 0, RouteRedistribution{
							// Connected: true,
						}))

						// Add IGP on the parent side (parent index in array is
						// its ID - 1, as usual)
						parentIGP := getOSPFConfig(parentCfg.BGP.RouterID, 0,
							RouteRedistribution{
								//Connected: true,
								BGP: true,
							})
						parentIGP.VRF = vpn.VRF
						parentCfg.IGP = append(
							parentCfg.IGP,
							parentIGP,
						)
						// parentCfg.nextOSPF++
					} else {
						c.IGP = append(c.IGP, getOSPF6Config(c.BGP.RouterID))
					}
					if as.RedistributeIGP {
						c.BGP.Redistribute.OSPF = true
					}
					break
				case "IS-IS", "ISIS":
					if as.RedistributeIGP {
						c.BGP.Redistribute.ISIS = true
					}
					c.IGP = append(c.IGP,
						getISISConfig(r.Router.Loopback[0].IP, 1, 2, RouteRedistribution{
							// Connected: true,
						}))
					parentIGP := getISISConfig(parentCfg.Interfaces["lo"].IPs[0].IP, 1, 2,
						RouteRedistribution{
							//Connected: true,
							BGP: true,
						})
					parentIGP.VRF = vpn.VRF
					parentCfg.IGP = append(
						parentCfg.IGP,
						parentIGP,
					)
					break
				default:
					break
				}
				// Interfaces
				for _, iface := range r.Router.Links {
					ifCfg := IfConfig{
						IPs:         []net.IPNet{iface.IP},
						Description: iface.Description,
						Speed:       iface.Speed,
						IGPConfig:   make([]IGPIfConfig, 0, 5),
					}
					switch igp {
					case "OSPF":
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, OSPFIfConfig{
								V6:        !is4,
								Cost:      iface.Cost,
								ProcessID: 0,
								Area:      0,
							})
						if parentIf := as.GetMatchingLink(nil, iface); parentIf != nil {
							pIfCfg := IfConfig{
								IPs:         []net.IPNet{parentIf.IP},
								Description: parentIf.Description,
								Speed:       parentIf.Speed,
								IGPConfig:   make([]IGPIfConfig, 0, 5),
								External:    true,
								VRF:         vpn.VRF,
							}
							pIfCfg.IGPConfig = append(pIfCfg.IGPConfig, OSPFIfConfig{
								V6:        !is4,
								Cost:      parentIf.Cost,
								ProcessID: 0,
								Area:      0,
							})

							parentCfg.Interfaces[parentIf.IfName] = pIfCfg
						}
					case "ISIS", "IS-IS":
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, ISISIfConfig{
								V6:          !is4,
								ProcessName: isisDefaultProcess,
								Cost:        iface.Cost,
								CircuitType: 2,
							})
						break
					}
					c.Interfaces[iface.IfName] = ifCfg
				}

				// Also add IGP config for loopback interface
				if nbLo > 0 {
					ifCfg := c.Interfaces["lo"]
					switch igp {
					case "OSPF":
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, OSPFIfConfig{
								V6:        !is4,
								ProcessID: 0,
								Area:      0,
							})
					case "ISIS", "IS-IS":
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, ISISIfConfig{
								V6:          !is4,
								ProcessName: isisDefaultProcess,
								Passive:     true,
							})
						break
					}
					c.Interfaces["lo"] = ifCfg
				}

				configs[idx][j] = c
				j++
			}
		}
		idx++
	}
	configs[idx] = generateIXPConfigs(p)
	return configs
}

func generateIXPConfigs(p *project.Project) []*FRRConfig {
	configs := make([]*FRRConfig, len(p.IXPs))
	for idx, ixp := range p.IXPs {
		c := &FRRConfig{
			Hostname:     ixp.RouteServer.Hostname,
			IXP:          true,
			Interfaces:   make(map[string]IfConfig, 2), // to IXP brige + lo
			StaticRoutes: make(staticRoutes, len(ixp.RouteServer.Links)),
		}

		// Loopback interface
		nbLo := len(ixp.RouteServer.Loopback)
		if nbLo > 0 {
			ips := make([]net.IPNet, nbLo)
			for idx, ip := range ixp.RouteServer.Loopback {
				ips[idx] = ip
			}
			c.Interfaces["lo"] = IfConfig{
				IPs: ips,
			}
		}

		// BGP
		c.BGP = BGPConfig{
			ASN:       ixp.ASN,
			Neighbors: make(map[string]BGPNbr),
		}

		if nbLo > 0 {
			c.BGP.RouterID = ixp.RouteServer.Loopback[0].IP.String()
		}

		for ip, nbr := range ixp.RouteServer.Neighbors {
			c.BGP.Neighbors[ip] = BGPNbr(*nbr)
			if nbr.RemoteAS != ixp.ASN {
				c.StaticRoutes[nbr.IfName] =
					append(c.StaticRoutes[nbr.IfName], ip+"/32")
			}
		}

		// Interfaces
		for _, iface := range ixp.RouteServer.Links {
			ifCfg := IfConfig{
				IPs:         []net.IPNet{iface.IP},
				Description: iface.Description,
				Speed:       iface.Speed,
				External:    iface.External,
			}
			c.Interfaces[iface.IfName] = ifCfg
			configs[idx] = c
		}
	}
	return configs
}

func sep(w io.Writer) {
	fmt.Fprintln(w, "!")
}

func writeStatic(dst io.Writer, routes staticRoutes) {
	sep(dst)
	for ifName, ips := range routes {
		for _, ip := range ips {
			fmt.Fprintln(dst, "ip route", ip, ifName)
		}
	}
	sep(dst)
}

func writeBGP(dst io.Writer, c BGPConfig) {
	sep(dst)

	// Here we create temp builders for address-family sections so we don't have
	// to iterate multiple times over the map of neighbors
	af4 := strings.Builder{}
	vpn4 := strings.Builder{}

	fmt.Fprintln(dst, "router bgp", c.ASN)
	if c.RouterID != "" {
		fmt.Fprintln(dst, " bgp router-id", c.RouterID)
	}
	for ip, v := range c.Neighbors {
		fmt.Fprintln(dst, " neighbor", ip, "remote-as", v.RemoteAS)
		if v.UpdateSource != "" {
			fmt.Fprintln(dst, " neighbor", ip, "update-source", v.UpdateSource)
		}
		if !v.ConnCheck {
			fmt.Fprintln(dst, " neighbor", ip, "disable-connected-check")
		}

		// address-family ipv4 unicast
		if v.AF.IPv4 {
			fmt.Fprintln(&af4, "  neighbor", ip, "activate")
			if v.NextHopSelf {
				fmt.Fprintln(&af4, "  neighbor", ip, "next-hop-self")
			}
			if v.RouteMapsIn != nil {
				for _, m := range v.RouteMapsIn {
					fmt.Fprintln(&af4, "  neighbor", ip, "route-map", m, "in")
				}
			}
			if v.RouteMapsOut != nil {
				for _, m := range v.RouteMapsOut {
					fmt.Fprintln(&af4, "  neighbor", ip, "route-map", m, "out")
				}
			}
			if v.RRClient {
				fmt.Fprintln(&af4, "  neighbor", ip, "route-reflector-client")
			}
			if v.RSClient {
				fmt.Fprintln(&af4, "  neighbor", ip, "route-server-client")
			}
		}

		// address-family ipv4 vpn
		if v.AF.VPNv4 {
			fmt.Fprintln(&vpn4, "  neighbor", ip, "activate")
			fmt.Fprintln(&vpn4, "  neighbor", ip, "send-community extended")
			if v.RRClient {
				fmt.Fprintln(&af4, "  neighbor", ip, "route-reflector-client")
			}
		}
	}

	fmt.Fprintln(dst, " !")

	// address-family
	fmt.Fprintln(dst, " address-family ipv4 unicast")
	c.Redistribute.Write(dst, 2)
	for _, network := range c.Networks {
		fmt.Fprintln(dst, "  network", network)
	}
	fmt.Fprint(dst, af4.String())
	fmt.Fprintln(dst, " exit-address-family")

	fmt.Fprintln(dst, " !")

	if vpn4.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv4 vpn")
		fmt.Fprint(dst, vpn4.String())
		fmt.Fprintln(dst, " exit-address-family")
	}

	sep(dst)

	for vrf, cfg := range c.VRF {
		fmt.Fprintln(dst, "router bgp", c.ASN, "vrf", vrf)
		fmt.Fprintln(dst, " address-family ipv4 unicast")
		fmt.Fprintf(dst, "  rd vpn export %d:%d\n", c.ASN, cfg.RD)
		fmt.Fprintln(dst, "  label vpn export auto")
		fmt.Fprintf(dst, "  rt vpn import %d:%d\n", c.ASN, cfg.RT.In)
		fmt.Fprintf(dst, "  rt vpn export %d:%d\n", c.ASN, cfg.RT.Out)
		cfg.Redistribute.Write(dst, 2)
		fmt.Fprintln(dst, "  import vpn\n  export vpn")
		fmt.Fprintln(dst, " exit-address-family")
		sep(dst)
	}

	sep(dst)
}

func writeISIS(dst io.Writer, c ISISConfig) {
	sep(dst)

	fmt.Fprintln(dst, "router isis", c.ProcessName)
	fmt.Fprintln(dst, " net", c.ISO)
	fmt.Fprintln(dst, " metric-style wide")
	fmt.Fprintln(dst, " is-type", isisTypeString(c.Type))

	// Here we write the redistribution manually as ISIS syntax is not standard
	c.writeRedistribute(dst, true, false)

	sep(dst)
}

func writeOSPF(dst io.Writer, c OSPFConfig) {
	sep(dst)

	if c.ProcessID > 0 {
		fmt.Fprint(dst, "router ospf ", c.ProcessID)
	} else {
		fmt.Fprint(dst, "router ospf")
	}
	if c.VRF != "" {
		fmt.Fprintln(dst, " vrf", c.VRF)
	} else {
		fmt.Fprint(dst, "\n")
	}
	// if c.RouterID != "" {
	// 	fmt.Fprintln(dst, " ospf router-id", c.RouterID)
	// }

	c.Redistribute.Write(dst, 1)

	sep(dst)
}

func writeOSPF6(dst io.Writer, c OSPF6Config, ifs map[string]IfConfig) {
	sep(dst)

	// multi-instance OSPFv3 is not supported yet on FRRouting
	fmt.Fprintln(dst, "router ospf6")
	for n, i := range ifs {
		for _, e := range i.IGPConfig {
			switch e.(type) {
			case OSPFIfConfig:
				if e.(OSPFIfConfig).V6 {
					fmt.Fprintln(dst, " interface", n, "area 0")
				}
				break
			default:
				break
			}
		}
	}
	if c.RouterID != "" {
		fmt.Fprintln(dst, " ospf6 router-id", c.RouterID)
	}

	c.Redistribute.Write(dst, 1)

	sep(dst)
}

func writeInterface(dst io.Writer, name string, c IfConfig) {
	sep(dst)

	if c.VRF != "" {
		fmt.Fprintln(dst, "interface", name, "vrf", c.VRF)
	} else {
		fmt.Fprintln(dst, "interface", name)
	}
	if c.Description != "" {
		fmt.Fprintln(dst, " description", c.Description)
	}
	for _, ip := range c.IPs {
		fmt.Fprintln(dst, " ip address", ip.String())
	}
	for _, i := range c.IGPConfig {
		i.Write(dst)
	}

	sep(dst)
}

func (c *FRRConfig) writeMPLS(dst io.Writer) {
	sep(dst)

	fmt.Fprintln(dst, "mpls ldp")
	fmt.Fprintln(dst, " router-id", c.BGP.RouterID)

	fmt.Fprintln(dst, " address-family ipv4")
	fmt.Fprintln(dst, "  discovery transport-address", c.BGP.RouterID)
	for ifname, i := range c.Interfaces {
		if !i.External {
			fmt.Fprintln(dst, "  interface", ifname)
		}
	}
	fmt.Fprintln(dst, " exit-address-family")

	sep(dst)
}

func writeRelationsMaps(dst io.Writer, asn int) {

	// Default route maps
	provComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Provider.Community)
	provLP := strconv.Itoa(config.DefaultBGPSettings.Provider.LocalPref)
	peerComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Peer.Community)
	peerLP := strconv.Itoa(config.DefaultBGPSettings.Peer.LocalPref)
	custComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Customer.Community)
	custLP := strconv.Itoa(config.DefaultBGPSettings.Customer.LocalPref)
	fmt.Fprintf(dst,
		`!
bgp community-list standard PROVIDER permit %[1]s
bgp community-list standard PEER permit %[3]s
bgp community-list standard CUSTOMER permit %[5]s
!
route-map PEER_OUT deny 10
 match community PROVIDER
 !
route-map PEER_OUT deny 15
 match community PEER
!
route-map PEER_OUT permit 20
!
route-map PROVIDER_OUT deny 10
 match community PEER
!
route-map PROVIDER_OUT deny 15
 match community PROVIDER
!
route-map PROVIDER_OUT permit 20
!
route-map CUSTOMER_OUT permit 20
!
route-map PEER_IN permit 20
 set community %[3]s
 set local-preference %[4]s
!
route-map CUSTOMER_IN permit 10
 set community %[5]s
 set local-preference %[6]s
!
route-map PROVIDER_IN permit 10
 set community %[1]s
 set local-preference %[2]s
!
`, provComm, provLP, peerComm, peerLP, custComm, custLP)

	sep(dst)
}

func WriteConfig(c FRRConfig) {
	genDir := utils.GetDirectoryFromKey("ConfigDir", config.DefaultConfigDir)
	var filename string
	if c.BGP.ASN == 0 {
		filename = fmt.Sprintf("%s/conf_cust_%s", genDir, c.Hostname)
	} else {
		filename = fmt.Sprintf("%s/conf_%d_%s", genDir, c.BGP.ASN, c.Hostname)
	}
	if config.VFlag {
		fmt.Println("writing", filename)
	}
	file, err := os.Create(filename)
	if err != nil {
		utils.Fatalln(err)
	}
	defer file.Close()

	dst := &strings.Builder{}

	fmt.Fprintf(dst,
		`frr version %s
frr defaults traditional
log file /var/log/frr.log errors
hostname %s
service integrated-vtysh-config
password topomate
`, frrVersion, c.Hostname)
	sep(dst)

	for name, cfg := range c.Interfaces {
		writeInterface(dst, name, cfg)
	}

	writeStatic(dst, c.StaticRoutes)

	if c.BGP.ASN > 0 {
		writeBGP(dst, c.BGP)
		if !c.IXP { // no need for default route-maps in IXP
			writeRelationsMaps(dst, c.BGP.ASN)
		}
	}

	for _, igp := range c.IGP {
		switch igp.(type) {
		case OSPFConfig:
			writeOSPF(dst, igp.(OSPFConfig))
			break
		case OSPF6Config:
			writeOSPF6(dst, igp.(OSPF6Config), c.internalIfs())
			break
		case ISISConfig:
			writeISIS(dst, igp.(ISISConfig))
			break
		default:
			break
		}
	}
	if c.MPLS {
		c.writeMPLS(dst)
	}

	fmt.Fprintln(dst, "line vty")

	file.WriteString(dst.String())

}

func WriteAll(configs [][]*FRRConfig) {
	for _, asCfg := range configs {
		for _, cfg := range asCfg {
			WriteConfig(*cfg)
		}
	}
}

/* OSPF CONFIGURATION */

func getOSPFConfig(routerID string, process int, distrib RouteRedistribution) OSPFConfig {
	cfg := OSPFConfig{
		ProcessID:    process,
		Redistribute: distrib,
		RouterID:     routerID,
	}
	return cfg
}

func getOSPF6Config(routerID string) OSPF6Config {
	cfg := OSPF6Config{
		Redistribute: RouteRedistribution{
			Connected: true,
		},
		RouterID: routerID,
	}
	return cfg
}

/* IS-IS */

func getISISConfig(ip net.IP, area, t int, distrib RouteRedistribution) ISISConfig {
	cfg := ISISConfig{
		ProcessName:  isisDefaultProcess,
		Type:         t,
		Redistribute: distrib,
	}
	ip = ip.To4()
	if ip == nil {
		return cfg
	}
	parts := [4]string{
		fmt.Sprintf("%03d", ip[0]),
		fmt.Sprintf("%03d", ip[1]),
		fmt.Sprintf("%03d", ip[2]),
		fmt.Sprintf("%03d", ip[3]),
	}
	iso := fmt.Sprintf(
		"49.%04d.%s%c.%s%s.%c%s.00",
		area,
		parts[0], parts[1][0],
		parts[1][1:3], parts[2][0:2],
		parts[2][2], parts[3],
	)
	cfg.ISO = iso
	return cfg
}
