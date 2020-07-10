package project

import (
	"fmt"
	"net"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/config"
)

const (
	Provider = iota
	Customer = iota
	Peer     = iota
)

// ExternalLinkItem represents a side of an ExternalLink
type ExternalLinkItem struct {
	ASN       int
	Router    *Router
	Interface *NetInterface
	Relation  int
}

// ExternalLink represents a link between 2 routers from different AS
type ExternalLink struct {
	From *ExternalLinkItem
	To   *ExternalLinkItem
}

// NewExtLinkItem returns a poiter to an ExternalLinkItem based on the
// provided informations
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

func (p *Project) parseExternal(k config.ExternalLink) {
	l := &ExternalLink{
		From: NewExtLinkItem(
			k.From.ASN,
			p.AS[k.From.ASN].Routers[k.From.RouterID-1],
		),
		To: NewExtLinkItem(
			k.To.ASN,
			p.AS[k.To.ASN].Routers[k.To.RouterID-1],
		),
	}
	switch strings.ToLower(k.Relationship) {
	case "p2c":
		l.From.Relation = Provider
		l.To.Relation = Customer
		break
	case "c2p":
		l.From.Relation = Customer
		l.To.Relation = Provider
		break
	case "p2p":
		l.From.Relation = Peer
		l.To.Relation = Peer
		break
	default:
		break
	}
	l.setupExternal(&p.AS[k.From.ASN].Network.NextAvailable)
	p.Ext = append(p.Ext, l)
}

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
