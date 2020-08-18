package frr

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/project"
	"github.com/rahveiz/topomate/utils"
)

var nextRouteTarget = 1
var nextRouteDescriptor = 1

func GenerateConfig(p *project.Project) [][]*FRRConfig {
	configs := make([][]*FRRConfig, len(p.AS)+1)
	idx := 0
	for i, as := range p.AS {
		n := as.TotalContainers()
		is4 := as.Network.IPNet.IP.To4() != nil

		configs[idx] = make([]*FRRConfig, 0, n)
		for _, r := range as.Routers {
			c := &FRRConfig{
				Hostname:     r.Hostname,
				Interfaces:   make(map[string]IfConfig, n),
				StaticRoutes: make(staticRoutes, len(r.Links)),
				MPLS:         as.MPLS,
				ipv6:         !is4,
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

			// RPKI
			tmp := strings.Builder{}
			writeRPKI(&tmp, p.RPKI, as.RPKI.Servers)
			c.RPKIBuffer = tmp.String()

			// BGP
			c.BGP = BGPConfig{
				ASN:       i,
				Neighbors: make(map[string]BGPNbr, n),
				Disabled:  as.BGP.Disabled,
				Redistribute: RouteRedistribution{
					ConnectedOwn: true,
				},
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
				if as.BGP.RedistributeIGP {
					c.BGP.Redistribute.OSPF = true
				}
				// Check if we need to setup OSPFv2 or OSPFv3
				if !is4 {
					c.IGP = append(c.IGP, getOSPF6Config(c.BGP.RouterID))
				}

				oCfg := getOSPFConfig(c.BGP.RouterID, 0)

				// No custom config or OSPFv3 (areas not supported)
				if r.IGP.OSPF == nil || !is4 {
					c.IGP = append(c.IGP, oCfg)
					break
				}

				for _, oNet := range r.IGP.OSPF {
					oCfg.Networks = append(oCfg.Networks, oNet)
					if as.IsOSPFStub(oNet.Area) {
						oCfg.Stubs[oNet.Area] = true
					}
				}
				// add loopback in the first area specified
				oCfg.Networks = append(oCfg.Networks, project.OSPFNet{
					Area:   oCfg.Networks[0].Area,
					Prefix: r.Loopback[0].String(),
				})
				c.IGP = append(c.IGP, oCfg)

				break
			case "IS-IS", "ISIS":
				if as.BGP.RedistributeIGP {
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
					ip4, ip6 := ifCfg.GetIPType()

					switch igp {
					case "OSPF":
						if r.IGP.OSPF == nil {
							ifCfg.IGPConfig =
								append(ifCfg.IGPConfig, OSPFIfConfig{
									V4:        ip4,
									V6:        ip6,
									Cost:      iface.Cost,
									ProcessID: 0,
									Area:      0,
								})
						}
					case "ISIS", "IS-IS":
						// Default circuit-type is 2
						circuit := iface.IGP.ISIS.Circuit
						if circuit == 0 {
							circuit = 2
						}

						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, ISISIfConfig{
								V4:          ip4,
								V6:          ip6,
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
				ip4, ip6 := ifCfg.GetIPType()
				switch igp {
				case "OSPF":
					if r.IGP.OSPF == nil {
						ifCfg.IGPConfig =
							append(ifCfg.IGPConfig, OSPFIfConfig{
								V4:        ip4,
								V6:        ip6,
								ProcessID: 0,
								Area:      0,
							})
					}
				case "ISIS", "IS-IS":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, ISISIfConfig{
							V4:          ip4,
							V6:          ip6,
							ProcessName: isisDefaultProcess,
							Passive:     true,
						})
					break
				}
				c.Interfaces["lo"] = ifCfg
			}

			c.BGP.VRF = make(map[string]VRFConfig, 5)
			configs[idx] = append(configs[idx], c)
		}

		// VPNS
		configs[idx] = append(configs[idx], generateVPNConfig(as, configs[idx])...)
		idx++
		// Reset RD / RT values for the next AS
		nextRouteTarget = 1
		nextRouteDescriptor = 1
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
	var af4, vpn4, af6, vpn6 strings.Builder

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

		// address-family ipv6 unicast
		if v.AF.IPv6 {
			fmt.Fprintln(&af6, "  neighbor", ip, "activate")
			if v.NextHopSelf {
				fmt.Fprintln(&af6, "  neighbor", ip, "next-hop-self")
			}
			if v.RouteMapsIn != nil {
				for _, m := range v.RouteMapsIn {
					fmt.Fprintln(&af6, "  neighbor", ip, "route-map", m, "in")
				}
			}
			if v.RouteMapsOut != nil {
				for _, m := range v.RouteMapsOut {
					fmt.Fprintln(&af6, "  neighbor", ip, "route-map", m, "out")
				}
			}
			if v.RRClient {
				fmt.Fprintln(&af6, "  neighbor", ip, "route-reflector-client")
			}
			if v.RSClient {
				fmt.Fprintln(&af6, "  neighbor", ip, "route-server-client")
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
	if af4.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv4 unicast")
		c.Redistribute.Write(dst, 2)
		for _, network := range c.Networks {
			fmt.Fprintln(dst, "  network", network)
		}
		fmt.Fprint(dst, af4.String())
		fmt.Fprintln(dst, " exit-address-family")
		fmt.Fprintln(dst, " !")
	}

	if af6.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv6 unicast")
		c.Redistribute.Write(dst, 2)
		for _, network := range c.Networks6 {
			fmt.Fprintln(dst, "  network", network)
		}
		fmt.Fprint(dst, af6.String())
		fmt.Fprintln(dst, " exit-address-family")
		fmt.Fprintln(dst, " !")
	}

	if vpn4.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv4 vpn")
		fmt.Fprint(dst, vpn4.String())
		fmt.Fprintln(dst, " exit-address-family")
	}

	if vpn6.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv6 vpn")
		fmt.Fprint(dst, vpn6.String())
		fmt.Fprintln(dst, " exit-address-family")
	}

	sep(dst)

	for vrf, cfg := range c.VRF {
		fmt.Fprintln(dst, "router bgp", c.ASN, "vrf", vrf)
		fmt.Fprintln(dst, " address-family ipv4 unicast")
		fmt.Fprintf(dst, "  rd vpn export %d:%d\n", c.ASN, cfg.RD)
		fmt.Fprintln(dst, "  label vpn export auto")
		if cfg.RT.In > 0 {
			fmt.Fprintf(dst, "  rt vpn import %d:%d\n", c.ASN, cfg.RT.In)
			fmt.Fprintln(dst, "  import vpn")
		}
		if cfg.RT.Out > 0 {
			fmt.Fprintf(dst, "  rt vpn export %d:%d\n", c.ASN, cfg.RT.Out)
			fmt.Fprintln(dst, "  export vpn")
		}
		cfg.Redistribute.Write(dst, 2)
		// fmt.Fprintln(dst, "  import vpn\n  export vpn")
		fmt.Fprintln(dst, " exit-address-family")
		sep(dst)
	}

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
	for _, n := range c.Networks {
		fmt.Fprintf(dst, " network %s area %d\n", n.Prefix, n.Area)
	}
	for stub := range c.Stubs {
		fmt.Fprintln(dst, " area", stub, "stub")
	}

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
					fmt.Fprintln(dst, " interface", n, "area 0.0.0.0")
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
		if len(ip.IP) > 0 {
			fmt.Fprintln(dst, " ip address", ip.String())
		}
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

func writePrefixItems(dst io.Writer, prefix string, order int, is6 bool) {
	sep(dst)
	if !is6 {
		fmt.Fprintln(dst, "ip prefix-list OWN_PREFIX permit", prefix, "le 32")
	} else {
		fmt.Fprintln(dst, "ip prefix-list OWN_PREFIX permit", prefix, "le 128")
	}
	fmt.Fprintln(dst, "route-map OWN_PREFIX permit", order)
	fmt.Fprintln(dst, " match ip address prefix-list OWN_PREFIX")
	sep(dst)
}

func WriteConfig(c FRRConfig) {
	genDir := utils.GetDirectoryFromKey("ConfigDir", "")
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

	fmt.Fprintf(dst, c.RPKIBuffer)

	if c.BGP.ASN > 0 && !c.BGP.Disabled {
		writeBGP(dst, c.BGP)
		if !c.IXP { // no need for default route-maps in IXP
			writeRelationsMaps(dst, c.BGP.ASN)
		}
		writeRPKIMaps(dst)
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
			igp.(ISISConfig).writeISIS(dst, !c.ipv6, c.ipv6)
			break
		default:
			break
		}
	}

	n := 1

	for _, p := range c.BGP.Networks {
		writePrefixItems(dst, p, n, false)
		n++
	}

	for _, p := range c.BGP.Networks6 {
		writePrefixItems(dst, p, n, true)
		n++
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

func getOSPFConfig(routerID string, process int) OSPFConfig {
	cfg := OSPFConfig{
		ProcessID: process,
		RouterID:  routerID,
		Networks:  make([]project.OSPFNet, 0, 2),
		Stubs:     make(map[int]bool, 2),
	}
	return cfg
}

func getOSPF6Config(routerID string) OSPF6Config {
	cfg := OSPF6Config{
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
