/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

export function makeGhaWorkflow(params: {
  tokenName: string;
  clusterPublicUrl: string;
  clusterName: string;
}) {
  return GHA_WORKFLOW.replaceAll(
    ':token_name',
    JSON.stringify(params.tokenName)
  )
    .replaceAll(':cluster_public_url', JSON.stringify(params.clusterPublicUrl))
    .replaceAll(
      ':kubernetes_cluster',
      JSON.stringify(params.clusterName || 'my-kubernetes-cluster')
    );
}

export const GHA_WORKFLOW = `# This file contains a GitHub Actions workflow which enrolls with Teleport in
# order to access a Kubernetes cluster using kubectl or other tools compatible
# with kubeconfig, such as Helm.

# Save this file to your GitHub repository in \`.github/workflows\`. You can edit
# the events that trigger your workflow, such as when pushing to a named branch
# or triggering it manually.

# Note: the workflow file may need to exist on the "main branch" in your repo
# before it will appear in GitHub Actions.

# For more information about using GitHub Actions read the getting started
# guide; https://docs.github.com/en/actions/get-started/quickstart

on: workflow_dispatch

env:
  TELEPORT_PROXY_ADDR: :cluster_public_url
  TELEPORT_JOIN_TOKEN_NAME: :token_name
  TELEPORT_K8S_CLUSTER_NAME: :kubernetes_cluster

jobs:
  demo:
    permissions:
      # The "id-token: write" permission is required or \`tbot\` will not be able
      # to authenticate with the cluster.
      id-token: write
      contents: read
    name: Teleport Kubernetes Access
    runs-on: ubuntu-latest

    steps:
    - name: Checkout repository
      uses: actions/checkout@v5

    - name: Fetch Teleport binaries
      uses: teleport-actions/setup@v1
      with:
        version: auto
        proxy: \${{ env.TELEPORT_PROXY_ADDR }}

    - name: Fetch credentials using Machine & Workload Identity
      uses: teleport-actions/auth-k8s@v2
      with:
        proxy: \${{ env.TELEPORT_PROXY_ADDR }}
        token: \${{ env.TELEPORT_JOIN_TOKEN_NAME }}
        kubernetes-cluster: \${{ env.TELEPORT_K8S_CLUSTER_NAME }}
        # Enable the submission of anonymous usage telemetry. This helps us
        # shape the future development of \`tbot\`. You can disable this by
        # omitting this.
        anonymous-telemetry: 1

    # Use kubectl or other compatible tools to interact with your Kubernetes
    # cluster.
    - name: List pods
      run: kubectl version
`;
