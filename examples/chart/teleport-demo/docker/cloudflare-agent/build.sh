#!/bin/bash
docker build -t gcr.io/kubeadm-167321/cloudflare-agent:latest .
docker push gcr.io/kubeadm-167321/cloudflare-agent:latest