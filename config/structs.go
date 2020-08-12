package config

const (
	fromCustomer = 10
	fromProvider = 20
	fromPeer     = 30
)

type BaseConfig struct {
	Name         string                `yaml:"name,omitempty"`
	Global       GlobalConfig          `yaml:"global_settings"`
	AS           []ASConfig            `yaml:"autonomous_systems"`
	ExternalFile string                `yaml:"external_links_file"`
	External     []ExternalLink        `yaml:"external_links"`
	IXPs         []IXPConfig           `yaml:"ixps"`
	RPKI         map[string]RPKIConfig `yaml:"rpki"`
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
	ASN        int           `yaml:"asn,omitempty"`
	NumRouters int           `yaml:"routers,omitempty"`
	IGP        string        `yaml:"igp,omitempty"`
	ISIS       ISISConfig    `yaml:"isis"`
	OSPF       OSPFConfig    `yaml:"ospf"`
	Prefix     string        `yaml:"prefix,omitempty"`
	LoRange    string        `yaml:"loopback_start,omitempty"`
	BGP        BGPConfig     `yaml:"bgp"`
	Links      InternalLinks `yaml:"links,omitempty"`
	MPLS       bool          `yaml:"mpls,omitempty"`
	VPN        []VPNConfig
	RPKI       struct {
		Servers []string `yaml:"servers"`
	} `yaml:"rpki"`
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
	IBGP            IBGPConfig `yaml:"ibgp"`
	Disabled        bool       `yaml:"disabled"`
	RedistributeIGP bool       `yaml:"redistribute_igp"`
}

type VPNConfig struct {
	VRF       string `yaml:"vrf"`
	HubMode   bool   `yaml:"hub_and_spoke"`
	Customers []struct {
		Hostname     string `yaml:"hostname"`
		Loopback     string `yaml:"loopback"`
		RemoteSubnet string `yaml:"remote_subnet"`
		Subnet       string `yaml:"subnet"`
		SubnetDown   string `yaml:"downstream_subnet"`
		Parent       int    `yaml:"parent"`
		Hub          bool
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

type IXPConfig struct {
	ASN      int      `yaml:"asn"`
	Peers    []string `yaml:"peers,flow"`
	Prefix   string   `yaml:"prefix"`
	Loopback string   `yaml:"loopback"`
}

type ISISConfig struct {
	L1    []int         `yaml:"level-1,flow"`
	L2    []int         `yaml:"level-2,flow"`
	L12   []int         `yaml:"level-1-2,flow"`
	Areas map[int][]int `yaml:"areas,flow"`
}

type networkOSPF struct {
	Prefix  string `yaml:"prefix"`
	Area    int    `yaml:"area"`
	Routers []int  `yaml:"routers,flow"`
}

type OSPFConfig struct {
	Networks []networkOSPF `yaml:"networks"`
	Stubs    []int         `yaml:"stubs"`
	// Areas map[int]struct {
	// 	Networks []string `yaml:"networks,flow"`
	// 	Routers  []int    `yaml:"routers,flow"`
	// 	Stub     bool     `yaml:"stub"`
	// } `yaml:"areas"`
}

type RPKIConfig struct {
	// ASN        int      `yaml:"asn"`
	Address string `yaml:"server_address"`
	// NeighborAS []string `yaml:"neighbors_as,flow"`
	RouterLink struct {
		ASN      int `yaml:"asn"`
		RouterID int `yaml:"router_id"`
	} `yaml:"linked_to"`
	CacheFile string `yaml:"cache_file"`
}
