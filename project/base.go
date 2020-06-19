package project

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/internal/link"
	"github.com/rahveiz/topomate/utils"

	"gopkg.in/yaml.v2"
)

type Project struct {
	Name string
	AS   map[int]*AutonomousSystem
	Ext  []*ExternalLink
}

// ReadConfig reads a yaml file, parses it and returns a Project
func ReadConfig(path string) *Project {
	conf := &config.BaseConfig{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	err = yaml.Unmarshal(data, conf)
	utils.Check(err)

	nbAS := len(conf.AS)

	proj := &Project{
		Name: conf.Name,
		AS:   make(map[int]*AutonomousSystem, nbAS),
	}

	// Generate routers
	for _, k := range conf.AS {
		// Copy informations
		proj.AS[k.ASN] = &AutonomousSystem{
			ASN:     k.ASN,
			IGP:     k.IGP,
			Routers: make([]*Router, k.NumRouters),
		}

		a := proj.AS[k.ASN]

		for i := 0; i < k.NumRouters; i++ {
			host := "R" + strconv.Itoa(i+1)
			a.Routers[i] = &Router{
				ID:            i + 1,
				Hostname:      host,
				ContainerName: "AS" + strconv.Itoa(k.ASN) + "-" + host,
				NextInterface: 0,
				Neighbors:     make(map[string]BGPNbr, k.NumRouters+nbAS),
			}
		}

		// Setup links
		a.SetupLinks(k.Links)

		// Parse network prefix
		_, n, err := net.ParseCIDR(k.Prefix)
		if err != nil {
			utils.Fatalln(err)
		}
		a.Network = Net{
			IPNet: n,
		}

		a.ReserveSubnets(k.Links.SubnetLength)
		a.linkRouters()

	}

	// External links setup

	for _, k := range conf.External {
		l := &ExternalLink{
			From: ExternalEndpoint{
				ASN:    k.From.ASN,
				Router: proj.AS[k.From.ASN].Routers[k.From.RouterID-1],
			},
			To: ExternalEndpoint{
				ASN:    k.To.ASN,
				Router: proj.AS[k.To.ASN].Routers[k.To.RouterID-1],
			},
		}
		l.SetupExternal(&proj.AS[k.From.ASN].Network.NextAvailable)
		proj.Ext = append(proj.Ext, l)
	}
	proj.linkExternal()
	return proj
}

func (p *Project) Print() {
	// for _, l := range p.Ext {
	// 	fmt.Println(l.From.Interface, l.To.Interface)
	// }
}

func (p *Project) StartAll() {
	var wg sync.WaitGroup
	for asn, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			configPath := fmt.Sprintf(
				"%s/conf_%d_%s",
				utils.GetDirectoryFromKey("config_dir", "~/.topogen"),
				asn,
				v.Routers[i].Hostname,
			)
			go func(r Router, wg *sync.WaitGroup, path string) {
				r.StartContainer(nil, path)
				wg.Done()

			}(*v.Routers[i], &wg, configPath)
		}
	}
	wg.Wait()
	for _, v := range p.AS {
		v.ApplyLinks()
	}
	p.ApplyExternal()
}

func (p *Project) StopAll() {
	var wg sync.WaitGroup
	for _, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			go func(r Router, wg *sync.WaitGroup) {
				r.StopContainer(nil)
				wg.Done()
			}(*v.Routers[i], &wg)
		}
	}
	wg.Wait()
	for _, v := range p.AS {
		v.RemoveLinks()
	}
	p.RemoveExternal()
}

func (p Project) ApplyExternal() {
	for _, v := range p.Ext {
		brName := fmt.Sprintf("ext-%d%s-%d%s",
			v.From.ASN,
			v.From.Router.Hostname,
			v.To.ASN,
			v.To.Router.Hostname,
		)

		link.CreateBridge(brName)
		link.AddPortToContainer(brName, v.From.Interface.IfName, v.From.Router.ContainerName)
		link.AddPortToContainer(brName, v.To.Interface.IfName, v.To.Router.ContainerName)
	}
}

func (p Project) RemoveExternal() {
	for _, v := range p.Ext {
		brName := fmt.Sprintf("ext-%d%s-%d%s",
			v.From.ASN,
			v.From.Router.Hostname,
			v.To.ASN,
			v.To.Router.Hostname,
		)

		link.DeleteBridge(brName)
	}
}

func (p *Project) linkExternal() {
	for _, lnk := range p.Ext {

		lnk.From.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.To.ASN, lnk.To.Router.Hostname)
		lnk.From.Router.Links =
			append(lnk.From.Router.Links, lnk.From.Interface)
		// lnk.From.Router.ExtLinks =
		// 	append(lnk.From.Router.ExtLinks, NetInterfaceExt{ASN: lnk.To.ASN, Interface: lnk.To.Interface})
		lnk.From.Router.Neighbors[lnk.To.Interface.IP.IP.String()] = BGPNbr{
			RemoteAS:     lnk.To.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
		}

		lnk.To.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.From.ASN, lnk.From.Router.Hostname)
		lnk.To.Router.Links =
			append(lnk.To.Router.Links, lnk.To.Interface)
		// lnk.To.Router.ExtLinks =
		// 	append(lnk.To.Router.ExtLinks, NetInterfaceExt{ASN: lnk.From.ASN, Interface: lnk.From.Interface})
		lnk.To.Router.Neighbors[lnk.From.Interface.IP.IP.String()] = BGPNbr{
			RemoteAS:     lnk.From.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
		}
	}
}
