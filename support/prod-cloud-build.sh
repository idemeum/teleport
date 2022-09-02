#!/bin/bash
build=$1
echo "Build : $build"
sh support/cloud-build.sh -s idemeum-prod-public -r us-west-2 -p prod -b $build
