package project

import (
	"strconv"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/config"
)

func (p *Project) parseRPKIConfig(cfg config.RPKIConfig) {
	if cfg.Hostname == "" {
		return
	}
	rtr := &Host{
		Hostname:      cfg.Hostname,
		ContainerName: "AS" + strconv.Itoa(cfg.RouterLink.ASN) + "-" + cfg.Hostname,
		DockerImage:   config.DockerRTRImage,
	}
	currentAS := p.AS[cfg.RouterLink.ASN]
	router := currentAS.getRouter(cfg.RouterLink.RouterID)

	// Create a link between the router and the RTR

	linkRouter := NewLinkItem(router)
	linkRouter.Interface.Description = "linked to " + cfg.Hostname
	linkRouter.Interface.External = true // no iBGP

	linkRTR := NewHostLinkItem(rtr)
	n := currentAS.Network.NextIP()
	linkRTR.Interface.IP = n
	n.IP = cidr.Inc(n.IP)
	linkRouter.Interface.IP = n

	currentAS.HostLinks = append(currentAS.HostLinks, HostLink{
		Router: linkRouter,
		Host:   linkRTR,
	})
	currentAS.Hosts = append(currentAS.Hosts, rtr)

	router.Links = append(router.Links, linkRouter.Interface)

}
