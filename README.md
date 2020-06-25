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
    loopback_start: '172.16.10.1/32'
    igp: OSPF
    redistribute_igp: true
    prefix: '192.168.8.0/26'
    links:
      kind: 'full-mesh'
      subnet_length: 30
  - asn: 20
    routers: 2
    loopback_start: '172.16.20.1/32'
    igp: OSPF
    redistribute_igp: true
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
hostname R2
log syslog informational
no ipv6 forwarding
service integrated-vtysh-config
!
!
interface lo
 ip address 172.16.20.2/32
 ip ospf area 0
!
!
interface eth0
 description linked to R1
 ip address 10.10.10.2/30
 ip ospf area 0
!
!
interface eth1
 description linked to AS10 (R1)
 ip address 192.168.8.26/30
!
!
ip route 172.16.10.1/32 eth1
!
!
router bgp 20
 bgp router-id 172.16.20.2
 neighbor 172.16.20.1 remote-as 20
 neighbor 172.16.20.1 update-source lo
 neighbor 172.16.20.1 disable-connected-check
 neighbor 172.16.10.1 remote-as 10
 neighbor 172.16.10.1 update-source lo
 neighbor 172.16.10.1 disable-connected-check
 !
 address-family ipv4 unicast
  redistribute ospf
  network 10.10.10.0/28
 exit-address-family
!
!
router ospf
 redistribute connected
!
line vty
```


## Notes converning MPLS

If you want to use MPLS, the following kernel modules must be enabled on the host machine

```
mpls_router
mpls_iptunne
```