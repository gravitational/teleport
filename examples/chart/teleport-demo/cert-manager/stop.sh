#!/bin/bash
helm delete --purge cert-manager
kubectl delete -f .
