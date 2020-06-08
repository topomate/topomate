package config

import (
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/rahveiz/topomate/utils"

	"gopkg.in/yaml.v2"
)

type Router struct {
	Hostname string
}
type AutonomousSystem struct {
	ASN        int          `yaml:"asn"`
	NumRouters int          `yaml:"routers"`
	IGP        string       `yaml:"igp"`
	Routers    []Router     `yaml:"-"`
	Links      []LinkModule `yaml:"links"`
}
type BaseConfig struct {
	As []*AutonomousSystem `yaml:"autonomous_systems"`
}

func ReadConfig(path string) *BaseConfig {
	conf := &BaseConfig{}
	data, err := ioutil.ReadFile(path)
	utils.Check(err)
	err = yaml.Unmarshal(data, conf)
	utils.Check(err)

	// Generate routers
	for _, k := range conf.As {
		k.Routers = make([]Router, k.NumRouters)
		for i := 0; i < k.NumRouters; i++ {
			k.Routers[i] = Router{
				Hostname: "R" + strconv.Itoa(i),
			}
		}
	}

	return conf
}

func (c *BaseConfig) Print() {
	for _, v := range c.As {
		fmt.Println(*v)
	}
}

// func ()
