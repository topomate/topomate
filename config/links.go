package config

import (
	"net"
	"os"
	"strconv"

	"github.com/rahveiz/topomate/utils"
)

type Link struct {
	First    string
	FirstIP  net.IP
	Second   string
	SecondIP net.IP
	Speed    int
}

type LinkModule struct {
	Kind  string              `yaml:"kind"`
	Specs []map[string]string `yaml:"specs"`
}

func (lm *LinkModule) SetupManual() []Link {
	links := make([]Link, len(lm.Specs))
	for idx, v := range lm.Specs {
		l := Link{}
		if First, ok := v["first"]; ok {
			l.First = First
		} else {
			utils.PrintError("Manual link setup error: first key missing")
		}
		if Second, ok := v["second"]; ok {
			l.Second = Second
		} else {
			utils.PrintError("Manual link setup error: second key missing")
		}
		l.Speed = 10000
		links[idx] = l
	}
	return links
}

func (lm *LinkModule) SetupRing(nbRouters int) []Link {
	if nbRouters < 3 {
		utils.PrintError("Cannot create ring topology with less than 3 routers.")
		os.Exit(1)
	}
	links := make([]Link, nbRouters)
	for i := 1; i <= nbRouters; i++ {
		links[i-1] = Link{
			First:  strconv.Itoa(i),
			Second: strconv.Itoa((i % nbRouters) + 1),
			Speed:  10000,
		}

	}
	return links
}

func (lm *LinkModule) SetupFullMesh(nbRouters int) []Link {
	if nbRouters < 2 {
		return nil
	}
	links := make([]Link, nbRouters*(nbRouters-1)/2)
	counter := 0
	for i := 1; i <= nbRouters; i++ {
		for j := i + 1; j <= nbRouters; j++ {
			links[counter] = Link{
				First:  strconv.Itoa(i),
				Second: strconv.Itoa(j),
				Speed:  10000,
			}
			counter++
		}
	}
	return links
}
