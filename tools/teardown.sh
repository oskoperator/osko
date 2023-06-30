#!/bin/sh
set -o errexit

teardown() {
    docker rm -f kind-registry
    kind delete cluster
}

read -e -p "Would you like to delete the running local registry \`kind-registry\` and run \`kind delete cluster\`? [Y/y] " choice
[[ "$choice" == [Yy]* ]] && teardown || echo "Aborted"
