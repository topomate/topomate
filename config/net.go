package config

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
)

type Net struct {
	IPNet *net.IPNet
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

func (n *Net) getIPHosts() []net.IP {
	m, _ := n.IPNet.Mask.Size()
	nbHosts := int(math.Pow(2, 32-float64(m)))
	hosts := make([]net.IP, nbHosts)
	mask := binary.BigEndian.Uint32(n.IPNet.Mask)
	start := binary.BigEndian.Uint32(n.IPNet.IP)

	fmt.Println(nbHosts, "hosts")

	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)
	fmt.Println(finish)

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
