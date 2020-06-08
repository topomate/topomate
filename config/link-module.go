package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/utils"
)

type Link struct {
	From  string
	To    string
	Speed int
}

type LinkModule struct {
	Kind  string              `yaml:"kind"`
	Specs []map[string]string `yaml:"specs"`
	Links []Link
}

func (a *AutonomousSystem) SetupLinks() {
	switch kind := strings.ToLower(a.Links.Kind); kind {
	case "manual":
		a.Links.SetupManual()
	case "ring":
		a.Links.SetupRing(a.NumRouters)
	case "full-mesh":
		a.Links.SetupFullMesh(a.NumRouters)
	default:
		fmt.Println("Not implemented")
	}
}

func (lm *LinkModule) SetupManual() {
	lm.Links = make([]Link, len(lm.Specs))
	for idx, v := range lm.Specs {
		l := Link{}
		if from, ok := v["from"]; ok {
			l.From = from
		} else {
			utils.PrintError("Manual link setup error: from key missing")
		}
		if to, ok := v["to"]; ok {
			l.To = to
		} else {
			utils.PrintError("Manual link setup error: from key missing")
		}
		l.Speed = 10000
		lm.Links[idx] = l
	}
}

func (lm *LinkModule) SetupRing(nbRouters int) {
	if nbRouters < 3 {
		utils.PrintError("Cannot create ring topology with less than 3 routers.")
		os.Exit(1)
	}
	lm.Links = make([]Link, nbRouters)
	for i := 1; i <= nbRouters; i++ {
		lm.Links[i-1] = Link{
			From:  strconv.Itoa(i),
			To:    strconv.Itoa((i % nbRouters) + 1),
			Speed: 10000,
		}

	}
}

func (lm *LinkModule) SetupFullMesh(nbRouters int) {
	if nbRouters < 2 {
		return
	}
	lm.Links = make([]Link, nbRouters*(nbRouters-1)/2)
	counter := 0
	for i := 1; i <= nbRouters; i++ {
		for j := i + 1; j <= nbRouters; j++ {
			lm.Links[counter] = Link{
				From:  strconv.Itoa(i),
				To:    strconv.Itoa(j),
				Speed: 10000,
			}
			counter++
		}
	}
}
