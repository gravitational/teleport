#!/bin/bash
kubectl apply -f psp-role-privileged.yaml
kubectl apply -f cert-manager-psp-rolebinding.yaml
if [[ "$1" == "prod" ]]; then
    echo "Starting cert-manager [prod]"
    helm install --name cert-manager --namespace kube-system --set rbac.create=true --set ingressShim.defaultIssuerName=letsencrypt-prod --set ingressShim.defaultIssuerKind=ClusterIssuer stable/cert-manager
else
    echo "Starting cert-manager [staging]"
    helm install --name cert-manager --namespace kube-system --set rbac.create=true --set ingressShim.defaultIssuerName=letsencrypt-staging --set ingressShim.defaultIssuerKind=ClusterIssuer stable/cert-manager
fi
kubectl apply -f .
