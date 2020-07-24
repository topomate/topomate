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

// CheckLevel returns the level of the router designed by routerID.
// It returns 1 for a L1 router, 2 for L2, 3 for L1-2 and 0 if it is not found.
func (c *ISISConfig) CheckLevel(routerID int) int {
	if c.L1 != nil {
		for _, e := range c.L1 {
			if e == routerID {
				return 1
			}
		}
	}
	if c.L2 != nil {
		for _, e := range c.L2 {
			if e == routerID {
				return 2
			}
		}
	}
	if c.L12 != nil {
		for _, e := range c.L12 {
			if e == routerID {
				return 3
			}
		}
	}
	return 0
}

// CheckArea returns the area of the router designed by routerID. It defaults to 1.
func (c *ISISConfig) CheckArea(routerID int) int {
	for area, e := range c.Areas {
		for _, i := range e {
			if i == routerID {
				return area
			}
		}
	}
	return 1
}
