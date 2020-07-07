#!/bin/bash

docker stop $(docker ps -q)
docker rm $(docker ps -aq)

for bridge in $(sudo ovs-vsctl list-br); do
    sudo ovs-vsctl del-br $bridge
done