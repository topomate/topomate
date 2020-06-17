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
			c.BGP = BGPConfig{
				ASN:       as.ASN,
				Neighbors: make(map[string]BGPNbr, n),
			}
			for _, lnk := range r.Links {
				c.BGP.Neighbors[lnk.IP.String()] = BGPNbr{
					RemoteAS:     as.ASN,
					ConnCheck:    false,
					NextHopSelf:  false, //lnk.RouterIndex < 2,
					UpdateSource: "lo",
				}
			}
			configs[i][j] = c
		}
	}
	return configs
}

func sep(w io.Writer) {
	fmt.Fprintln(w, "!")
}

func WriteConfig(c FRRConfig) {
	file, err := os.Create("generated/frr.conf")
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

	/* BGP Configuration */

	fmt.Fprintln(dst, "router bgp", c.BGP.ASN)
	for ip, v := range c.BGP.Neighbors {
		fmt.Fprintln(dst, " neighbor", ip, "remote-as", v.RemoteAS)
		fmt.Fprintln(dst, " neighbor", ip, "update-source", v.UpdateSource)
		if !v.ConnCheck {
			fmt.Fprintln(dst, " neighbor", ip, "disable-connected-check")
		}
	}

	fmt.Fprintln(dst, " !")

	// address-family
	fmt.Fprintln(dst, " address-family ipv4 unicast")
	for ip, v := range c.BGP.Neighbors {
		if v.NextHopSelf {
			fmt.Fprintln(dst, "  neighbor", ip, "next-hop-self")
		}
	}
	fmt.Fprintln(dst, " exit-address-family")

	sep(dst)

	file.WriteString(dst.String())

	/* END BGP Configuration */

}
