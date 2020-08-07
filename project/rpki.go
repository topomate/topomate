package project

import (
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

func (p *Project) parseRPKIConfig(rpkiConfig map[string]config.RPKIConfig) {
	p.RPKI = make(map[string]RPKIServer, len(rpkiConfig))
	for hostname, cfg := range rpkiConfig {
		rtr := &Host{
			Hostname:      hostname,
			ContainerName: "AS" + strconv.Itoa(cfg.RouterLink.ASN) + "-" + hostname,
			Command:       strings.Fields("-bind :8083 -verify=false -checktime=false -cache=/rpki.json"),
			DockerImage:   config.DockerRTRImage,
		}

		rtr.Files = []HostFile{{
			HostPath:      utils.ResolveFilePath(cfg.CacheFile),
			ContainerPath: "/rpki.json",
		}}

		currentAS := p.AS[cfg.RouterLink.ASN]
		router := currentAS.getRouter(cfg.RouterLink.RouterID)

		// Create a link between the router and the RTR

		linkRouter := NewLinkItem(router)
		linkRouter.Interface.Description = "linked to " + hostname
		linkRouter.Interface.External = true // no iBGP

		linkRTR := NewHostLinkItem(rtr)

		linkRTR.Interface.IP, linkRouter.Interface.IP = currentAS.Network.NextLinkIPs()

		currentAS.HostLinks = append(currentAS.HostLinks, HostLink{
			Router: linkRouter,
			Host:   linkRTR,
		})
		currentAS.Hosts = append(currentAS.Hosts, rtr)

		router.Links = append(router.Links, linkRouter.Interface)
		// p.addRPKIentry(hostname, linkRTR.Interface.IP.IP)
		p.RPKI[hostname] = RPKIServer{
			IP:   linkRTR.Interface.IP.IP.String(),
			Port: 8083,
		}

	}
}

// func (p *Project) addRPKIentry(name string, ip net.IP) {
// 	for _, v := range p.AS {
// 		if v.RPKI.Servers == nil {
// 			continue
// 		}
// 		for n, e := range v.RPKI.Servers {
// 			if n == name {
// 				e.Enabled = true
// 				e.IP = ip
// 				e.Port = 8083
// 			}
// 		}
// 	}
// }
