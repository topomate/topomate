package project

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/internal/link"
	"github.com/rahveiz/topomate/internal/ovsdocker"
	"github.com/rahveiz/topomate/utils"
)

const separator = "."

type IXP struct {
	ASN         int
	Network     Net
	RouteServer *Router
	Links       []*ExternalLinkItem
}

func (p *Project) parseIXPConfig(cfg config.IXPConfig) IXP {
	name := "IXP-" + strconv.Itoa(cfg.ASN)
	ixp := IXP{
		ASN: cfg.ASN,
		RouteServer: &Router{
			ID:            1,
			Hostname:      name,
			ContainerName: name,
			NextInterface: 0,
			CustomImage:   config.DockerRSImage,
			Neighbors:     make(map[string]*BGPNbr, len(cfg.Peers)),
		},
	}

	// Parse loopback

	_, n, err := net.ParseCIDR(cfg.Loopback)
	if err != nil {
		utils.Fatalln(err)
	}
	ixp.RouteServer.Loopback = append(ixp.RouteServer.Loopback, *n)

	// Parse network CIDR

	_, n, err = net.ParseCIDR(cfg.Prefix)
	if err != nil {
		utils.Fatalln(err)
	}

	ixp.Network.IPNet = n
	ixp.Network.NextAvailable = &net.IPNet{
		IP:   cidr.Inc(n.IP),
		Mask: n.Mask,
	}

	ixp.Links = make([]*ExternalLinkItem, 0, len(cfg.Peers)+1) // peers + rs

	ixp.Links = append(ixp.Links, NewExtLinkItem(ixp.ASN, ixp.RouteServer))
	ixp.Links[0].Interface.IP = ixp.Network.NextIP()

	for _, peer := range cfg.Peers {
		fields := strings.Fields(peer)
		if len(fields) == 0 {
			continue
		}
		_p := strings.SplitN(fields[0], ".", 2)
		peerASN, err := strconv.Atoi(_p[0])
		if err != nil {
			utils.Fatalln(err)
		}
		if len(_p) < 2 {
			utils.Fatalf("IXP link error: peer entry %s malformed (must be <ASN>.<Router_ID>)\n", fields[0])
		}
		peerRouter := _p[1]
		if _, ok := p.AS[peerASN]; !ok {
			utils.Fatalf("IXP link error: ASN%d does not exist\n", peerASN)
		}
		l := NewExtLinkItem(peerASN, p.AS[peerASN].getRouter(peerRouter))

		if len(fields) >= 2 {
			speed, err := strconv.Atoi(fields[1])
			if err != nil {
				utils.Fatalln(err)
			}
			l.Interface.SetSpeedAndCost(speed)
		}

		l.Interface.IP = ixp.Network.NextIP()
		l.Interface.Description = fmt.Sprint("Linked to IXP ", ixp.ASN)
		ixp.Links = append(ixp.Links, l)
	}

	return ixp
}

func (ixp *IXP) linkIXP() {

	// rsID := ixp.RouteServer.LoID()
	ixp.Links[0].Router.Links =
		append(ixp.RouteServer.Links, ixp.Links[0].Interface)
	rmIn, rmOut := getRouteMaps(Peer, nil, nil) // PEER route-maps
	// For each peer, we create an iBGP session between it and the route-server
	for i, lnk := range ixp.Links {
		// Skip first link (RouteServer)
		if i == 0 {
			continue
		}
		// routerID := lnk.Router.LoID()

		// Peer
		lnk.Router.Links = append(lnk.Router.Links, lnk.Interface)
		lnk.Router.Neighbors[ixp.RouteServer.Links[0].IP.IP.String()] = &BGPNbr{
			RemoteAS: ixp.ASN,
			// UpdateSource: "lo",
			NextHopSelf:  true,
			AF:           AddressFamily{IPv4: true},
			IfName:       lnk.Interface.IfName,
			RouteMapsIn:  rmIn,
			RouteMapsOut: rmOut,
		}

		// RS
		ixp.RouteServer.Neighbors[lnk.Interface.IP.IP.String()] = &BGPNbr{
			RemoteAS: lnk.ASN,
			// UpdateSource: "lo",
			IfName:   ixp.Links[0].Interface.IfName,
			AF:       AddressFamily{IPv4: true},
			RSClient: true,
		}
	}
}

func (p *Project) ApplyIXPLinks() {
	for _, ixp := range p.IXPs {
		brName := fmt.Sprintf("ixp-%d", ixp.ASN)
		link.CreateBridge(brName)

		for _, lnk := range ixp.Links {
			settings := ovsdocker.DefaultParams()
			hostIf := ovsdocker.OVSInterface{}

			settings.Speed = lnk.Interface.Speed
			link.AddPortToContainer(brName,
				lnk.Interface.IfName,
				lnk.Router.ContainerName,
				settings, &hostIf, true)
			if _, ok := p.AllLinks[lnk.Router.ContainerName]; !ok {
				p.AllLinks[lnk.Router.ContainerName] = make([]ovsdocker.OVSInterface, 0, len(p.Ext))
			}
			p.AllLinks[lnk.Router.ContainerName] = append(p.AllLinks[lnk.Router.ContainerName], hostIf)
		}

	}
}

func (p *Project) RemoteIXPLinks() {
	for _, ixp := range p.IXPs {
		brName := fmt.Sprintf("ixp-%d", ixp.ASN)
		link.DeleteBridge(brName)
	}
}
