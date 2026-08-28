# kubeconfig creation tools

This directory contains tools for creating `kubeconfig` files for use in
Teleport.

## get-kubeconfig.sh

`get-kubeconfig.sh` creates a `kubeconfig` file for a single cluster.
The resulting `kubeconfig` uses long-lived service account credentials
independent of any provider-specific authentication mechanisms.

This script uses `kubectl` to get the credentials for the new `kubeconfig`.
Run `kubectl config get-contexts` before using this script to make sure you're
targeting the correct cluster.

## merge-kubeconfigs.sh

`merge-kubeconfigs.sh` takes a list of `kubeconfig` paths and merges them into
one file named `merged-kubeconfig`.
