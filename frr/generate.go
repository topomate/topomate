package frr

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

func GenerateConfig(p *config.Project) [][]FRRConfig {
	configs := make([][]FRRConfig, len(p.AS))

	for i, as := range p.AS {
		n := len(as.Routers)
		configs[i] = make([]FRRConfig, n)
		for j, r := range as.Routers {
			c := FRRConfig{
				Hostname:   r.Hostname,
				Interfaces: make(map[string]IfConfig, n),
			}

			// BGP
			c.BGP = BGPConfig{
				ASN:       as.ASN,
				Neighbors: make(map[string]BGPNbr, n),
				Networks:  []string{as.Network.IPNet.IP.String()},
			}
			for _, lnk := range r.Links {
				c.BGP.Neighbors[lnk.IP.String()] = BGPNbr{
					RemoteAS:     as.ASN,
					ConnCheck:    false,
					NextHopSelf:  false, //lnk.RouterIndex < 2,
					UpdateSource: "lo",
				}
			}

			// IGP
			switch strings.ToUpper(as.IGP) {
			case "OSPF":
				c.IGP = append(c.IGP, getOSPFConfig(*as))
			default:
				break
			}

			configs[i][j] = c
		}
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
log syslog informational
no ipv6 forwarding
service integrated-vtysh-config
`)
	sep(dst)
	fmt.Fprintln(dst, "hostname", c.Hostname)
	sep(dst)
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

func getOSPFConfig(as config.AutonomousSystem) OSPFConfig {
	cfg := OSPFConfig{
		ProcessID: 0,
		Redistribute: RouteRedistribution{
			Connected: true,
		},
	}
	return cfg
}
