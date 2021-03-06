package project

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
)

const (
	defaultSubnetLength4 = 30
)

type IGPSettings struct {
	ISIS struct {
		Circuit int
		Passive bool
	}
	OSPFArea int
}

type NetInterface struct {
	IfName      string
	Description string
	IP          net.IPNet
	Speed       int
	External    bool
	Cost        int
	VRF         string
	IGP         IGPSettings
}

type LinkItem struct {
	Router    *Router
	Interface *NetInterface
}

type Link struct {
	First  *LinkItem
	Second *LinkItem
}

func NewLinkItem(router *Router) *LinkItem {
	ifName := fmt.Sprintf("eth%d", router.NextInterface)
	router.NextInterface++
	return &LinkItem{
		Router: router,
		Interface: &NetInterface{
			IfName: ifName,
			IP:     net.IPNet{},
			Speed:  10000,
			Cost:   10000,
		},
	}
}

func (l *NetInterface) SetSpeedAndCost(v int) {
	l.Speed = v
	l.Cost = v
}

func (l *NetInterface) IsDownstreamVRF() bool {
	return strings.HasSuffix(l.VRF, "_down")
}

// SetupManual generates an internal links configuration based on the provided
// informations
func (a *AutonomousSystem) SetupManual(lm config.InternalLinks, noCost bool) []Link {
	var links []Link
	if lm.Specs == nil {
		if lm.Filepath == "" {
			utils.Fatalln("Manual link setup error: please provide either a file or specs")
		}
		if filepath.IsAbs(lm.Filepath) {
			links = a.internalFromFile(lm.Filepath)
		} else {
			links = a.internalFromFile(config.ConfigDir + "/" + lm.Filepath)
		}
	} else {
		links = make([]Link, len(lm.Specs))
		for idx, v := range lm.Specs {
			l := Link{}
			var f, s *Router
			if first, ok := v["first"]; ok {
				f = a.getRouter(first)
				l.First = NewLinkItem(f)
			} else {
				utils.Fatalln("Manual link setup error: first key missing")
			}
			if second, ok := v["second"]; ok {
				s = a.getRouter(second)
				l.Second = NewLinkItem(s)
			} else {
				utils.PrintError("Manual link setup error: second key missing")
			}
			l.First.Interface.Description = fmt.Sprintf("linked to %s", s.Hostname)
			l.Second.Interface.Description = fmt.Sprintf("linked to %s", f.Hostname)
			links[idx] = l
		}
	}

	// if a preset is present
	switch strings.ToLower(lm.Preset) {
	case "ring":
		links = append(links, a.SetupRing(lm, noCost)...)
		break
	case "full-mesh":
		links = append(links, a.SetupFullMesh(lm, noCost)...)
		break
	default:
		break
	}
	return links
}

// SetupRing generates an internal links configuration using a ring topology
func (a *AutonomousSystem) SetupRing(lm config.InternalLinks, noCost bool) []Link {
	nbRouters := len(a.Routers)
	if nbRouters < 3 {
		utils.Fatalln("Cannot create ring topology with less than 3 routers.")
	}
	links := make([]Link, nbRouters)
	for i := 1; i <= nbRouters; i++ {
		f := a.getRouter(i)
		s := a.getRouter((i % nbRouters) + 1)
		links[i-1] = Link{
			First:  NewLinkItem(f),
			Second: NewLinkItem(s),
		}
		if noCost {
			if lm.Speed > 0 {
				links[i-1].First.Interface.Speed = lm.Speed
				links[i-1].Second.Interface.Speed = lm.Speed
			}
			links[i-1].First.Interface.Cost = 0
			links[i-1].Second.Interface.Cost = 0
		} else {
			if lm.Speed > 0 {
				links[i-1].First.Interface.SetSpeedAndCost(lm.Speed)
				links[i-1].Second.Interface.SetSpeedAndCost(lm.Speed)
			}
		}
		if lm.Cost > 0 {
			links[i-1].First.Interface.Cost = lm.Cost
			links[i-1].Second.Interface.Cost = lm.Cost
		}

		links[i-1].First.Interface.Description = fmt.Sprintf("linked to %s", s.Hostname)
		links[i-1].Second.Interface.Description = fmt.Sprintf("linked to %s", f.Hostname)
	}
	return links
}

// SetupFullMesh generates an internal links configuration using a full-mesh topology
func (a *AutonomousSystem) SetupFullMesh(lm config.InternalLinks, noCost bool) []Link {
	nbRouters := len(a.Routers)
	// if nbRouters < 2 {
	// 	return nil
	// }
	links := make([]Link, nbRouters*(nbRouters-1)/2)
	counter := 0
	for i := 1; i <= nbRouters; i++ {
		for j := i + 1; j <= nbRouters; j++ {
			f := a.getRouter(i)
			s := a.getRouter(j)
			links[counter] = Link{
				First:  NewLinkItem(f),
				Second: NewLinkItem(s),
			}
			if noCost {
				if lm.Speed > 0 {
					links[counter].First.Interface.Speed = lm.Speed
					links[counter].Second.Interface.Speed = lm.Speed
				}
				links[counter].First.Interface.Cost = 0
				links[counter].Second.Interface.Cost = 0
			} else {
				if lm.Speed > 0 {
					links[counter].First.Interface.SetSpeedAndCost(lm.Speed)
					links[counter].Second.Interface.SetSpeedAndCost(lm.Speed)
				}
			}
			if lm.Cost > 0 {
				links[counter].First.Interface.Cost = lm.Cost
				links[counter].Second.Interface.Cost = lm.Cost
			}
			links[counter].First.Interface.Description = fmt.Sprintf("linked to %s", s.Hostname)
			links[counter].Second.Interface.Description = fmt.Sprintf("linked to %s", f.Hostname)
			counter++
		}
	}
	return links
}
