package frr

import (
	"fmt"
	"io"
	"strconv"
)

type staticRoutes struct {
	V4 map[string][]string
	V6 map[string][]string
}

func initStatic(size int) staticRoutes {
	return staticRoutes{
		V4: make(map[string][]string, size),
		V6: make(map[string][]string, size),
	}
}

func (s *staticRoutes) add(dest string, prefixLen int, gateway string) {
	s.V4[gateway] = append(s.V4[gateway], dest+"/"+strconv.Itoa(prefixLen))
}

func (s *staticRoutes) add6(dest string, prefixLen int, gateway string) {
	s.V6[gateway] = append(s.V6[gateway], dest+"/"+strconv.Itoa(prefixLen))
}

func (c *staticRoutes) Write(dst io.Writer) {
	sep(dst)
	for ifName, ips := range c.V4 {
		for _, ip := range ips {
			fmt.Fprintln(dst, "ip route", ip, ifName)
		}
	}
	for ifName, ips := range c.V6 {
		for _, ip := range ips {
			fmt.Fprintln(dst, "ipv6 route", ip, ifName)
		}
	}
	sep(dst)
}
