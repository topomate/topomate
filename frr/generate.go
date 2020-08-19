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
				StaticRoutes: initStatic(len(r.Links)),
				MPLS:         as.MPLS,
				DefaultIPv6:  !is4,
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
				c.BGP.Networks.V4 = []string{as.Network.IPNet.String()}
			} else {
				c.BGP.Networks.V6 = []string{as.Network.IPNet.String()}
			}

			c.BGP.setupRouterID(r)

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

			// Add static entries for BGP neighbors
			for ip, nbr := range r.Neighbors {
				c.BGP.Neighbors[ip] = BGPNbr(*nbr)
				if nbr.RemoteAS != as.ASN {
					// use IP instead of interface name if found (IPv6 only)
					gw := nbr.IfName
					found := false
					for _, lnk := range r.Links {
						if lnk.IfName == nbr.IfName {
							remoteLink := p.FindMatchingExtLink(lnk)
							if remoteLink != nil && remoteLink.IP.IP.To4() == nil {
								gw = remoteLink.IP.IP.String()
								c.StaticRoutes.add6(ip, nbr.Mask, gw)
								found = true
							}
						}
					}

					if !found {
						c.StaticRoutes.add(ip, nbr.Mask, gw)
					}
				}
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
			StaticRoutes: initStatic(len(ixp.RouteServer.Links)),
		}

		is4 := true

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
			is4 = ixp.RouteServer.Loopback[0].IP.To4 != nil
		}

		// BGP
		c.BGP = BGPConfig{
			ASN:       ixp.ASN,
			Neighbors: make(map[string]BGPNbr),
		}

		c.BGP.setupRouterID(ixp.RouteServer)

		for ip, nbr := range ixp.RouteServer.Neighbors {
			c.BGP.Neighbors[ip] = BGPNbr(*nbr)
			if nbr.RemoteAS != ixp.ASN {
				if is4 {
					c.StaticRoutes.add(ip, nbr.Mask, nbr.IfName)
				} else {
					c.StaticRoutes.add6(ip, nbr.Mask, nbr.IfName)
				}
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

	if !c.DefaultIPv6 {
		fmt.Fprintln(dst, " address-family ipv4")
		if ip, ok := c.firstLoopback(false); ok {
			fmt.Fprintln(dst, "  discovery transport-address", ip)
		}
	} else {
		fmt.Fprintln(dst, " address-family ipv6")
		if ip, ok := c.firstLoopback(true); ok {
			fmt.Fprintln(dst, "  discovery transport-address", ip)
		}
	}

	for ifname, i := range c.Interfaces {
		if !i.External {
			fmt.Fprintln(dst, "  interface", ifname)
		}
	}
	fmt.Fprintln(dst, " exit-address-family")

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

	c.StaticRoutes.Write(dst)

	fmt.Fprintf(dst, c.RPKIBuffer)

	if c.BGP.ASN > 0 && !c.BGP.Disabled {
		c.BGP.Write(dst)
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
			igp.(ISISConfig).writeISIS(dst, !c.DefaultIPv6, c.DefaultIPv6)
			break
		default:
			break
		}
	}

	if c.MPLS {
		c.writeMPLS(dst)
	}

	c.writeUtilities(dst)

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
