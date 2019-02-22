#!/bin/bash
docker build -t gcr.io/kubeadm-167321/namespace-cleaner:latest .
docker push gcr.io/kubeadm-167321/namespace-cleaner:latest