package frr

import (
	"net"
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

type BGPNbr struct {
	RemoteAS     int
	UpdateSource string
	ConnCheck    bool
	NextHopSelf  bool
}

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
