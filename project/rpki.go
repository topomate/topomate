package project

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

type roaEntry struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
	Asn       string `json:"asn"`
}

type rpkiSource struct {
	Roas []roaEntry `json:"roas"`
}

func (p *Project) parseRPKIConfig(rpkiConfig map[string]config.RPKIConfig) {
	p.RPKI = make(map[string]RPKIServer, len(rpkiConfig))
	for hostname, cfg := range rpkiConfig {
		rtr := &Host{
			Hostname:      hostname,
			ContainerName: "AS" + strconv.Itoa(cfg.RouterLink.ASN) + "-" + hostname,
			Command:       strings.Fields("-bind :8083 -verify=false -checktime=false -cache=/rpki.json"),
			DockerImage:   config.DockerRTRImage,
		}

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

		var cachePath string
		// generate ROA
		if cfg.ROAs != nil {

			filename := fmt.Sprintf("%s/rpki_%s.json",
				utils.GetDirectoryFromKey("ConfigDir", ""), hostname)
			generateRPKICache(cfg.ROAs, filename)
			cachePath = filename
		} else {
			// no roa entry, search for file
			if cfg.CacheFile == "" {
				utils.Fatalln("RPKI Server error: no roas specified and no cache file provided")
			}
			cachePath = utils.ResolveFilePath(cfg.CacheFile)
		}

		rtr.Files = []HostFile{{
			HostPath:      cachePath,
			ContainerPath: "/rpki.json",
		}}

		currentAS.Hosts = append(currentAS.Hosts, rtr)

		router.Links = append(router.Links, linkRouter.Interface)
		// p.addRPKIentry(hostname, linkRTR.Interface.IP.IP)
		p.RPKI[hostname] = RPKIServer{
			IP:   linkRTR.Interface.IP.IP.String(),
			Port: 8083,
		}
	}
}

func generateRPKICache(src []config.ROA, path string) {
	cache := rpkiSource{
		Roas: make([]roaEntry, 0, len(src)),
	}

	for _, e := range src {
		_, n, err := net.ParseCIDR(e.Prefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ROA entry %v ignored: %v\n", e, err)
			continue
		}
		cur, max := n.Mask.Size()

		if e.MaxLength > max {
			fmt.Fprintf(os.Stderr, "ROA entry %v ignored: maxLength superior to maximum prefix length\n", e)
			continue
		}
		if e.MaxLength < cur {
			fmt.Fprintf(os.Stderr, "ROA entry %v ignored: maxLength inferior to specified prefix\n", e)
			continue
		}

		cache.Roas = append(cache.Roas, roaEntry{
			Prefix:    n.String(),
			MaxLength: e.MaxLength,
			Asn:       "AS" + strconv.Itoa(e.ASN),
		})
	}
	j, err := json.Marshal(cache)
	if err != nil {
		utils.Fatalln(err)
	}
	outfile, err := os.Create(path)
	if err != nil {
		utils.Fatalln(err)
	}
	defer outfile.Close()
	_, err = outfile.Write(j)
	if err != nil {
		utils.Fatalln(err)
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
