package frr

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/project"
)

type BGPNbr project.BGPNbr

type BGPNetworks struct {
	V4 []string
	V6 []string
}

type BGPConfig struct {
	ASN          int
	RouterID     string
	Neighbors    map[string]BGPNbr
	Networks     BGPNetworks
	Redistribute RouteRedistribution
	VRF          map[string]VRFConfig
	Disabled     bool
}

var genericID = net.ParseIP("10.1.1.1")

func getGeneric() string {
	id := genericID
	genericID = cidr.Inc(genericID)
	return id.String()
}

func (c *BGPConfig) setupRouterID(router *project.Router) {
	for _, ip := range router.Loopback {
		if ip.IP.To4() != nil { // is IPv4
			c.RouterID = ip.IP.String()
			break
		}
	}

	if c.RouterID == "" {
		c.RouterID = getGeneric()
	}
}

func (c *BGPConfig) Write(dst io.Writer) {
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
		for _, network := range c.Networks.V4 {
			fmt.Fprintln(dst, "  network", network)
		}
		fmt.Fprint(dst, af4.String())
		fmt.Fprintln(dst, " exit-address-family")
		fmt.Fprintln(dst, " !")
	}

	if af6.Len() > 0 {
		fmt.Fprintln(dst, " address-family ipv6 unicast")
		c.Redistribute.Write(dst, 2)
		for _, network := range c.Networks.V6 {
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
