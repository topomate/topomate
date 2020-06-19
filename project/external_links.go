package project

import (
	"fmt"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
)

type NetInterfaceExt struct {
	ASN       int
	Interface *NetInterface
}

type ExternalEndpoint struct {
	ASN       int
	Router    *Router
	Interface *NetInterface
}

type ExternalLink struct {
	From ExternalEndpoint
	To   ExternalEndpoint
}

func NewNetInterfaceExt(asn int, router *Router) NetInterfaceExt {
	ifName := fmt.Sprintf("eth%d", router.NextInterface)
	router.NextInterface++
	iface := &NetInterface{
		RouterID: router.ID,
		IfName:   ifName,
		IP:       net.IPNet{},
		Speed:    10000,
	}
	return NetInterfaceExt{
		ASN:       asn,
		Interface: iface,
	}
}

func (e *ExternalLink) SetupExternal(p **net.IPNet) {
	e.From.Interface = NewNetInterface(e.From.Router)
	e.To.Interface = NewNetInterface(e.To.Router)
	if p == nil {
		return
	}
	prefix := *p
	prefixLen, _ := prefix.Mask.Size()
	addrCnt := cidr.AddressCount(prefix) - 2 // number of hosts available
	assigned := uint64(0)

	e.From.Interface.IP = net.IPNet{
		IP:   prefix.IP,
		Mask: prefix.Mask,
	}
	prefix.IP = cidr.Inc(prefix.IP)
	e.To.Interface.IP = net.IPNet{
		IP:   prefix.IP,
		Mask: prefix.Mask,
	}
	assigned += 2

	// check if we need to get next subnet
	if assigned+2 > addrCnt {
		prefix, _ = cidr.NextSubnet(prefix, prefixLen)
		assigned = 0
	}

	(*p).IP = cidr.Inc(prefix.IP)

}
