#!/bin/bash
helm delete --purge nginx-ingress
kubectl delete -f .
