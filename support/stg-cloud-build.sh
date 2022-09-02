#!/bin/bash
build=$1
echo "Build : $build"
sh support/cloud-build.sh -s idemeum-staging-public -r us-west-2 -p stg -b $build
