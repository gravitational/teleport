#!/bin/bash

# Copyright 2018 Gravitational, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script receives a new set of credentials for auth server
# that are based on TLS client certificates. This is better than using
# cloud provider specific auth "plugins" because it works on any cluster
# and does not require any extra binaries. It produces kubeconfig to build/kubeconfig

# This script can be used to retreive x509 certificate from a GKE cluster
# which can be used for accessing the CSR API with Teleport
#
# For more information see:
# https://gravitational.com/blog/kubectl-gke/
#
# Produce CSR request first

set -eu -o pipefail

# Set OS specific values.
if [[ "$OSTYPE" == "linux-gnu" ]]; then
    REQUEST_ID=$(uuid)
    BASE64_DECODE_FLAG="-d"
    BASE64_WRAP_FLAG="-w 0"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    REQUEST_ID=$(uuidgen)
    BASE64_DECODE_FLAG="-D"
    BASE64_WRAP_FLAG=""
else
    echo "Unknown OS ${OSTYPE}"
    exit 1
fi

mkdir -p build
pushd build
cat > csr <<EOF
{
  "hosts": [
  ],
  "CN": "teleport",
  "names": [{
        "O": "system:masters"
    }],
  "key": {
    "algo": "ecdsa",
    "size": 256
  }
}
EOF

cat csr | cfssl genkey - | cfssljson -bare server


# Create Kubernetes CSR
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${REQUEST_ID}
spec:
  groups:
  - system:authenticated
  request: $(cat server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - client auth
EOF
kubectl certificate approve ${REQUEST_ID}

kubectl get csr ${REQUEST_ID} -o jsonpath='{.status.certificate}' \
    | base64 ${BASE64_DECODE_FLAG} > server.crt

kubectl -n kube-system exec $(kubectl get pods -n kube-system -l k8s-app=kube-dns  -o jsonpath='{.items[0].metadata.name}') -c kubedns -- /bin/cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt > ca.crt

# Extract cluster IP from the current context
CURRENT_CONTEXT=$(kubectl config current-context)
CURRENT_CLUSTER=$(kubectl config view -o jsonpath="{.contexts[?(@.name == \"${CURRENT_CONTEXT}\"})].context.cluster}")
CURRENT_CLUSTER_ADDR=$(kubectl config view -o jsonpath="{.clusters[?(@.name == \"${CURRENT_CLUSTER}\"})].cluster.server}")

cat > kubeconfig <<EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat ca.crt | base64 ${BASE64_WRAP_FLAG})
    server: ${CURRENT_CLUSTER_ADDR}
  name: k8s
contexts:
- context:
    cluster: k8s
    user: teleport
  name: k8s
current-context: k8s
kind: Config
preferences: {}
users:
- name: teleport
  user:
    client-certificate-data: $(cat server.crt | base64 ${BASE64_WRAP_FLAG})
    client-key-data: $(cat server-key.pem | base64 ${BASE64_WRAP_FLAG})
EOF

popd
