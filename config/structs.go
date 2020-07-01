package config

type BaseConfig struct {
	Name     string         `yaml:"name,omitempty"`
	AS       []ASConfig     `yaml:"autonomous_systems"`
	External []ExternalLink `yaml:"external_links"`
}

type ASConfig struct {
	ASN             int           `yaml:"asn,omitempty"`
	NumRouters      int           `yaml:"routers,omitempty"`
	IGP             string        `yaml:"igp,omitempty"`
	RedistributeIGP bool          `yaml:"redistribute_igp"`
	Prefix          string        `yaml:"prefix,omitempty"`
	LoRange         string        `yaml:"loopback_start,omitempty"`
	Links           InternalLinks `yaml:"links,omitempty"`
	MPLS            bool          `yaml:"mpls,omitempty"`
}

type ExternalEndpoint struct {
	ASN      int `yaml:"asn"`
	RouterID int `yaml:"router_id"`
}

type ExternalLink struct {
	From ExternalEndpoint `yaml:"from"`
	To   ExternalEndpoint `yaml:"to"`
}

type InternalLinks struct {
	Kind         string              `yaml:"kind"`
	SubnetLength int                 `yaml:"subnet_length"`
	Preset       string              `yaml:"preset,omitempty"`
	Specs        []map[string]string `yaml:"specs"`
}
