package config

func getOrDefaultInt(val, def int) int {
	if val == 0 {
		return def
	}
	return val
}

func (c *GlobalBGPConfig) ToGlobal() {
	cfg := GlobalBGPConfig{
		Customer: BGPRelationConfig{
			Community: getOrDefaultInt(c.Customer.Community, fromCustomer),
			LocalPref: getOrDefaultInt(c.Customer.LocalPref, 300),
		},
		Provider: BGPRelationConfig{
			Community: getOrDefaultInt(c.Provider.Community, fromProvider),
			LocalPref: getOrDefaultInt(c.Provider.LocalPref, 100),
		},
		Peer: BGPRelationConfig{
			Community: getOrDefaultInt(c.Peer.Community, fromPeer),
			LocalPref: getOrDefaultInt(c.Peer.LocalPref, 200),
		},
	}
	DefaultBGPSettings = cfg
}
