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
		configs[idx] = make([]FRRConfig, n)
		for j, r := range as.Routers {
			c := FRRConfig{
				Hostname:   r.Hostname,
				Interfaces: make(map[string]IfConfig, n),
			}

			// Internal interfaces
			for _, iface := range r.Links {
				c.Interfaces[iface.IfName] = IfConfig{
					IPs:         []net.IPNet{iface.IP},
					Description: iface.Description,
				}
			}

			// BGP
			c.BGP = BGPConfig{
				ASN:       i,
				Neighbors: make(map[string]BGPNbr, n),
				Networks:  []string{as.Network.IPNet.String()},
			}
			for ip, nbr := range r.Neighbors { // eBGP
				c.BGP.Neighbors[ip] = BGPNbr(nbr)
			}
			// IGP
			switch strings.ToUpper(as.IGP) {
			case "OSPF":
				c.IGP = append(c.IGP, getOSPFConfig(*as))
			default:
				break
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

func writeBGP(dst io.Writer, c BGPConfig) {
	sep(dst)
	fmt.Fprintln(dst, "router bgp", c.ASN)
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

	c.Redistribute.Write(dst, 1)

	sep(dst)
}

func writeInterface(dst io.Writer, name string, c IfConfig) {
	sep(dst)

	fmt.Fprintln(dst, "interface", name)
	fmt.Fprintln(dst, " description", c.Description)
	for _, ip := range c.IPs {
		fmt.Fprintln(dst, " ip address", ip.String())
	}

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
no ipv6 forwarding
service integrated-vtysh-config
`, c.Hostname)
	sep(dst)

	for name, cfg := range c.Interfaces {
		writeInterface(dst, name, cfg)
	}

	writeBGP(dst, c.BGP)

	for _, igp := range c.IGP {
		switch igp.(type) {
		case OSPFConfig:
			writeOSPF(dst, igp.(OSPFConfig))
			break
		default:
			break
		}
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

func getOSPFConfig(as project.AutonomousSystem) OSPFConfig {
	cfg := OSPFConfig{
		ProcessID: 0,
		Redistribute: RouteRedistribution{
			Connected: true,
		},
	}
	return cfg
}
