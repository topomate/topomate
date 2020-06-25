package project

import (
	"fmt"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

// SetupManual generates an internal links configuration based on the provided
// informations
func (a AutonomousSystem) SetupManual(lm config.InternalLinks) []Link {
	links := make([]Link, len(lm.Specs))
	for idx, v := range lm.Specs {
		l := Link{}
		var f, s *Router
		if first, ok := v["first"]; ok {
			f = a.getRouter(first)
			l.First = NewNetInterface(f)
		} else {
			utils.Fatalln("Manual link setup error: first key missing")
		}
		if second, ok := v["second"]; ok {
			s = a.getRouter(second)
			l.Second = NewNetInterface(s)
		} else {
			utils.PrintError("Manual link setup error: second key missing")
		}
		l.First.Description = fmt.Sprintf("linked to %s", s.Hostname)
		l.Second.Description = fmt.Sprintf("linked to %s", f.Hostname)
		links[idx] = l
	}

	// if a preset is present
	switch strings.ToLower(lm.Preset) {
	case "ring":
		links = append(links, a.SetupRing(lm)...)
		break
	case "full-mesh":
		links = append(links, a.SetupFullMesh(lm)...)
		break
	default:
		break
	}
	return links
}

// SetupRing generates an internal links configuration using a ring topology
func (a AutonomousSystem) SetupRing(lm config.InternalLinks) []Link {
	nbRouters := len(a.Routers)
	if nbRouters < 3 {
		utils.Fatalln("Cannot create ring topology with less than 3 routers.")
	}
	links := make([]Link, nbRouters)
	for i := 1; i <= nbRouters; i++ {
		f := a.getRouter(i)
		s := a.getRouter((i % nbRouters) + 1)
		links[i-1] = Link{
			First:  NewNetInterface(f),
			Second: NewNetInterface(s),
		}
		links[i-1].First.Description = fmt.Sprintf("linked to %s", s.Hostname)
		links[i-1].Second.Description = fmt.Sprintf("linked to %s", f.Hostname)
	}
	return links
}

// SetupRing generates an internal links configuration using a full-mesh topology
func (a AutonomousSystem) SetupFullMesh(lm config.InternalLinks) []Link {
	nbRouters := len(a.Routers)
	if nbRouters < 2 {
		return nil
	}
	links := make([]Link, nbRouters*(nbRouters-1)/2)
	counter := 0
	for i := 1; i <= nbRouters; i++ {
		for j := i + 1; j <= nbRouters; j++ {
			f := a.getRouter(i)
			s := a.getRouter(j)
			links[counter] = Link{
				First:  NewNetInterface(f),
				Second: NewNetInterface(s),
			}
			links[counter].First.Description = fmt.Sprintf("linked to %s", s.Hostname)
			links[counter].Second.Description = fmt.Sprintf("linked to %s", f.Hostname)
			counter++
		}
	}
	return links
}
