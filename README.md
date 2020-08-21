# topomate (WIP)

[![Go Report Card](https://goreportcard.com/badge/github.com/rahveiz/topomate)](https://goreportcard.com/report/github.com/rahveiz/topomate)
![CI](https://github.com/rahveiz/topomate/workflows/CI/badge.svg)

Network Topology Automation using FRRouting containers.
Documentation is available [here](https://topomate.github.io/topomate).


## How does it work ?

Topomate reads a YAML configuration file describing a network topology,
and generates containers, links and configurations from it.

## Trying Topomate

Topomate is still a WIP project with a CLI that is not that user-friendly.
If you want to try it, I highly suggest you to setup a VM. You'll find more
informations on [this page](https://github.com/rahveiz/topomate/wiki/Development-VM).

### Example configuration files

You can find configuration files in the *examples* folder.

## Notes concerning MPLS

If you want to use MPLS, the following kernel modules must be enabled on the host machine

```
mpls_router
mpls_iptunnel
```
