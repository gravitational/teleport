#!/bin/bash
helm init --wait
kubectl create secret generic teleport-license --from-file=license/license-enterprise.pem --dry-run -o yaml | kubectl apply -f -
