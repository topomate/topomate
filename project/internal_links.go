package project

import (
	"fmt"
	"net"
)

const (
	defaultSubnetLength4 = 30
)

type NetInterface struct {
	IfName      string
	Description string
	IP          net.IPNet
	Speed       int
	External    bool
}

type LinkItem struct {
	Router    *Router
	Interface *NetInterface
}

type Link struct {
	First  *LinkItem
	Second *LinkItem
}

// // IfNames returns the name of both interfaces of a link
// // as "eth" + destination index
// func (l Link) IfNames() (string, string) {
// 	a := fmt.Sprintf("eth%d", l.Second.RouterID)
// 	b := fmt.Sprintf("eth%d", l.First.RouterID)
// 	return a, b
// }

// func (l Link) BrName(asn int) string {
// 	return fmt.Sprintf(
// 		"as%d-br-%d-%d",
// 		asn, l.First.RouterID,
// 		l.Second.RouterID,
// 	)
// }

func NewLinkItem(router *Router) *LinkItem {
	ifName := fmt.Sprintf("eth%d", router.NextInterface)
	router.NextInterface++
	return &LinkItem{
		Router: router,
		Interface: &NetInterface{
			IfName: ifName,
			IP:     net.IPNet{},
			Speed:  10000,
		},
	}
}
