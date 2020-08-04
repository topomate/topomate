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
	if r.BGP {
		writeWithIndent(w, indent, "redistribute bgp")
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

	if !c.Passive {
		fmt.Fprintln(dst, " isis circuit-type", isisTypeString(c.CircuitType))
	} else {
		fmt.Fprintln(dst, " isis passive")
	}
	if c.Cost > 0 {
		fmt.Fprintln(dst, " isis metric", c.Cost)
	}
}

func (c OSPFIfConfig) Write(dst io.Writer) {
	if c.ProcessID > 0 {
		fmt.Fprintf(dst, " ip ospf %d area %d\n", c.ProcessID, c.Area)
	} else {
		fmt.Fprintln(dst, " ip ospf area", c.Area)
	}
	if c.Cost > 0 {
		fmt.Fprintln(dst, " bandwidth", c.Cost)
	}
}

func (c ISISConfig) writeRedistribution(w io.Writer, af string, level string) {
	if c.Redistribute.Connected {
		fmt.Fprintln(w, " redistribute", af, "connected", level)
	}
	if c.Redistribute.Static {
		fmt.Fprintln(w, " redistribute", af, "static", level)
	}
	if c.Redistribute.OSPF {
		fmt.Fprintln(w, " redistribute", af, "ospf", level)
	}
	if c.Redistribute.BGP {
		fmt.Fprintln(w, " redistribute", af, "bgp", level)
	}
}

func (c ISISConfig) writeRedistribute(dst io.Writer, v4 bool, v6 bool) {
	if v4 {
		switch c.Type {
		case 1:
			c.writeRedistribution(dst, "ipv4", "level-1")
			break
		case 2:
			c.writeRedistribution(dst, "ipv4", "level-2")
			break
		default:
			c.writeRedistribution(dst, "ipv4", "level-1")
			c.writeRedistribution(dst, "ipv4", "level-2")
		}
	}
	if v6 {
		if v4 {
			switch c.Type {
			case 1:
				c.writeRedistribution(dst, "ipv6", "level-1")
				break
			case 2:
				c.writeRedistribution(dst, "ipv6", "level-2")
				break
			default:
				c.writeRedistribution(dst, "ipv6", "level-1")
				c.writeRedistribution(dst, "ipv6", "level-2")
			}
		}
	}
}

func (pl *PrefixList) WriteMatch(dst io.Writer) {
	fmt.Fprintln(dst, " match ip address prefix-list", pl.Name)
}
