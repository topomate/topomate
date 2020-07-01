package project

import (
	"fmt"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
)

type ExternalLinkItem struct {
	ASN       int
	Router    *Router
	Interface *NetInterface
}

type ExternalLink struct {
	From *ExternalLinkItem
	To   *ExternalLinkItem
}

func NewExtLinkItem(asn int, router *Router) *ExternalLinkItem {
	ifName := fmt.Sprintf("eth%d", router.NextInterface)
	router.NextInterface++
	return &ExternalLinkItem{
		ASN:    asn,
		Router: router,
		Interface: &NetInterface{
			IfName:   ifName,
			IP:       net.IPNet{},
			Speed:    10000,
			External: true,
		},
	}
}

// func NewNetInterfaceExt(router *Router) *NetInterface {
// 	res := NewNetInterface(router)
// 	res.External = true
// 	return res
// }

func (e *ExternalLink) setupExternal(p **net.IPNet) {
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
