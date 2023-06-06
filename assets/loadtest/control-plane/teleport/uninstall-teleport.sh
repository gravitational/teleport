#!/bin/bash

set -euo pipefail

helm --namespace teleport uninstall teleport
