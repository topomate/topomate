package config

const (
	fromCustomer = 10
	fromProvider = 20
	fromPeer     = 30
)

type BaseConfig struct {
	Name         string         `yaml:"name,omitempty"`
	Global       GlobalConfig   `yaml:"global_settings"`
	AS           []ASConfig     `yaml:"autonomous_systems"`
	ExternalFile string         `yaml:"external_links_file"`
	External     []ExternalLink `yaml:"external_links"`
}

type GlobalConfig struct {
	BGP GlobalBGPConfig
}

type GlobalBGPConfig struct {
	Provider BGPRelationConfig `yaml:"provider,omitempty"`
	Customer BGPRelationConfig `yaml:"customer,omitempty"`
	Peer     BGPRelationConfig `yaml:"peer,omitempty"`
}

type BGPRelationConfig struct {
	Community int `yaml:"community,omitempty"`
	LocalPref int `yaml:"local_pref,omitempty"`
}

type ASConfig struct {
	ASN             int           `yaml:"asn,omitempty"`
	NumRouters      int           `yaml:"routers,omitempty"`
	IGP             string        `yaml:"igp,omitempty"`
	RedistributeIGP bool          `yaml:"redistribute_igp"`
	Prefix          string        `yaml:"prefix,omitempty"`
	LoRange         string        `yaml:"loopback_start,omitempty"`
	BGP             BGPConfig     `yaml:"bgp"`
	Links           InternalLinks `yaml:"links,omitempty"`
	MPLS            bool          `yaml:"mpls,omitempty"`
	VPN             []VPNConfig
}

// type IBGPConfig struct {
// 	File string `yaml:"file"`
// }

type IBGPConfig struct {
	Manual bool
	RR     []struct {
		Router  int   `yaml:"router"`
		Clients []int `yaml:"clients,flow"`
	} `yaml:"route_reflectors"`
	Cliques [][]int `yaml:"cliques,flow"`
}

type BGPConfig struct {
	IBGP IBGPConfig `yaml:"ibgp"`
}

type VPNConfig struct {
	VRF       string `yaml:"vrf"`
	Customers []struct {
		Hostname string `yaml:"hostname"`
		Loopback string `yaml:"loopback"`
		Subnet   string `yaml:"subnet"`
		Parent   int    `yaml:"parent"`
	} `yaml:"customers"`
}

type ExternalLinkItem struct {
	ASN      int `yaml:"asn"`
	RouterID int `yaml:"router_id"`
}

type ExternalLink struct {
	From         ExternalLinkItem `yaml:"from"`
	To           ExternalLinkItem `yaml:"to"`
	Relationship string           `yaml:"rel"`
}

type InternalLinks struct {
	Kind         string              `yaml:"kind"`
	SubnetLength int                 `yaml:"subnet_length"`
	Preset       string              `yaml:"preset,omitempty"`
	Specs        []map[string]string `yaml:"specs,omitempty"`
	Filepath     string              `yaml:"file"`
}
