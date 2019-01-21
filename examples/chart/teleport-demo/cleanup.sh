#!/bin/bash
helm delete --purge teleport
kubectl get all | awk '{print $1}' | egrep -v '^$' | grep -v NAME | grep teleport | xargs kubectl delete
