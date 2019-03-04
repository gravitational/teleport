#!/usr/bin/env bash
# this is needed to give tiller admin permissions on GKE
kubectl --namespace kube-system create serviceaccount tiller
kubectl create clusterrolebinding tiller --clusterrole cluster-admin --serviceaccount=kube-system:tiller
helm init --service-account tiller --upgrade --force-upgrade --wait