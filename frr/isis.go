package frr

import (
	"fmt"
	"io"
)

type ISISConfig struct {
	ProcessName  string
	ISO          string
	Type         int
	Redistribute RouteRedistribution
	VRF          string
}

type ISISIfConfig struct {
	V4          bool
	V6          bool
	ProcessName string
	CircuitType int
	Cost        int
	Passive     bool
}

func (c ISISConfig) writeISIS(dst io.Writer, v4, v6 bool) {
	sep(dst)

	fmt.Fprintln(dst, "router isis", c.ProcessName)
	fmt.Fprintln(dst, " net", c.ISO)
	fmt.Fprintln(dst, " metric-style wide")
	fmt.Fprintln(dst, " is-type", isisTypeString(c.Type))

	// If L1L2, we distribute a default route to the L1 neighbors
	if c.Type == 3 {
		fmt.Fprintln(dst, " set-attached-bit")
		if v4 {
			fmt.Fprintln(dst, " default-information originate ipv4 level-1 always")
		}
		if v6 {
			fmt.Fprintln(dst, " default-information originate ipv6 level-1 always")
		}
	}

	// Here we write the redistribution manually as ISIS syntax is not standard
	c.writeRedistribute(dst, v4, v6)

	sep(dst)
}

func (c ISISIfConfig) Write(dst io.Writer) {
	if c.V4 {
		fmt.Fprintln(dst, " ip router isis", c.ProcessName)
	}
	if c.V6 {
		fmt.Fprintln(dst, " ipv6 router isis", c.ProcessName)
	}

	if !c.Passive {
		fmt.Fprintln(dst, " isis circuit-type", isisTypeString(c.CircuitType))
	} else {
		fmt.Fprintln(dst, " isis passive")
	}
	if c.Cost > 0 {
		fmt.Fprintln(dst, " isis metric", c.Cost)
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
