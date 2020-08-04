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
	IXPs     []IXP
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
			ASN:     k.ASN,
			IGP:     k.IGP,
			MPLS:    k.MPLS,
			Routers: make([]*Router, k.NumRouters),
		}

		if config.VFlag {
			fmt.Printf("Generating %d routers for AS %d.\n", k.NumRouters, k.ASN)
		}

		// Get current AS
		a := proj.AS[k.ASN]

		a.BGP.RedistributeIGP = k.BGP.RedistributeIGP
		a.BGP.Disabled = k.BGP.Disabled

		// Parse network prefix
		if k.Prefix != "" {
			_, n, err := net.ParseCIDR(k.Prefix)
			if err != nil {
				utils.Fatalln(err)
			}
			a.Network = Net{
				IPNet: n,
				NextAvailable: &net.IPNet{
					IP:   cidr.Inc(n.IP),
					Mask: n.Mask,
				},
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
			id := i + 1
			host := "R" + strconv.Itoa(i+1)
			a.Routers[i] = &Router{
				ID:            id,
				Hostname:      host,
				ContainerName: "AS" + strconv.Itoa(k.ASN) + "-" + host,
				NextInterface: 0,
				Neighbors:     make(map[string]*BGPNbr, k.NumRouters+nbAS),
			}

			// Generate loopback address if needed
			if loNet != nil {
				a.Routers[i].Loopback =
					append(a.Routers[i].Loopback, *loNet)
				loNet.IP = cidr.Inc(loNet.IP)
			}

			/***************************** IS-IS  *****************************/
			if a.IGPType() == IGPISIS && k.ISIS.Areas != nil {
				if lvl := k.ISIS.CheckLevel(id); lvl != 0 {
					a.Routers[i].IGP.ISIS.Level = lvl
				}
				a.Routers[i].IGP.ISIS.Area = k.ISIS.CheckArea(id)
			}

		}
		/****************************** OSPF ******************************/
		if a.IGPType() == IGPOSPF && k.OSPF.Networks != nil {
			for _, n := range k.OSPF.Networks {
				// check if network is valid
				if _, _, err := net.ParseCIDR(n.Prefix); err != nil {
					utils.Fatalln(err)
				}

				for _, rID := range n.Routers {
					r := a.getRouter(rID)
					if r.IGP.OSPF == nil {
						r.IGP.OSPF = []OSPFNet{{
							Prefix: n.Prefix,
							Area:   n.Area,
						}}
					} else {
						r.IGP.OSPF =
							append(r.IGP.OSPF, OSPFNet{
								Prefix: n.Prefix,
								Area:   n.Area,
							})
					}
				}
				// a.Routers[i].IGP.OSPF.Networks[n.Prefix] = n.Area
			}
			a.OSPF.Stubs = k.OSPF.Stubs
		}

		// Setup links
		a.SetupLinks(k.Links)

		a.ReserveSubnets(k.Links.SubnetLength)
		if !k.BGP.IBGP.Manual {
			a.linkRouters(true)
		} else {
			a.linkRouters(false)
			a.setupIBGP(k.BGP.IBGP)
		}

		/*********************** Customer routers setup ***********************/
		a.VPN = make([]VPN, len(k.VPN))
		for idx, vpn := range k.VPN {
			a.VPN[idx].VRF = vpn.VRF
			a.VPN[idx].Customers = make([]VPNCustomer, len(vpn.Customers))
			a.VPN[idx].Neighbors = make(map[string]bool, len(vpn.Customers))

			if vpn.HubMode {
				a.VPN[idx].SpokeSubnets = make([]net.IPNet, 0, len(vpn.Customers))
			}

			for i, v := range vpn.Customers {
				_, n, err := net.ParseCIDR(v.Subnet)
				if err != nil {
					utils.Fatalln(err)
				}
				n.IP = cidr.Inc(n.IP)
				router := &Router{
					ID:            i + 1,
					Hostname:      v.Hostname,
					Links:         make([]*NetInterface, 1),
					ContainerName: fmt.Sprintf("AS%d-Cust-%s", k.ASN, v.Hostname),
				}
				if v.Loopback != "" {
					if _, n, err := net.ParseCIDR(v.Loopback); err == nil {
						router.Loopback = append(router.Loopback, *n)
					}
				}
				parentRouter := a.Routers[v.Parent-1]
				a.VPN[idx].Customers[i].Router = router
				a.VPN[idx].Customers[i].Parent = parentRouter
				a.VPN[idx].Customers[i].Hub = v.Hub

				// if we use a hub, parse the remote subnet
				if vpn.HubMode && !v.Hub {
					_, rmt, err := net.ParseCIDR(v.RemoteSubnet)
					if err != nil {
						utils.Fatalln(err)
					}
					a.VPN[idx].SpokeSubnets = append(a.VPN[idx].SpokeSubnets, *rmt)
				}

				a.VPN[idx].Neighbors[parentRouter.LoID()] = true
				l := Link{
					First:  NewLinkItem(parentRouter),
					Second: NewLinkItem(router),
				}
				l.First.Interface.IP = *n
				l.First.Interface.Description =
					fmt.Sprintf("linked to customer %s", v.Hostname)
				l.First.Interface.External = true // not exactly part of the AS
				l.First.Interface.VRF = vpn.VRF
				n.IP = cidr.Inc(n.IP)
				l.Second.Interface.IP = *n

				parentRouter.Links = append(parentRouter.Links, l.First.Interface)
				router.Links[0] = l.Second.Interface
				a.Links = append(a.Links, l)

				// if it is the hub, we also need to add a downstream link
				// if v.Hub {
				// 	_, dn, err := net.ParseCIDR(v.Subnet)
				// 	if err != nil {
				// 		utils.Fatalln(err)
				// 	}

				// 	l := Link{
				// 		First:  NewLinkItem(parentRouter),
				// 		Second: NewLinkItem(router),
				// 	}

				// 	dn.IP = cidr.Inc(n.IP)
				// 	l.First.Interface.IP = *n
				// 	l.First.Interface.Description =
				// 		fmt.Sprintf("linked to customer %s (downstream)", v.Hostname)
				// 	l.First.Interface.External = true
				// 	l.First.Interface.VRF = vpn.VRF + "_down"

				// 	n.IP = cidr.Inc(n.IP)
				// 	l.Second.Interface.IP = *n

				// 	parentRouter.Links = append(parentRouter.Links, l.First.Interface)
				// 	router.Links = append(router.Links, l.Second.Interface)
				// 	a.Links = append(a.Links, l)
				// }
			}
		}
		a.linkVPN()
	}

	/************************** External links setup **************************/
	if conf.External == nil {
		if conf.ExternalFile != "" {
			if filepath.IsAbs(conf.ExternalFile) {
				proj.externalFromFile(conf.ExternalFile)
			} else {
				proj.externalFromFile(config.ConfigDir + "/" + conf.ExternalFile)
			}
		}
	} else {

		for _, k := range conf.External {
			proj.parseExternal(k)
		}
	}
	proj.linkExternal()

	/******************************* IXP setup *******************************/
	proj.IXPs = make([]IXP, len(conf.IXPs))
	for i, ixpCfg := range conf.IXPs {
		proj.IXPs[i] = proj.parseIXPConfig(ixpCfg)
		proj.IXPs[i].linkIXP()
	}
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
			for id, b := range r.Neighbors {
				fmt.Println(id, b)
			}
		}
		for _, vpn := range v.VPN {
			fmt.Println("===", vpn.VRF)
			fmt.Println(vpn.Neighbors)
			for _, r := range vpn.Customers {
				fmt.Println(r.Router.Loopback)
				for _, l := range r.Router.Links {
					fmt.Println(l)
				}
			}
		}
	}

	for _, ixp := range p.IXPs {
		fmt.Println("=> IXP", ixp.ASN)
		fmt.Println(ixp.RouteServer.Loopback[0])
		for _, l := range ixp.Links {
			fmt.Println(*l.Interface)
		}
	}
}

