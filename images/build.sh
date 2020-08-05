#!/bin/sh

current_dir=$(dirname "${0}")

docker build ${current_dir}/router -t topomate/router
docker build ${current_dir}/route-server-frr -t topomate/route-server
docker build ${current_dir}/rtr -t topomate/rtr