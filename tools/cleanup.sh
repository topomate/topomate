#!/bin/bash

docker stop $(docker ps | grep topomate | cut -d' ' -f1) 2>/dev/null
docker rm $(docker ps -a | grep topomate | cut -d' ' -f1) 2>/dev/null
docker rmi --force $(docker image ls | grep topomate | tr -s ' ' | cut -d' ' -f3) 2>/dev/null

for bridge in $(sudo ovs-vsctl list-br); do
    sudo ovs-vsctl del-br $bridge
done