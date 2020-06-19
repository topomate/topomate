package project

import "net"

type ExternalEndpoint struct {
	ASN    int
	Router *Router
	IP     *net.IPNet
}

type ExternalLink struct {
	From ExternalEndpoint
	To   ExternalEndpoint
}
