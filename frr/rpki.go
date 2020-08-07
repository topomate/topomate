package frr

import (
	"fmt"
	"io"

	"github.com/rahveiz/topomate/project"
)

func writeRPKI(dst io.Writer, servers map[string]project.RPKIServer, selected []string) {
	if len(selected) == 0 {
		return
	}
	sep(dst)
	fmt.Fprintln(dst, "rpki")
	for idx, s := range selected {
		srv, ok := servers[s]
		if ok {
			fmt.Fprintln(dst, " rpki cache", srv.IP, srv.Port, "preference", idx+1)
		}
	}
	sep(dst)
}