// StartAll starts all containers (creates them before if needed) with the configurations
// present the configuration directory, and apply links
func (p *Project) StartAll(linksFlag string) {
	var wg sync.WaitGroup

	reloadReady := make(chan struct{}) // will be used to trigger a config reload
	wgTotal := len(p.IXPs)
	wg.Add(len(p.IXPs))
	for asn, v := range p.AS {
		totalContainers := v.TotalContainers()
		wg.Add(totalContainers)
		wgTotal += totalContainers
		// Create containers for provider routers
		for i := 0; i < len(v.Routers); i++ {
			configPath := fmt.Sprintf(
				"%s/conf_%d_%s",
				utils.GetDirectoryFromKey("ConfigDir", ""),
				asn,
				v.Routers[i].Hostname,
			)
			go func(r Router, wg *sync.WaitGroup, path string) {
				r.StartContainer(nil, path)
				wg.Done()
				<-reloadReady // wait until links are applied
				r.StartFRR()
				wg.Done()
			}(*v.Routers[i], &wg, configPath)
		}

		// Create containers for customers
		for i := 0; i < len(v.VPN); i++ {
			for j := 0; j < len(v.VPN[i].Customers); j++ {
				configPath := fmt.Sprintf(
					"%s/conf_cust_%s",
					utils.GetDirectoryFromKey("ConfigDir", ""),
					v.VPN[i].Customers[j].Router.Hostname,
				)
				go func(r Router, wg *sync.WaitGroup, path string) {
					r.StartContainer(nil, path)
					wg.Done()
					<-reloadReady // wait until links are applied
					r.StartFRR()
					wg.Done()
				}(*v.VPN[i].Customers[j].Router, &wg, configPath)
			}
		}
	}
	// Create containers for IXPs
	for i := 0; i < len(p.IXPs); i++ {
		configPath := fmt.Sprintf(
			"%s/conf_%d_%s",
			utils.GetDirectoryFromKey("ConfigDir", ""),
			p.IXPs[i].ASN,
			p.IXPs[i].RouteServer.Hostname,
		)
		go func(r Router, wg *sync.WaitGroup, path string) {
			r.StartContainer(nil, path)
			wg.Done()
			<-reloadReady // wait until links are applied
			r.StartFRR()
			wg.Done()
		}(*p.IXPs[i].RouteServer, &wg, configPath)
	}
	wg.Wait()

	if config.VFlag {
		fmt.Println("Applying links with OVS...")
	}

	p.AllLinks = make(ovsdocker.OVSBulk, 1024)
	// currently, internal links must be applied in priority
	switch strings.ToLower(linksFlag) {
	case "internal":
		p.ApplyInternalLinks()
		break
	case "external":
		p.ApplyExternalLinks()
		p.ApplyIXPLinks()
		break
	case "none":
		break
	default:
		p.ApplyInternalLinks()
		p.ApplyExternalLinks()
		p.ApplyIXPLinks()
		break
	}
	wg.Add(wgTotal)
	close(reloadReady) // trigger configuration reload
	p.saveLinks()
	wg.Wait()
}

