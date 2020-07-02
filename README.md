# topomate (WIP)

[![Go Report Card](https://goreportcard.com/badge/github.com/rahveiz/topomate)](https://goreportcard.com/report/github.com/rahveiz/topomate)
![CI](https://github.com/rahveiz/topomate/workflows/CI/badge.svg)

Network Topology Automation using FRRouting containers


## How does it work ?

Topomate reads a YAML configuration file describing a network topology,
and generates containers, links and configurations from it.

### Example configuration file

```yaml
autonomous_systems:
  - asn: 1
    routers: 2
    loopback_start: '172.16.10.1/32'
    igp: OSPF
    redistribute_igp: true
    prefix: '10.1.1.0/24'
    mpls: true
    links:
      kind: 'full-mesh'
      subnet_length: 30
  - asn: 2
    routers: 2
    loopback_start: '172.16.20.1/32'
    igp: OSPF
    redistribute_igp: true
    prefix: '10.1.2.0/24'
    links:
      kind: 'full-mesh'
      subnet_length: 30
  - asn: 3
    routers: 2
    loopback_start: '172.16.30.1/32'
    igp: OSPF
    redistribute_igp: true
    prefix: '10.1.3.0/24'
    links:
      kind: 'full-mesh'
      subnet_length: 30
  - asn: 4
    routers: 2
    loopback_start: '172.16.40.1/32'
    igp: OSPF
    redistribute_igp: true
    prefix: '10.1.4.0/24'
    links:
      kind: 'full-mesh'
      subnet_length: 30

external_links:
  - from:
      asn: 1
      router_id: 1
    to:
      asn: 2
      router_id: 1
    rel: 'p2c'
  - from:
      asn: 2
      router_id: 1
    to:
      asn: 3
      router_id: 1
    rel: 'p2p'
  - from:
      asn: 2
      router_id: 1
    to:
      asn: 4
      router_id: 1
    rel: 'p2c'
```

This file will :

* generate 8 containers (AS1-R1, AS1-R2, ..., AS4-R2) and the corresponding FRRouting
configuration files (that will be copied to the corresponding container)
* create 1 OVS bridge per AS for internal links that will interconnect containers
(using veth pairs, and OpenFlow rules)
* create 1 OVS bridge per external link

Currently, the router ID used for BGP and OSPF is the first loopback address.

## Notes concerning MPLS

If you want to use MPLS, the following kernel modules must be enabled on the host machine

```
mpls_router
mpls_iptunnel
```
