Quick Start Guide
=================

Notes
#####

Topomate does not require to be run as root, but you will need to have sudo
rights (as many commands will be called with sudo internally).

You can also use the commands with `sudo`, the application will detect your
own home directory automatically.

Basic Configuration
###################

We'll define a simple configuration with 2 AS (AS1 and AS2).

First, we'll describe the specification of AS1. Let's say that AS1 is made of
4 routers using a full-mesh topology, and that it administrates the
`10.1.1.1/24` prefix. `/30` networks will be used to interconnect the different
routers. We also specify a name that will be the directory where the
configuration files will be put (defaults to "generated" if the key is not
present).

.. code-block:: yaml
  :linenos:

   name: "quickstart_topology"
   
   autonomous_systems:
    - asn: 1
      routers: 4
      prefix: '10.1.1.0/24'
      links:
        kind: 'full-mesh'
        subnet_length: 30

Now, we will specify the loopback addresses of the routers. The configuration
uses the `loopback_start` element, that will be the loopback address of the
first router of the AS. The other routers will use contiguous addresses (i.e: R2
will have the `172.16.10.2/32` address).

We will also specify which IGP to use, here it is OSPF. We want our BGP
configuration to redistribute prefixes learnt by OSPF so we set the
`redistribute_igp` element to `true`.


.. code-block:: yaml
  :linenos:

  autonomous_systems:
    - asn: 1
      routers: 4
      prefix: '10.1.1.0/24'
      loopback_start: '172.16.10.1/32'
      igp: OSPF
      redistribute_igp: true
      links:
        kind: 'full-mesh'
        subnet_length: 30
      

AS1 is now specified. We can do the same process for AS2.

.. code-block:: yaml
  :linenos:

  autonomous_systems:
    - asn: 1
      routers: 4
      prefix: '10.1.1.0/24'
      loopback_start: '172.16.10.1/32'
      igp: OSPF
      redistribute_igp: true
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


We will then describe the external links (between AS1 and AS2). Here, we will
set AS1 as a provider and AS2 as a customer.

.. code-block:: yaml
  :linenos:

  external_links:
    - from:
        asn: 1
        router_id: 1
      to:
        asn: 2
        router_id: 1
      rel: 'p2c'

Here, the relation is provider-customer (`p2c`), so the router specified in
`from` will be the provider. If you want to reverse the relationship, you can
either invert the `from` and `to` specification, or you can set the relation to
`c2p`.


Starting the network topology
#############################

To start a network topology, simply run the following command:

.. code-block:: bash

   topomate start /path/to/config/file.yaml

By default, the configurations will be generated in your home directory using
the following path format: `~/topomate/<name>/conf_<ASN>_<hostname>`
(i.e. `~/quickstart_topology/conf_1_R3`).

Topomate will also generate a JSON file containing data relative to the links.

If you have made modifications on the configuration files that you want to keep,
you can use the `--no-generate` flag to start the topology without overriding
the current configuration files.



Stopping the topology
#####################

To stop a topology, use the `stop` command:

.. code-block:: bash

   topomate stop /path/to/config/file.yaml


Only generate configuration files
#################################

If you only want the FRR configuration files, you can use the `generate` command.

.. code-block:: bash

   topomate generate /path/to/config/file.yaml


Restart a router
###################

A router can be restarted using the `restart` command. It will synchronise the
configuration file and links will be re-applied.

.. code-block:: bash

   topomate restart <container_name>

.. warning::
  Do not restart a router using the `docker restart` command, as the
  configuration won't be synchronized, and most importantly the links won't be
  reapplied as Topomate does not use the native Docker networking solution.

Interacting with FRRouting
***********************

Once the containers are started, you can use the common `docker` commands
to interract with FRR.

You can directly launch the `vtysh` using

.. code-block:: bash
   docker exec -it <container_name> vtysh

or you can use a traditionnal shell (`ash`, `bash`)

.. code-block:: bash
   docker exec -it <container_name> ash

and launch `vtysh` from this shell.


Using other tools
*****************

From a traditionnal shell, there are some tools already installed :

* `tcpdump`
* `tcptraceroute`
* `iperf3`

.. note::
  Use the `docker cp` command to copy files such as `.pcap` files to
  your host (useful if you want to analyze a capture with Wireshark).