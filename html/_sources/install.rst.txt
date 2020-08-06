Installation
============

Requirements
############

To use ``topomate``, you will have to install some tools.

Docker
------

Docker is the container engine used to emulate routers using FRRouting.

.. code-block:: bash

   sudo apt-get install docker.io

Open vSwitch
------------

Open vSwitch is used to interconnect containers together. OVS is preferred to
the default network engine provided with Docker as it allows more customization
of the links (i.e: adding a delay or a throughput limit).

.. code-block:: bash
   
   sudo apt-get install openvswitch-switch


Notes concerning MPLS
#####################

If you want to use MPLS in your topologies, you will need to enable some kernel
modules on the host.
You can enable them using ``modprobe``:

.. code-block:: bash

   sudo modprobe -a mpls_router
   sudo modprobe -a mpls_iptunnel

To make these changes persistent, edit ``/etc/modules-load.d/modules.conf``
and add the following lines:

.. code-block:: bash

   mpls_router
   mpls_iptunnel