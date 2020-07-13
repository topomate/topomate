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

func GenerateConfig(p *project.Project) [][]FRRConfig {
	configs := make([][]FRRConfig, len(p.AS))
	idx := 0
	for i, as := range p.AS {
		n := as.TotalContainers()
		is4 := as.Network.IPNet.IP.To4() != nil

		configs[idx] = make([]FRRConfig, n)
		j := 0
		for _, r := range as.Routers {
			c := FRRConfig{
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
					IPs: ips,
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
				c.BGP.Neighbors[ip] = BGPNbr(nbr)
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
					c.IGP = append(c.IGP, getOSPFConfig(c.BGP.RouterID))
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
				c.IGP = append(c.IGP, getISISConfig(r.Loopback[0].IP, 1, 2))
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
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, ISISIfConfig{
								V6:          !is4,
								ProcessName: isisDefaultProcess,
								Cost:        iface.Cost,
								CircuitType: 2,
							})
						break
					}
				}
				c.Interfaces[iface.IfName] = ifCfg
			}

			configs[idx][j] = c
			j++
		}

		// VPNS
		for _, vpn := range as.VPN {
			for _, r := range vpn.Customers {
				c := FRRConfig{
					Hostname:     r.Hostname,
					Interfaces:   make(map[string]IfConfig, n),
					StaticRoutes: make(staticRoutes, len(r.Links)),
				}

				// IGP
				igp := strings.ToUpper(as.IGP)
				switch igp {
				case "OSPF":
					// Check if we need to setup OSPFv2 or OSPFv3
					if is4 {
						c.IGP = append(c.IGP, getOSPFConfig(c.BGP.RouterID))
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
					c.IGP = append(c.IGP, getISISConfig(r.Loopback[0].IP, 1, 2))
				default:
					break
				}

				// Interfaces
				for _, iface := range r.Links {
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

				configs[idx][j] = c
				j++
			}
		}
		idx++
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
	fmt.Fprintln(dst, "router bgp", c.ASN)
	if c.RouterID != "" {
		fmt.Fprintln(dst, " bgp router-id", c.RouterID)
	}
	for ip, v := range c.Neighbors {
		fmt.Fprintln(dst, " neighbor", ip, "remote-as", v.RemoteAS)
		fmt.Fprintln(dst, " neighbor", ip, "update-source", v.UpdateSource)
		if !v.ConnCheck {
			fmt.Fprintln(dst, " neighbor", ip, "disable-connected-check")
		}
	}

	fmt.Fprintln(dst, " !")

	// address-family
	fmt.Fprintln(dst, " address-family ipv4 unicast")
	c.Redistribute.Write(dst, 2)
	for _, network := range c.Networks {
		fmt.Fprintln(dst, "  network", network)
	}
	for ip, v := range c.Neighbors {
		if v.NextHopSelf {
			fmt.Fprintln(dst, "  neighbor", ip, "next-hop-self")
		}
		if v.RouteMapsIn != nil {
			for _, m := range v.RouteMapsIn {
				fmt.Fprintln(dst, "  neighbor", ip, "route-map", m, "in")
			}
		}
		if v.RouteMapsOut != nil {
			for _, m := range v.RouteMapsOut {
				fmt.Fprintln(dst, "  neighbor", ip, "route-map", m, "out")
			}
		}
	}
	fmt.Fprintln(dst, " exit-address-family")

	sep(dst)
}

func writeISIS(dst io.Writer, c ISISConfig) {
	sep(dst)

	fmt.Fprintln(dst, "router isis", c.ProcessName)
	fmt.Fprintln(dst, " net", c.ISO)
	fmt.Fprintln(dst, " metric-style wide")
	fmt.Fprintln(dst, " is-type", isisTypeString(c.Type))

	sep(dst)
}

func writeOSPF(dst io.Writer, c OSPFConfig) {
	sep(dst)

	if c.ProcessID > 0 {
		fmt.Fprintln(dst, "router ospf", c.ProcessID)
	} else {
		fmt.Fprintln(dst, "router ospf")
	}
	if c.RouterID != "" {
		fmt.Fprintln(dst, " ospf router-id", c.RouterID)
	}

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

	fmt.Fprintln(dst, "interface", name)
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
bgp community-list standard PROVIDER seq 5 permit %[1]s
bgp community-list standard PEER seq 5 permit %[3]s
bgp community-list standard CUSTOMER seq 5 permit %[5]s
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
		`frr version 7.3
frr defaults traditional
hostname %s
log syslog informational
service integrated-vtysh-config
`, c.Hostname)
	sep(dst)

	for name, cfg := range c.Interfaces {
		writeInterface(dst, name, cfg)
	}

	writeStatic(dst, c.StaticRoutes)

	if c.BGP.ASN > 0 {
		writeBGP(dst, c.BGP)
		writeRelationsMaps(dst, c.BGP.ASN)
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

func WriteAll(configs [][]FRRConfig) {
	for _, asCfg := range configs {
		for _, cfg := range asCfg {
			WriteConfig(cfg)
		}
	}
}

/* OSPF CONFIGURATION */

func getOSPFConfig(routerID string) OSPFConfig {
	cfg := OSPFConfig{
		ProcessID: 0,
		Redistribute: RouteRedistribution{
			Connected: true,
		},
		RouterID: routerID,
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

func getISISConfig(ip net.IP, area, t int) ISISConfig {
	cfg := ISISConfig{
		ProcessName: isisDefaultProcess,
		Type:        t,
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
