package project

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
)

type Net struct {
	IPNet         *net.IPNet
	NextAvailable *net.IPNet
}

func (n Net) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.IPNet.String())
}

func (n *Net) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	_, ipnet, err := net.ParseCIDR(s)
	if err == nil {
		n.IPNet = ipnet
		return nil
	}
	return err
}

// AllIPs returns a slice containing all IPs in a network
// (identifier and broadcast included)
func (n Net) AllIPs() []net.IP {
	nbHosts := n.Size()
	hosts := make([]net.IP, nbHosts)
	mask := binary.BigEndian.Uint32(n.IPNet.Mask)
	start := binary.BigEndian.Uint32(n.IPNet.IP)

	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)

	// loop through addresses as uint32
	cnt := 0
	for i := start; i <= finish; i++ {
		// fmt.Println("pok")
		// convert back to net.IP
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		hosts[cnt] = ip
		cnt++
	}
	return hosts
}

// Size returns the size of a network (number of addresses)
func (n Net) Size() int {
	m, _ := n.IPNet.Mask.Size()
	return int(math.Pow(2, 32-float64(m)))
}

// Hosts returns a slice of hosts in a network
func (n Net) Hosts() []net.IP {
	return n.AllIPs()[1 : n.Size()-1]
}
