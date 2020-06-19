package frr

import (
	"net"

	"github.com/rahveiz/topomate/project"
)

type FRRConfig struct {
	Hostname   string
	Interfaces map[string]IfConfig
	BGP        BGPConfig
	IGP        []interface{}
}

type IfConfig struct {
	Description string
	IPs         []net.IPNet
	OSPF        []int
}

type BGPNbr project.BGPNbr

type BGPConfig struct {
	ASN       int
	Neighbors map[string]BGPNbr
	Networks  []string
}

type OSPFConfig struct {
	ProcessID    int
	Redistribute RouteRedistribution
}

type RouteRedistribution struct {
	Static    bool
	OSPF      bool
	Connected bool
	ISIS      bool
}
