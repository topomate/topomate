package frr

import (
	"net"
)

type FRRConfig struct {
	Hostname   string
	Interfaces map[string]IfConfig
	BGP        BGPConfig
}

type IfConfig struct {
	IPs  []net.IPNet
	OSPF []int
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
}
