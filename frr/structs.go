package frr

import (
	"io"
	"net"

	"github.com/rahveiz/topomate/project"
)

const (
	fromCustomer       = 10
	fromProvider       = 20
	fromPeer           = 30
	isisDefaultProcess = "1"
	frrVersion         = "7.4.0"
)

type staticRoutes map[string][]string

type FRRConfig struct {
	Hostname     string
	Interfaces   map[string]IfConfig
	BGP          BGPConfig
	IGP          []interface{}
	MPLS         bool
	StaticRoutes map[string][]string
	nextOSPF     int
	IXP          bool
	RPKIBuffer   string
	PrefixLists  []PrefixList
	RouteMaps    []RouteMap
	ipv6         bool
}

type IfConfig struct {
	Description string
	IPs         []net.IPNet
	IGPConfig   []IGPIfConfig
	Speed       int
	External    bool
	VRF         string
}

type BGPNbr project.BGPNbr

type BGPConfig struct {
	ASN          int
	RouterID     string
	Neighbors    map[string]BGPNbr
	Networks     []string
	Networks6    []string
	Redistribute RouteRedistribution
	VRF          map[string]VRFConfig
	Disabled     bool
}

type VRFConfig struct {
	RD           int
	RT           RouteTarget
	Redistribute RouteRedistribution
}

type RouteTarget struct {
	In  int
	Out int
}

type OSPFConfig struct {
	ProcessID    int
	VRF          string
	Redistribute RouteRedistribution
	RouterID     string
	Networks     []project.OSPFNet
	Stubs        map[int]bool
}

type OSPF6Config struct {
	Redistribute RouteRedistribution
	RouterID     string
}

type RouteRedistribution struct {
	Static       bool
	OSPF         bool
	Connected    bool
	ConnectedOwn bool
	ISIS         bool
	BGP          bool
}

type IGPIfConfig interface {
	Write(dst io.Writer)
}

type OSPFIfConfig struct {
	V4        bool
	V6        bool
	ProcessID int
	Area      int
	Cost      int
}

type PrefixList struct {
	Name   string
	Prefix string
	Deny   bool
}

type RouteMapMatch interface {
	WriteMatch(dst io.Writer)
}

type RouteMap struct {
	Name  string
	Match RouteMapMatch
	Deny  bool
}
