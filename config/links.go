package config

import (
	"fmt"
	"net"
	"strconv"

	"github.com/rahveiz/topomate/utils"
)

type NetLink struct {
	RouterIndex int
	IP          net.IP
	Speed       int
}

type Link struct {
	First  NetLink
	Second NetLink
}

type LinkModule struct {
	Kind  string              `yaml:"kind"`
	Specs []map[string]string `yaml:"specs"`
}

// IfNames returns the name of both interfaces of a link
// as "eth" + destination index
func (l Link) IfNames() (string, string) {
	a := fmt.Sprintf("eth%d", l.Second.RouterIndex)
	b := fmt.Sprintf("eth%d", l.First.RouterIndex)
	return a, b
}

func (l Link) BrName(asn int) string {
	return fmt.Sprintf(
		"as%d-br-%d-%d",
		asn, l.First.RouterIndex,
		l.Second.RouterIndex,
	)
}

func NewNetLink(index interface{}) NetLink {
	var idx int
	var err error
	switch index.(type) {
	case int:
		idx = index.(int)
		break
	case string:
		idx, err = strconv.Atoi(index.(string))
		if err != nil {
			utils.Fatalln(err)
		}
		break
	default:
		utils.Fatalln("NewNetLink: index type mismtach")
	}

	return NetLink{
		RouterIndex: idx,
		IP:          net.IP{},
		Speed:       10000,
	}
}

func (lm *LinkModule) SetupManual() []Link {
	links := make([]Link, len(lm.Specs))
	for idx, v := range lm.Specs {
		l := Link{}
		if first, ok := v["first"]; ok {
			l.First = NewNetLink(first)
		} else {
			utils.Fatalln("Manual link setup error: first key missing")
		}
		if second, ok := v["second"]; ok {
			l.Second = NewNetLink(second)
		} else {
			utils.PrintError("Manual link setup error: second key missing")
		}
		links[idx] = l
	}
	return links
}

func (lm *LinkModule) SetupRing(nbRouters int) []Link {
	if nbRouters < 3 {
		utils.Fatalln("Cannot create ring topology with less than 3 routers.")
	}
	links := make([]Link, nbRouters)
	for i := 1; i <= nbRouters; i++ {
		links[i-1] = Link{
			First:  NewNetLink(i),
			Second: NewNetLink((i % nbRouters) + 1),
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
				First:  NewNetLink(i),
				Second: NewNetLink(j),
			}
			counter++
		}
	}
	return links
}
