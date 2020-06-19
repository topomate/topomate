# topomate (WIP)

[![Go Report Card](https://goreportcard.com/badge/github.com/rahveiz/topomate)](https://goreportcard.com/report/github.com/rahveiz/topomate)
![CI](https://github.com/rahveiz/topomate/workflows/CI/badge.svg)

Network Topology Automation using FRRouting containers


## How does it work ?

Topomate reads a YAML configuration file describing a network topology,
and generates containers, links and configurations from it.

### Configuration example

The following configuration :


```yaml
autonomous_systems:
  - asn: 10
    routers: 4
    igp: OSPF
    prefix: '192.168.8.0/26'
    links:
      kind: 'full-mesh'
      subnet_length: 30
  - asn: 20
    routers: 2
    igp: OSPF
    prefix: '10.10.10.0/28'
    links:
      kind: 'full-mesh'
      subnet_length: 30  
  
external_links:
  - from:
      asn: 10
      router_id: 1
    to:
      asn: 20
      router_id: 2
```

will generate 6 containers (AS10-R1..4 and AS20-R1..2), create bridges and links
using OVS, and generate FRR configuration files.

For AS10-R1, the following configuration will be generated :

```
frr version 7.3
frr defaults traditional
hostname R1
log syslog informational
no ipv6 forwarding
service integrated-vtysh-config
!
!
interface eth0
 description linked to R2
 ip address 192.168.8.1/30
!
!
interface eth1
 description linked to R3
 ip address 192.168.8.5/30
!
!
interface eth2
 description linked to R4
 ip address 192.168.8.9/30
!
!
interface eth3
 description linked to AS20 (R2)
 ip address 192.168.8.25/30
!
!
router bgp 10
 neighbor 192.168.8.2 remote-as 10
 neighbor 192.168.8.2 update-source lo
 neighbor 192.168.8.2 disable-connected-check
 neighbor 192.168.8.6 remote-as 10
 neighbor 192.168.8.6 update-source lo
 neighbor 192.168.8.6 disable-connected-check
 neighbor 192.168.8.10 remote-as 10
 neighbor 192.168.8.10 update-source lo
 neighbor 192.168.8.10 disable-connected-check
 neighbor 192.168.8.26 remote-as 20
 neighbor 192.168.8.26 update-source lo
 neighbor 192.168.8.26 disable-connected-check
 !
 address-family ipv4 unicast
  network 192.168.8.0/26
 exit-address-family
!
!
router ospf
 redistribute connected
!
line vty
```