// StopAll stops all containers and removes all links
func (p *Project) StopAll() {
	var wg sync.WaitGroup
	wg.Add(len(p.IXPs))
	for asn, v := range p.AS {
		wg.Add(v.TotalContainers())
		// Provider
		for i := 0; i < len(v.Routers); i++ {
			configPath := fmt.Sprintf(
				"%s/conf_%d_%s",
				utils.GetDirectoryFromKey("ConfigDir", ""),
				asn,
				v.Routers[i].Hostname,
			)
			go func(r Router, wg *sync.WaitGroup, path string) {
				r.StopContainer(nil, path)
				wg.Done()
			}(*v.Routers[i], &wg, configPath)
		}

		// Customers
		for i := 0; i < len(v.VPN); i++ {
			for j := 0; j < len(v.VPN[i].Customers); j++ {
				configPath := fmt.Sprintf(
					"%s/conf_cust_%s",
					utils.GetDirectoryFromKey("ConfigDir", ""),
					v.VPN[i].Customers[j].Router.Hostname,
				)
				go func(r Router, wg *sync.WaitGroup, path string) {
					r.StopContainer(nil, path)
					wg.Done()
				}(*v.VPN[i].Customers[j].Router, &wg, configPath)
			}
		}
	}
	for i := 0; i < len(p.IXPs); i++ {
		go func(r Router, wg *sync.WaitGroup, path string) {
			r.StopContainer(nil, path)
			wg.Done()
		}(*p.IXPs[i].RouteServer, &wg, "")
	}
	wg.Wait()
	p.RemoveInternalLinks()
	p.RemoveExternalLinks()
	p.RemoteIXPLinks()
	os.Remove(utils.GetDirectoryFromKey("MainDir", "") + "/links.json")
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
		settings.VRF = v.First.Interface.VRF

		link.AddPortToContainer(brName, ifA, idA, settings, hostIf, false)
		// res = append(res, *hostIf)
		if _, ok := m[idA]; !ok {
			m[idA] = make([]ovsdocker.OVSInterface, 0, len(links))
		}
		m[idA] = append(m[idA], *hostIf)
		settings.OFPort++

		settings.Speed = v.Second.Interface.Speed
		settings.VRF = v.Second.Interface.VRF
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
		lnk.From.Router.Neighbors[toID] = &BGPNbr{
			RemoteAS:     lnk.To.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.From.Interface.IfName,
			RouteMapsIn:  rmIn,
			RouteMapsOut: rmOut,
			AF:           AddressFamily{IPv4: true},
		}

		// Do the same thing for the second part of the link
		lnk.To.Interface.Description = fmt.Sprintf("linked to AS%d (%s)", lnk.From.ASN, lnk.From.Router.Hostname)
		lnk.To.Router.Links =
			append(lnk.To.Router.Links, lnk.To.Interface)

		rmIn, rmOut = getRouteMaps(lnk.From.Relation, nil, nil)
		lnk.To.Router.Neighbors[fromID] = &BGPNbr{
			RemoteAS:     lnk.From.ASN,
			UpdateSource: "lo",
			ConnCheck:    false,
			NextHopSelf:  false,
			IfName:       lnk.To.Interface.IfName,
			RouteMapsIn:  rmIn,
			RouteMapsOut: rmOut,
			AF:           AddressFamily{IPv4: true},
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
