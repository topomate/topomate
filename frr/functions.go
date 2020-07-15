package frr

import (
	"fmt"
	"io"
)

func indent(w io.Writer, depth int) {
	for i := 0; i < depth; i++ {
		fmt.Fprint(w, " ")
	}
}

func writeWithIndent(w io.Writer, depth int, s string) {
	indent(w, depth)
	fmt.Fprintln(w, s)
}

func (r RouteRedistribution) Write(w io.Writer, indent int) {
	if r.Connected {
		writeWithIndent(w, indent, "redistribute connected")
	}
	if r.Static {
		writeWithIndent(w, indent, "redistribute static")
	}
	if r.OSPF {
		writeWithIndent(w, indent, "redistribute ospf")
	}
	if r.ISIS {
		writeWithIndent(w, indent, "redistribute isis")
	}
}

func (c *FRRConfig) internalIfs() map[string]IfConfig {
	res := make(map[string]IfConfig, len(c.Interfaces))
	for n, i := range c.Interfaces {
		if !i.External {
			res[n] = i
		}
	}
	return res
}

func (c *FRRConfig) externalIfs() map[string]IfConfig {
	res := make(map[string]IfConfig, len(c.Interfaces))
	for n, i := range c.Interfaces {
		if i.External {
			res[n] = i
		}
	}
	return res
}

func isisTypeString(t int) string {
	var ctype string
	switch t {
	case 1:
		ctype = "level-1"
		break
	case 2:
		ctype = "level-2-only"
		break
	default:
		ctype = "level-1-2"
		break
	}
	return ctype
}

func (c ISISIfConfig) Write(dst io.Writer) {
	ipver := " ip"
	if c.V6 {
		ipver = " ipv6"
	}
	fmt.Fprintln(dst, ipver, "router isis", c.ProcessName)

	fmt.Fprintln(dst, " isis circuit-type", isisTypeString(c.CircuitType))
}

func (c OSPFIfConfig) Write(dst io.Writer) {
	if c.ProcessID > 0 {
		fmt.Fprintf(dst, " ip ospf %d area %d\n", c.ProcessID, c.Area)
	} else {
		fmt.Fprintln(dst, " ip ospf area", c.Area)
	}
	fmt.Fprintln(dst, " bandwidth", c.Cost)
}
