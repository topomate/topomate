package config

import (
	"fmt"
	"io/ioutil"

	"github.com/rahveiz/topomate/utils"

	"gopkg.in/yaml.v2"
)

type BaseConfig struct {
	AutonomousSystem []struct {
		ASN        int    `yaml:"asn"`
		NumRouters int    `yaml:"routers"`
		IGP        string `yaml:"igp"`
	} `yaml:"autonomous_systems"`
}

func ReadConfig(path string) {
	cfg := BaseConfig{}
	data, err := ioutil.ReadFile(path)
	utils.Check(err)
	err = yaml.Unmarshal(data, &cfg)
	utils.Check(err)
	fmt.Println(cfg)
}
