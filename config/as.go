package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/link"
)

// AutonomousSystem represents an AS in a Project
type AutonomousSystem struct {
	ASN     int
	IGP     string
	Network Net
	Routers []Router
	Links   []Link
}

func (a *AutonomousSystem) getContainerName(n string) string {
	return "AS" + strconv.Itoa(a.ASN) + "-R" + n
}

// SetupLinks generates the L2 configuration based on provided config
func (a *AutonomousSystem) SetupLinks(cfg LinkModule) {
	switch kind := strings.ToLower(cfg.Kind); kind {
	case "manual":
		a.Links = cfg.SetupManual()
	case "ring":
		a.Links = cfg.SetupRing(len(a.Routers))
	case "full-mesh":
		a.Links = cfg.SetupFullMesh(len(a.Routers))
	default:
		fmt.Println("Not implemented")
	}
}

// ApplyLinks applies the L2 configuration using OVS
func (a *AutonomousSystem) ApplyLinks() {
	for _, v := range a.Links {
		brName := fmt.Sprintf("as%d-br-%s-%s", a.ASN, v.First, v.Second)
		link.CreateBridge(brName)
		link.AddPortToContainer(brName, "eth"+v.Second, a.getContainerName(v.First))
		link.AddPortToContainer(brName, "eth"+v.First, a.getContainerName(v.Second))
	}
}

// RemoveLinks removes the L2 configuration of an AS
func (a *AutonomousSystem) RemoveLinks() {
	for _, v := range a.Links {
		brName := fmt.Sprintf("as%d-br-%s-%s", a.ASN, v.First, v.Second)
		link.DeleteBridge(brName)
	}
}
