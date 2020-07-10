package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/internal/link"
	"github.com/rahveiz/topomate/internal/ovsdocker"
	"github.com/rahveiz/topomate/utils"

	"gopkg.in/yaml.v2"
)

// Project is the main struct of topomate
type Project struct {
	Name     string
	AS       map[int]*AutonomousSystem
	Ext      []*ExternalLink
	AllLinks ovsdocker.OVSBulk
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

	config.ConfigDir = filepath.Dir(path)

	// Init global settings
	conf.Global.BGP.ToGlobal()

	nbAS := len(conf.AS)

	// Create a project
	proj := &Project{
		Name: conf.Name,
		AS:   make(map[int]*AutonomousSystem, nbAS),
		Ext:  make([]*ExternalLink, 0, 128),
	}

	// Iterate on AS elements from the config to fill the project
	for _, k := range conf.AS {
		// Basic validation
		if k.NumRouters < 1 {
			utils.Fatalf("AS%d: cannot generate AS without routers\n", k.ASN)
		}

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

	/************************** External links setup **************************/
	if conf.External == nil {
		if conf.ExternalFile == "" {
			utils.Fatalln("External links setup error: please provide either a file or manual specs")
		}
		if filepath.IsAbs(conf.ExternalFile) {
			proj.externalFromFile(conf.ExternalFile)
		} else {
			proj.externalFromFile(config.ConfigDir + "/" + conf.ExternalFile)
		}
	} else {

		for _, k := range conf.External {
			proj.parseExternal(k)
		}
	}
	proj.linkExternal()
	return proj
}

// Print displays some informations concerning the project
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
func (p *Project) StartAll(linksFlag string) {
	var wg sync.WaitGroup
	for asn, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			configPath := fmt.Sprintf(
				"%s/conf_%d_%s",
				utils.GetDirectoryFromKey("ConfigDir", config.DefaultConfigDir),
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

	p.AllLinks = make(ovsdocker.OVSBulk, 1024)
	switch strings.ToLower(linksFlag) {
	case "internal":
		p.ApplyInternalLinks()
		break
	case "external":
		p.ApplyExternalLinks()
		break
	case "none":
		break
	default:
		p.ApplyInternalLinks()
		p.ApplyExternalLinks()
		break
	}
	p.saveLinks()
}

// StopAll stops all containers and removes all links
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
	p.RemoveInternalLinks()
	p.RemoveExternalLinks()
	if err := os.Remove(utils.GetDirectoryFromKey("MainDir", "") + "/links.json"); err != nil {
		utils.Fatalln(err)
	}
}

func setupContainerLinks(brName string, links []Link, m ovsdocker.OVSBulk) {

	// Create an OVS bridge
	link.CreateBridge(brName)

	// Prepare a slice for bulk add to the OVS bridge (better performances)
	// res := make([]ovsdocker.OVSInterface, 0, len(links))

	hostIf := &ovsdocker.OVSInterface{}

	settings := ovsdocker.DefaultParams()
	settings.OFPort = 1
	for _, v := range links {
		idA := v.First.Router.ContainerName
		idB := v.Second.Router.ContainerName
		ifA := v.First.Interface.IfName
		ifB := v.Second.Interface.IfName

		settings.Speed = v.First.Interface.Speed

		link.AddPortToContainer(brName, ifA, idA, settings, hostIf, false)
		// res = append(res, *hostIf)
		if _, ok := m[idA]; !ok {
			m[idA] = make([]ovsdocker.OVSInterface, 0, len(links))
		}
		m[idA] = append(m[idA], *hostIf)
		settings.OFPort++

		settings.Speed = v.Second.Interface.Speed
		link.AddPortToContainer(brName, ifB, idB, settings, hostIf, false)
		// res = append(res, *hostIf)
		if _, ok := m[idB]; !ok {
			m[idB] = make([]ovsdocker.OVSInterface, 0, len(links))
		}
		m[idB] = append(m[idB], *hostIf)
		settings.OFPort++
	}
	// return res
}

func applyFlow(brName string, links []Link) {
	for _, v := range links {
		idA := v.First.Router.ContainerName
		idB := v.Second.Router.ContainerName
		ifA := v.First.Interface.IfName
		ifB := v.Second.Interface.IfName
		link.AddFlow(brName, idA, ifA, idB, ifB)
	}
}

// ApplyInternalLinks creates all internal links for each AS of the project
func (p *Project) ApplyInternalLinks() {

	for n, as := range p.AS {
		// Create bridge with name "int-<ASN>"
		brName := fmt.Sprintf("int-%d", n)
		// Setup container links
		setupContainerLinks(brName, as.Links, p.AllLinks)
	}

	// Link host interfaces to OVS bridges
	ovsdocker.AddToBridgeBulk(p.AllLinks)

	// Apply OpenFlow rules to the bridges
	for n, as := range p.AS {
		brName := fmt.Sprintf("int-%d", n)
		applyFlow(brName, as.Links)
	}
}

// RemoveInternalLinks removes all internal links of the project
func (p *Project) RemoveInternalLinks() {
	for n := range p.AS {
		link.DeleteBridge(fmt.Sprintf("int-%d", n))
	}
}

// ApplyExternalLinks creates all external links between the different AS
func (p *Project) ApplyExternalLinks() {
	for _, v := range p.Ext {

		brName := fmt.Sprintf("ext-%d%s-%d%s",
			v.From.ASN,
			v.From.Router.Hostname,
			v.To.ASN,
			v.To.Router.Hostname,
		)

		link.CreateBridge(brName)
		settings := ovsdocker.DefaultParams()
		hostIf := ovsdocker.OVSInterface{}

		settings.Speed = v.From.Interface.Speed
		link.AddPortToContainer(brName, v.From.Interface.IfName, v.From.Router.ContainerName, settings, &hostIf, true)
		if _, ok := p.AllLinks[v.From.Router.ContainerName]; !ok {
			p.AllLinks[v.From.Router.ContainerName] = make([]ovsdocker.OVSInterface, 0, len(p.Ext))
		}
		p.AllLinks[v.From.Router.ContainerName] = append(p.AllLinks[v.From.Router.ContainerName], hostIf)

		settings.Speed = v.To.Interface.Speed
		link.AddPortToContainer(brName, v.To.Interface.IfName, v.To.Router.ContainerName, settings, &hostIf, true)

		if _, ok := p.AllLinks[v.To.Router.ContainerName]; !ok {
			p.AllLinks[v.To.Router.ContainerName] = make([]ovsdocker.OVSInterface, 0, len(p.Ext))
		}
		p.AllLinks[v.To.Router.ContainerName] = append(p.AllLinks[v.To.Router.ContainerName], hostIf)
	}
}

// RemoveExternalLinks removes all external links
func (p *Project) RemoveExternalLinks() {
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

		rmIn, rmOut := getRouteMaps(lnk.To.Relation, nil, nil)
		// Add an entry in the neighbors table
		lnk.From.Router.Neighbors[toID] = BGPNbr{
			RemoteAS:     lnk.To.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.From.Interface.IfName,
			RouteMapsIn:  rmIn,
			RouteMapsOut: rmOut,
		}

		// Do the same thing for the second part of the link
		lnk.To.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.From.ASN, lnk.From.Router.Hostname)
		lnk.To.Router.Links =
			append(lnk.To.Router.Links, lnk.To.Interface)

		rmIn, rmOut = getRouteMaps(lnk.From.Relation, nil, nil)
		lnk.To.Router.Neighbors[fromID] = BGPNbr{
			RemoteAS:     lnk.From.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.To.Interface.IfName,
			RouteMapsIn:  rmIn,
			RouteMapsOut: rmOut,
		}
	}
}

func getRouteMaps(relation int, inMaps []string, outMaps []string) ([]string, []string) {
	in := make([]string, 0, len(inMaps)+1)
	out := make([]string, 0, len(outMaps)+1)
	switch relation {
	case Provider:
		in = append(in, "PROVIDER_IN")
		out = append(out, "PROVIDER_OUT")
		break
	case Peer:
		in = append(in, "PEER_IN")
		out = append(out, "PEER_OUT")
		break
	case Customer:
		in = append(in, "CUSTOMER_IN")
		out = append(out, "CUSTOMER_OUT")
		break
	default:
		break
	}

	return append(in, inMaps...), append(out, outMaps...)
}

func (p *Project) saveLinks() {
	// Save the interfaces configuration in json for restarts
	j, err := json.Marshal(p.AllLinks)
	if err != nil {
		utils.Fatalln(err)
	}
	f, err := os.Create(utils.GetDirectoryFromKey("MainDir", "") + "/links.json")
	if err != nil {
		utils.Fatalln(err)
	}
	defer f.Close()
	f.Write(j)
}
