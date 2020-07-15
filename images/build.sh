#!/bin/sh

current_dir=$(dirname "${0}")

docker build ${current_dir}/router -t topomate/router