package project

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
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

// NextIP returns the current NextAvailable IPNet, then increments the IP by one
func (n *Net) NextIP() net.IPNet {
	res := *n.NextAvailable
	n.NextAvailable.IP = cidr.Inc(n.NextAvailable.IP)
	return res
}

// Is4 returns true if Net is an IPV4 network
func (n Net) Is4() bool {
	return n.IPNet.IP.To4() != nil
}

// CheckPrefix returns the subnet length of the network. The second value
// return is true if the prefixLen provided is valable.
func (n Net) CheckPrefix(prefixLen int) (int, bool) {
	m, max := n.IPNet.Mask.Size()
	return m, !(prefixLen < m || prefixLen > max)
}
