package config

import (
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/k0kubun/pp"
	"github.com/rahveiz/topomate/utils"

	"gopkg.in/yaml.v2"
)

type ASConfig struct {
	ASN         int        `yaml:"asn,omitempty"`
	NumRouters  int        `yaml:"routers,omitempty"`
	IGP         string     `yaml:"igp,omitempty"`
	Prefix      string     `yaml:"prefix,omitempty"`
	LinksConfig LinkModule `yaml:"links,omitempty"`
}
type BaseConfig struct {
	Name string     `yaml:"name"`
	AS   []ASConfig `yaml:"autonomous_systems"`
}

type Project struct {
	Name string
	AS   []*AutonomousSystem
}

// ReadConfig reads a yaml file, parses it and returns a Project
func ReadConfig(path string) *Project {
	conf := &BaseConfig{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	err = yaml.Unmarshal(data, conf)
	utils.Check(err)

	proj := &Project{
		Name: conf.Name,
		AS:   make([]*AutonomousSystem, len(conf.AS)),
	}

	// Generate routers
	for idx, k := range conf.AS {
		// Copy informations
		proj.AS[idx] = &AutonomousSystem{
			ASN:     k.ASN,
			IGP:     k.IGP,
			Routers: make([]Router, k.NumRouters),
		}

		a := proj.AS[idx]

		for i := 0; i < k.NumRouters; i++ {
			host := "R" + strconv.Itoa(i+1)
			a.Routers[i] = Router{
				Hostname:      host,
				ContainerName: "AS" + strconv.Itoa(k.ASN) + "-" + host,
			}
		}

		// Setup links
		a.SetupLinks(k.LinksConfig)

		// Parse network prefix
		_, n, err := net.ParseCIDR(k.Prefix)
		if err != nil {
			utils.Fatalln(err)
		}
		a.Network = Net{
			IPNet: n,
		}
	}
	return proj
}

func (p *Project) Print() {
	for _, v := range p.AS {
		// v.SetupLinks()
		pp.Println(*v)
	}
}

func (p *Project) StartAll() {
	var wg sync.WaitGroup
	for _, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			go func(r Router, wg *sync.WaitGroup) {
				r.StartContainer(nil)
				wg.Done()

			}(v.Routers[i], &wg)
		}
	}
	wg.Wait()
	for _, v := range p.AS {
		v.ApplyLinks()
	}
}

func (p *Project) StopAll() {
	var wg sync.WaitGroup
	for _, v := range p.AS {
		wg.Add(len(v.Routers))
		for i := 0; i < len(v.Routers); i++ {
			go func(r Router, wg *sync.WaitGroup) {
				r.StopContainer(nil)
				wg.Done()
			}(v.Routers[i], &wg)
		}
	}
	wg.Wait()
	for _, v := range p.AS {
		v.RemoveLinks()
	}
}

// func ()
