package frr

import (
	"net"

	"github.com/rahveiz/topomate/project"
)

type staticRoutes map[string][]string

type FRRConfig struct {
	Hostname     string
	Interfaces   map[string]IfConfig
	BGP          BGPConfig
	IGP          []interface{}
	StaticRoutes map[string][]string
}

type IfConfig struct {
	Description string
	IPs         []net.IPNet
	OSPF        int
	OSPF6       int
	Speed       int
	External    bool
}

type BGPNbr project.BGPNbr

type BGPConfig struct {
	ASN          int
	RouterID     string
	Neighbors    map[string]BGPNbr
	Networks     []string
	Networks6    []string
	Redistribute RouteRedistribution
}

type OSPFConfig struct {
	ProcessID    int
	Redistribute RouteRedistribution
}

type OSPF6Config struct {
	Redistribute RouteRedistribution
}

type RouteRedistribution struct {
	Static    bool
	OSPF      bool
	Connected bool
	ISIS      bool
}
