package frr

import (
	"net"
	"strings"

	"github.com/rahveiz/topomate/project"
)

func generateVPNConfig(as *project.AutonomousSystem, ASconfigs []*FRRConfig) []*FRRConfig {
	is4 := as.Network.Is4()
	total := 0
	for _, vpn := range as.VPN {
		total += len(vpn.Customers)
	}
	res := make([]*FRRConfig, 0, total)

	for _, vpn := range as.VPN {
		rtIn, rtOut := nextRouteTarget, nextRouteTarget

		// if hub is set, we need a second route-target
		if vpn.IsHubAndSpoke() {
			rtOut++
			nextRouteTarget++
		}

		for _, r := range vpn.Customers {
			c := &FRRConfig{
				Hostname:     r.Router.Hostname,
				Interfaces:   make(map[string]IfConfig, 4),
				StaticRoutes: initStatic(len(r.Router.Links)),
			}

			// Setup loopback interface
			nbLo := len(r.Router.Loopback)
			if nbLo > 0 {
				ips := make([]net.IPNet, nbLo)
				for idx, ip := range r.Router.Loopback {
					ips[idx] = ip
				}
				c.Interfaces["lo"] = IfConfig{
					IPs: ips,
				}
			}

			// IGP
			igp := strings.ToUpper(as.IGP)
			parentCfg := ASconfigs[r.Parent.ID-1]

			parentRt := RouteTarget{
				In:  rtIn,
				Out: rtOut,
			}

			// if we use hub-and-spoke VPN, some modifications are made on the hub
			if vpn.IsHubAndSpoke() && r.Hub {
				// invert the route-targets for the hub (only need to export)
				parentRt = RouteTarget{
					Out: rtIn,
				}

				// on the hub PE, we also add config for the downstream vrf
				parentCfg.BGP.VRF[vpn.VRF+"_down"] = VRFConfig{
					RD: nextRouteDescriptor,
					RT: RouteTarget{
						In: rtOut,
					},
				}
			}

			// if BGPVRF config is not present in parent, add it
			if _, ok := parentCfg.BGP.VRF[vpn.VRF]; !ok {
				parentCfg.BGP.VRF[vpn.VRF] = VRFConfig{
					RD: nextRouteDescriptor,
					RT: parentRt,
					Redistribute: RouteRedistribution{
						OSPF: true,
					},
				}
			}

			// setup IGP for CE and VRF IGP for PE
			switch igp {
			case "OSPF":
				// Check if we need to setup OSPFv2 or OSPFv3
				if is4 {
					oCfg := getOSPFConfig(c.BGP.RouterID, 0)
					if r.Hub {
						oCfg.Redistribute.Static = true
					}
					c.IGP = append(c.IGP, oCfg)

					// Add IGP on the parent side (parent index in array is
					// its ID - 1, as usual)
					parentIGP := getOSPFConfig(parentCfg.BGP.RouterID, 0)
					parentIGP.Redistribute.BGP = true
					parentIGP.VRF = vpn.VRF
					parentCfg.IGP = append(
						parentCfg.IGP,
						parentIGP,
					)
				} else {
					c.IGP = append(c.IGP, getOSPF6Config(c.BGP.RouterID))
				}
				break
			case "IS-IS", "ISIS":
				c.IGP = append(c.IGP,
					c.getISISConfig(1, 2, RouteRedistribution{
						// Connected: true,
					}))
				parentIGP := parentCfg.getISISConfig(1, 2,
					RouteRedistribution{
						BGP: true,
					})
				parentIGP.VRF = vpn.VRF
				parentCfg.IGP = append(
					parentCfg.IGP,
					parentIGP,
				)
				break
			default:
				break
			}

			// Interfaces
			for _, iface := range r.Router.Links {
				ifCfg := IfConfig{
					IPs:         []net.IPNet{iface.IP},
					Description: iface.Description,
					Speed:       iface.Speed,
					IGPConfig:   make([]IGPIfConfig, 0, 5),
				}
				switch igp {
				case "OSPF":
					ifIGP := OSPFIfConfig{
						V6:        !is4,
						Cost:      iface.Cost,
						ProcessID: 0,
						Area:      0,
					}

					// find the PE interface and configure it
					if parentIf := as.GetMatchingLink(nil, iface); parentIf != nil {
						pIfCfg := IfConfig{
							IPs:         []net.IPNet{parentIf.IP},
							Description: parentIf.Description,
							Speed:       parentIf.Speed,
							IGPConfig:   make([]IGPIfConfig, 0, 5),
							External:    true,
							VRF:         parentIf.VRF,
						}

						ifCfg.IGPConfig = append(ifCfg.IGPConfig, ifIGP)
						pIfCfg.IGPConfig = append(pIfCfg.IGPConfig, OSPFIfConfig{
							V6:        !is4,
							Cost:      parentIf.Cost,
							ProcessID: 0,
							Area:      0,
						})

						// if we are in the hub side, add the static routes to the spokes
						// we need to use the IP of the parent interface
						if r.Hub && parentIf.IsDownstreamVRF() {
							key := parentIf.IP.IP.String()
							for _, subnet := range vpn.SpokeSubnets {
								c.StaticRoutes.V4[key] =
									append(c.StaticRoutes.V4[key], subnet.String())
							}
						}

						parentCfg.Interfaces[parentIf.IfName] = pIfCfg
					}
				case "ISIS", "IS-IS":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, ISISIfConfig{
							V6:          !is4,
							ProcessName: isisDefaultProcess,
							Cost:        iface.Cost,
							CircuitType: 2,
						})
					break
				}
				c.Interfaces[iface.IfName] = ifCfg
			}

			// Also add IGP config for loopback interface
			if nbLo > 0 {
				ifCfg := c.Interfaces["lo"]
				switch igp {
				case "OSPF":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, OSPFIfConfig{
							V6:        !is4,
							ProcessID: 0,
							Area:      0,
						})
				case "ISIS", "IS-IS":
					ifCfg.IGPConfig =
						append(ifCfg.IGPConfig, ISISIfConfig{
							V6:          !is4,
							ProcessName: isisDefaultProcess,
							Passive:     true,
						})
					break
				}
				c.Interfaces["lo"] = ifCfg
			}

			res = append(res, c)
		}
		nextRouteDescriptor++
		nextRouteTarget++
	}
	return res
}
