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
 set community %[3]s
 set local-preference %[4]s
!
route-map CUSTOMER_IN permit 10
 set community %[5]s
 set local-preference %[6]s
!
route-map PROVIDER_IN permit 10
 set community %[1]s
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
