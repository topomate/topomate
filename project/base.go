package project

import (
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"sync"

	"github.com/apparentlymart/go-cidr/cidr"
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

	// Read a config file
	conf := &config.BaseConfig{}
	if config.VFlag {
		fmt.Println("Reading configuration file:", path)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		utils.Fatalln(err)
	}
	if err := yaml.Unmarshal(data, conf); err != nil {
		utils.Fatalln(err)
	}

	nbAS := len(conf.AS)

	// Create a project
	proj := &Project{
		Name: conf.Name,
		AS:   make(map[int]*AutonomousSystem, nbAS),
	}

	// Iterate on AS elements from the config to fill the project
	for _, k := range conf.AS {
		// Copy informations from the config
		proj.AS[k.ASN] = &AutonomousSystem{
			ASN:             k.ASN,
			IGP:             k.IGP,
			RedistributeIGP: k.RedistributeIGP,
			MPLS:            k.MPLS,
			Routers:         make([]*Router, k.NumRouters),
		}

		if config.VFlag {
			fmt.Printf("Generating %d routers for AS %d.\n", k.NumRouters, k.ASN)
		}

		// Get current AS
		a := proj.AS[k.ASN]

		// Parse network prefix
		if k.Prefix != "" {
			_, n, err := net.ParseCIDR(k.Prefix)
			if err != nil {
				utils.Fatalln(err)
			}
			a.Network = Net{
				IPNet: n,
			}
		}

		var loNet *net.IPNet
		if k.LoRange != "" {
			// Parse loopback network
			_, n, err := net.ParseCIDR(k.LoRange)
			if err != nil {
				utils.Fatalln(err)
			}
			a.LoStart = *n
			loNet = n
		}

		// Generate router elements
		for i := 0; i < k.NumRouters; i++ {
			host := "R" + strconv.Itoa(i+1)
			a.Routers[i] = &Router{
				ID:            i + 1,
				Hostname:      host,
				ContainerName: "AS" + strconv.Itoa(k.ASN) + "-" + host,
				NextInterface: 0,
				Neighbors:     make(map[string]BGPNbr, k.NumRouters+nbAS),
			}

			// Generate loopback address if needed
			if loNet != nil {
				a.Routers[i].Loopback =
					append(a.Routers[i].Loopback, *loNet)
				loNet.IP = cidr.Inc(loNet.IP)
			}
		}

		// Setup links
		a.SetupLinks(k.Links)

		a.ReserveSubnets(k.Links.SubnetLength)
		a.linkRouters()

	}

	// External links setup

	for _, k := range conf.External {
		l := &ExternalLink{
			From: NewExtLinkItem(
				k.From.ASN,
				proj.AS[k.From.ASN].Routers[k.From.RouterID-1],
			),
			To: NewExtLinkItem(
				k.To.ASN,
				proj.AS[k.To.ASN].Routers[k.To.RouterID-1],
			),
		}
		l.setupExternal(&proj.AS[k.From.ASN].Network.NextAvailable)
		proj.Ext = append(proj.Ext, l)
	}
	proj.linkExternal()
	return proj
}

func (p *Project) Print() {
	for n, v := range p.AS {
		fmt.Println("->AS", n)
		for _, r := range v.Routers {
			fmt.Println("-- Router", r.ID)
			for _, l := range r.Links {
				fmt.Println(l)
			}
			for _, b := range r.Neighbors {
				fmt.Println(b)
			}
		}
	}
}

// StartAll starts all containers (creates them before if needed) with the configurations
// present the configuration directory, and apply links
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
		link.AddPortToContainer(brName, v.From.Interface.IfName, v.From.Router.ContainerName, nil)
		link.AddPortToContainer(brName, v.To.Interface.IfName, v.To.Router.ContainerName, nil)
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

	// Iterate on external links
	for _, lnk := range p.Ext {

		// Get IP without mask as identifier for BGP config
		fromID := lnk.From.Interface.IP.IP.String()
		toID := lnk.To.Interface.IP.IP.String()

		// If a loopback is preset, prefer it
		if len(lnk.From.Router.Loopback) > 0 {
			fromID = lnk.From.Router.Loopback[0].IP.String()
		}
		if len(lnk.To.Router.Loopback) > 0 {
			toID = lnk.To.Router.Loopback[0].IP.String()
		}

		// Add description
		lnk.From.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.To.ASN, lnk.To.Router.Hostname)

		// Add a reference to the interface to the router so it can access its properties
		lnk.From.Router.Links =
			append(lnk.From.Router.Links, lnk.From.Interface)

		// Add an entry in the neighbors table
		lnk.From.Router.Neighbors[toID] = BGPNbr{
			RemoteAS:     lnk.To.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.From.Interface.IfName,
		}

		// Do the same thing for the second part of the link
		lnk.To.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.From.ASN, lnk.From.Router.Hostname)
		lnk.To.Router.Links =
			append(lnk.To.Router.Links, lnk.To.Interface)

		lnk.To.Router.Neighbors[fromID] = BGPNbr{
			RemoteAS:     lnk.From.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.To.Interface.IfName,
		}
	}
}
