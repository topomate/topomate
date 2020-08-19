package frr

import (
	"fmt"
	"io"
	"strconv"

	"github.com/rahveiz/topomate/config"
)

func writeRelationsMaps(dst io.Writer, asn int) {

	// Default route maps
	provComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Provider.Community)
	provLP := strconv.Itoa(config.DefaultBGPSettings.Provider.LocalPref)
	peerComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Peer.Community)
	peerLP := strconv.Itoa(config.DefaultBGPSettings.Peer.LocalPref)
	custComm := fmt.Sprintf("%d:%d", asn, config.DefaultBGPSettings.Customer.Community)
	custLP := strconv.Itoa(config.DefaultBGPSettings.Customer.LocalPref)
	fmt.Fprintf(dst,
		`!
bgp community-list standard PROVIDER permit %[1]s
bgp community-list standard PEER permit %[3]s
bgp community-list standard CUSTOMER permit %[5]s
!
route-map PEER_OUT deny 10
 match community PROVIDER
!
route-map PEER_OUT deny 15
 match community PEER
!
route-map PEER_OUT permit 20
!
route-map PROVIDER_OUT deny 10
 match community PEER
!
route-map PROVIDER_OUT deny 15
 match community PROVIDER
!
route-map PROVIDER_OUT permit 20
!
route-map CUSTOMER_OUT permit 20
!
route-map PEER_IN permit 20
 set community additive %[3]s
 set local-preference %[4]s
!
route-map CUSTOMER_IN permit 10
 set community additive %[5]s
 set local-preference %[6]s
!
route-map PROVIDER_IN permit 10
 set community additive %[1]s
 set local-preference %[2]s
!
`, provComm, provLP, peerComm, peerLP, custComm, custLP)

	sep(dst)
}

func writeRPKIMaps(dst io.Writer) {
	fmt.Fprintf(dst,
		`!
route-map RPKI permit 10
 match rpki valid
 !
route-map RPKI deny 20
!
`)
}

func writeOwnPrefix(dst io.Writer, prefix string, order int, is6 bool) {
	sep(dst)
	if !is6 {
		fmt.Fprintln(dst, "ip prefix-list OWN_PREFIX permit", prefix, "le 32")
	} else {
		fmt.Fprintln(dst, "ipv6 prefix-list OWN_PREFIX permit", prefix, "le 128")
	}
	fmt.Fprintln(dst, "route-map OWN_PREFIX permit", order)
	if !is6 {
		fmt.Fprintln(dst, " match ip address prefix-list OWN_PREFIX")
	} else {
		fmt.Fprintln(dst, " match ipv6 address prefix-list OWN_PREFIX")
	}
	sep(dst)
}

func (c *FRRConfig) writeUtilities(dst io.Writer) {
	fmt.Fprintf(dst,
		`
! ###################################################################
! Utility items (generated for all routers by default)
!
`)
	writeComment(dst, " Own Prefix")
	n := 1
	for _, p := range c.BGP.Networks.V4 {
		writeOwnPrefix(dst, p, n, false)
		n++
	}

	for _, p := range c.BGP.Networks.V6 {
		writeOwnPrefix(dst, p, n, true)
		n++
	}

	writeComment(dst, "BGP relations maps")
	writeRelationsMaps(dst, c.BGP.ASN)
	writeComment(dst, "RPKI filter maps")
	writeRPKIMaps(dst)
}
