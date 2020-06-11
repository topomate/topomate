package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/link"

	"github.com/rahveiz/topomate/utils"
)

type Link struct {
	First  string
	Second string
	Speed  int
}

type LinkModule struct {
	Kind  string              `yaml:"kind"`
	Specs []map[string]string `yaml:"specs"`
}

func (a *AutonomousSystem) SetupLinks() {
	switch kind := strings.ToLower(a.LinksConfig.Kind); kind {
	case "manual":
		a.Links = a.LinksConfig.SetupManual()
	case "ring":
		a.Links = a.LinksConfig.SetupRing(a.NumRouters)
	case "full-mesh":
		a.Links = a.LinksConfig.SetupFullMesh(a.NumRouters)
	default:
		fmt.Println("Not implemented")
	}
}

func (a *AutonomousSystem) ApplyLinks() {
	for _, v := range a.Links {
		brName := fmt.Sprintf("as%d-br-%s-%s", a.ASN, v.First, v.Second)
		link.CreateBridge(brName)
		link.AddPortToContainer(brName, "eth"+v.Second, a.getContainerName(v.First))
		link.AddPortToContainer(brName, "eth"+v.First, a.getContainerName(v.Second))
	}
}

func (a *AutonomousSystem) RemoveLinks() {
	for _, v := range a.Links {
		brName := fmt.Sprintf("as%d-br-%s-%s", a.ASN, v.First, v.Second)
		link.DeleteBridge(brName)
	}
}

func (lm *LinkModule) SetupManual() []Link {
	links := make([]Link, len(lm.Specs))
	for idx, v := range lm.Specs {
		l := Link{}
		if First, ok := v["First"]; ok {
			l.First = First
		} else {
			utils.PrintError("Manual link setup error: First key missing")
		}
		if Second, ok := v["Second"]; ok {
			l.Second = Second
		} else {
			utils.PrintError("Manual link setup error: First key missing")
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
