#!/bin/bash

set -euo pipefail

# initialize dependencies

helm repo add teleport https://charts.releases.teleport.dev
helm repo add jetstack https://charts.jetstack.io
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
