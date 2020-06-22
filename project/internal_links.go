package project

import (
	"fmt"
	"net"
)

const (
	defaultSubnetLength4 = 30
)

type NetInterface struct {
	RouterID    int
	IfName      string
	Description string
	IP          net.IPNet
	Speed       int
	External    bool
}

type Link struct {
	First  *NetInterface
	Second *NetInterface
}

// IfNames returns the name of both interfaces of a link
// as "eth" + destination index
func (l Link) IfNames() (string, string) {
	a := fmt.Sprintf("eth%d", l.Second.RouterID)
	b := fmt.Sprintf("eth%d", l.First.RouterID)
	return a, b
}

func (l Link) BrName(asn int) string {
	return fmt.Sprintf(
		"as%d-br-%d-%d",
		asn, l.First.RouterID,
		l.Second.RouterID,
	)
}

func NewNetInterface(router *Router) *NetInterface {
	ifName := fmt.Sprintf("eth%d", router.NextInterface)
	router.NextInterface++
	return &NetInterface{
		RouterID: router.ID,
		IfName:   ifName,
		IP:       net.IPNet{},
		Speed:    10000,
	}
}
