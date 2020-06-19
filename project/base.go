package project

import (
	"fmt"
	"io/ioutil"
	"log"
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
	conf := &config.BaseConfig{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	err = yaml.Unmarshal(data, conf)
	utils.Check(err)

	proj := &Project{
		Name: conf.Name,
		AS:   make(map[int]*AutonomousSystem, len(conf.AS)),
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

	return proj
}

func (p *Project) Print() {

}

func (p *Project) StartAll() {
	var wg sync.WaitGroup
	for _, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			go func(r Router, wg *sync.WaitGroup) {
				r.StartContainer(nil)
				wg.Done()

			}(*v.Routers[i], &wg)
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
}

func (e *ExternalLink) SetupExternal(p **net.IPNet) {
	if p == nil {
		return
	}
	prefix := *p
	prefixLen, _ := prefix.Mask.Size()
	addrCnt := cidr.AddressCount(prefix) - 2 // number of hosts available
	assigned := uint64(0)

	e.From.IP = &net.IPNet{
		IP:   prefix.IP,
		Mask: prefix.Mask,
	}
	prefix.IP = cidr.Inc(prefix.IP)
	e.To.IP = &net.IPNet{
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

func (p Project) ApplyExternal() {
	for _, v := range p.Ext {
		brName := fmt.Sprintf("ext-%d%s-%d%s",
			v.From.ASN,
			v.From.Router.Hostname,
			v.To.ASN,
			v.To.Router.Hostname,
		)
		ifa := fmt.Sprintf("toAS%d", v.To.ASN)
		ifb := fmt.Sprintf("toAS%d", v.From.ASN)
		link.CreateBridge(brName)
		link.AddPortToContainer(brName, ifa, v.From.Router.ContainerName)
		link.AddPortToContainer(brName, ifb, v.To.Router.ContainerName)
	}
}
