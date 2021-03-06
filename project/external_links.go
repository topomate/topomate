package project

import (
	"fmt"
	"net"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

const (
	NoRel    = iota
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
			Cost:     10000,
			External: true,
		},
	}
}

func (p *Project) parseExternal(k config.ExternalLink) {
	if _, ok := p.AS[k.From.ASN]; !ok {
		utils.Fatalf("External link error : AS%d does not exist\n", k.From.ASN)
	}
	if _, ok := p.AS[k.To.ASN]; !ok {
		utils.Fatalf("External link error : AS%d does not exist\n", k.To.ASN)
	}
	l := &ExternalLink{
		From: NewExtLinkItem(
			k.From.ASN,
			p.AS[k.From.ASN].getRouter(k.From.RouterID),
		),
		To: NewExtLinkItem(
			k.To.ASN,
			p.AS[k.To.ASN].getRouter(k.To.RouterID),
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
	l.setupExternal(&p.AS[k.From.ASN].Network)
	p.Ext = append(p.Ext, l)
}

func (e *ExternalLink) setupExternal(p *Net) {
	if !p.AutoAddress {
		return
	}
	a, b := p.NextLinkIPs()
	e.From.Interface.IP = a
	e.To.Interface.IP = b
}

func (p *Project) GetMatchingExtLink(first, second *NetInterface) *NetInterface {
	if first != nil && second != nil {
		return nil
	}
	if first != nil {
		for _, v := range p.Ext {
			if v.From.Interface == first {
				return v.To.Interface
			}
		}
	} else {
		for _, v := range p.Ext {
			if v.To.Interface == second {
				return v.From.Interface
			}
		}
	}
	return nil
}

func (p *Project) FindMatchingExtLink(iface *NetInterface) *NetInterface {
	if iface == nil {
		return nil
	}

	for _, lnk := range p.Ext {
		if lnk.From.Interface == iface {
			return lnk.To.Interface
		}
		if lnk.To.Interface == iface {
			return lnk.From.Interface
		}
	}

	return nil
}
