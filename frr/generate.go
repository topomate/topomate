package frr

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/rahveiz/topomate/project"
	"github.com/rahveiz/topomate/utils"
)

func GenerateConfig(p *project.Project) [][]FRRConfig {
	configs := make([][]FRRConfig, len(p.AS))
	idx := 0
	for i, as := range p.AS {
		n := len(as.Routers)
		is4 := as.Network.IPNet.IP.To4() != nil

		configs[idx] = make([]FRRConfig, n)
		for j, r := range as.Routers {
			c := FRRConfig{
				Hostname:     r.Hostname,
				Interfaces:   make(map[string]IfConfig, n),
				StaticRoutes: make(staticRoutes, len(r.Links)),
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
			default:
				break
			}

			// Interfaces
			for _, iface := range r.Links {
				ifCfg := IfConfig{
					IPs:         []net.IPNet{iface.IP},
					Description: iface.Description,
					OSPF:        -1,
					Speed:       iface.Speed,
					External:    iface.External,
				}
				if igp == "OSPF" && !iface.External {
					if is4 {
						ifCfg.OSPF = 0
					} else {
						ifCfg.OSPF6 = 0
					}
				}
				c.Interfaces[iface.IfName] = ifCfg
			}

			configs[idx][j] = c
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
	}
	fmt.Fprintln(dst, " exit-address-family")

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
		if i.OSPF6 != -1 {
			fmt.Fprintln(dst, " interface", n, "area 0")
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
	if c.OSPF != -1 {
		fmt.Fprintln(dst, " ip ospf area", c.OSPF)
	}
	if c.Speed > 0 {
		fmt.Fprintln(dst, " bandwidth", c.Speed)
	}

	sep(dst)
}

func (c *FRRConfig) writeMPLS(dst io.Writer) {
	sep(dst)

	fmt.Fprintln(dst, "mpls ldp")
	fmt.Fprintln(dst, " router-id", c.BGP.RouterID)

	fmt.Fprintln(dst, " address-family ipv4")
	fmt.Fprintln(dst, "  discovery transport-address", c.BGP.RouterID)
	for ifname := range c.Interfaces {
		fmt.Fprintln(dst, "  interface", ifname)
	}
	fmt.Fprintln(dst, " exit-address-family")

	sep(dst)
}

func WriteConfig(c FRRConfig) {
	genDir := utils.GetDirectoryFromKey("config_directory", "~/.topogen")
	file, err := os.Create(fmt.Sprintf("%s/conf_%d_%s", genDir, c.BGP.ASN, c.Hostname))
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

	writeBGP(dst, c.BGP)

	for _, igp := range c.IGP {
		switch igp.(type) {
		case OSPFConfig:
			writeOSPF(dst, igp.(OSPFConfig))
			break
		case OSPF6Config:
			writeOSPF6(dst, igp.(OSPF6Config), c.internalIfs())
			break
		default:
			break
		}
	}

	c.writeMPLS(dst)

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
