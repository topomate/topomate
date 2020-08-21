Specification file
==================

This page describes the different elements you can configure in a specification
file.

General settings
----------------

name : string
  Name of the subdirectory where configuration files will be generated.
  It defaults to "generated" if the key is absent.


Autonomous Systems
------------------

autonomous_systems : array
  Describe the different AS of the topology.

  asn : int
    AS number.

  routers: 
    Number of routers in the AS.
    .. note:: Generated routers will be referred by an index (starting from 1) in multiple configuration elements

  loopback_start : string
    Loopback address of the first router.
    Following routers will use contiguous addresses.

  igp : string
    IGP used in the AS.
    
    Recognized values: IS-IS, OSPF


  prefix : string
    Prefix used by the AS. It will also be used to auto-assign IP addresses to the
    different interfaces.
  
  subnet_length : int
    Subnet length that will be used to auto-assign IP addresses for the different
    interfaces.

  mpls : bool
    Enable MPLS in the AS. Defaults to **false**.


  links
    Internal links specification.

      kind : string
        Kind of interconnexion to use.

        Supported values: full-mesh, ring, manual.

      speed : int
        Bandwidth that will be used for the links when using a non-manual kind
        of links, and for manual presets.

      cost : int
        Cost that will be used for the links when using a non-manual kind
        of links, and for manual presets. If the **speed** key is set and
        there are no cost, the cost defaults to the speed if using OSPF.

      preset : string
        Preset that will be applied to a manual configuration (if set).

        Supported values: full-mesh, ring.


      file : string
        Path (relative to the configuration file, or absolute) a file containing
        the links specification.
        Only valid if using a manual configuration.
        Uses the following format : 
        
        .. code-block::

          <router_id_A> <router_id_b> [<speed> [<igp_cost_a> <igp_cost_b>]]

        If only one IGP cost is set, it will be applied to both interfaces.
        If you want to set it to only one interface, use `*` on the other.

        **Example**

        .. code-block::

          1 2 1000 63 32
          4 3 1000 48
          2 3 1000 ** 10
          2 5 1000
          3 5 1000


    


Routing protocols options
^^^^^^^^^^^^^^^^^^^^^^^^^

There are optional keys available to further customize the routing protocols.

bgp 
  BGP settings
  
  ibgp
    iBGP settings

      disabled : bool
        If set, BGP configuration won't be generated

      redistribute_igp : bool
        If set, an attribute to redistribute the routes from the IGP in use will
        be added to the BGP process

      manual : bool
        If set, the iBGP sessions won't be use the automatic pattern
        (1 session / neighbor directly connected)
      
      route_reflectors : array
        Route-Reflectors settings

        router : int 
          ID of the router that will be a route-reflector
        clients : array (int)
          IDs of the RR clients
        
      cliques : array (array (int))
        IDs of routers that have a full-mesh (clique) iBGP neighborhood

isis
  IS-IS settings

  level-1 : array (int)
    IDs of L1 routers

  level-2 : array (int)
    IDs of L2 routers

  level-1-2 : array (int)
    IDs of L1-L2 routers

  areas : map (int, array (int))
    Describes the different areas (area as a key, array of router IDs as value)

    **Example**
    
    .. code-block:: yaml
    
      isis:
        level-1: [1, 4]
        level-2: [5]
        level-1-2: [2, 3]
        areas:
          1: [1, 2]
          2: [3, 4]
          3: [5]
   

ospf
  OSPF settings

  .. warning::
    This is still a work in progress. The generated configuration will not
    work out of the box.

  networks : array
    Describes the diffrent networks

    prefix : string
      Network prefix
    
    area : int
      Area for the prefix

    routers : array (int)
      IDs of routers with this network configured

  
  stubs : array (int)
    Areas that are stubs

  **Example**

  .. code-block:: yaml
  
    ospf:
      networks:
        - prefix: 192.168.1.1/24
          area: 0
          routers: [2, 3, 5]
        - prefix: 192.168.1.1/24
          area: 1
          routers: [1]
        - prefix: 192.168.1.1/24
          area: 2
          routers: [4]
      stubs: [1, 4]


L3VPN
^^^^^

Topomate allows you to define BGP/MPLS VPNs inside an AS. Please make sure
that MPLS is enabled on the AS when using VPNs.

vpn : array
  VPN settings

  vrf : string
    Name of the VRF that will be used.

  hub_and_spoke : bool
    If true, the VPN will be configured using an Hub-and-Spoke topology,
    using a secondary VRF on the hub.

  customers : array
    Customers specification

    hostname : string
      Hostname of the CE router.

    remote_subnet : string
      Subnet managed by the CE.

    loopback : string
      Loopback of the CE router. Must belong to the remote subnet.

    subnet : string
      Subnet used for the link between the PE and the CE.

    downstream_subnet : string
      Subnet used for the secondary link between the PE and the CE. Only used
      on a hub.

    parent : int
      ID of the parent router (PE) in the AS.
    
    hub : bool
      If set, consider that the current CE is a hub.




External links
--------------

external_links_file
  Path to the file describing the external links of the topology.

  The file must be of the following format:

  .. code-block::

     <ASN1>.<router_number> <ASN2>.<router_number> <relation> <speed>

  Supported relations:
    * **p2c**: provider-customer
    * **c2p**: customer-provider
    * **p2p**: peer-to-peer

  **Example**

    .. code-block::

       1.1 2.1 p2c 10000
       2.1 3.1 p2p 10000
       2.1 4.1 p2c 10000


IXP
---

Topomate allows you to simulate **I**\ nternet **E**\ xchange **P**\ oints.

ixps : array
  Describe the IXPs

  asn : int
    ASN of the IXP

  prefix : string
    Network prefix used by the routers participating in the IXP (usually local,
    won't be advertised)

  loopback : string
    Loopback address of the route-server
  
  peers : array (string)
    Routers connected to the IXP, in the following format :

    .. code-block::

     <ASN>.<router_ID> [<speed>]