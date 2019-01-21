#!/bin/bash
kubectl apply -f .
helm install --name nginx-ingress --set rbac.create=true stable/nginx-ingress
