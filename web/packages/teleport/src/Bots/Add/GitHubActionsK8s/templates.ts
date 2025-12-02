export function makeGhaWorkflow(params: {
  roleName: string;
  botName: string;
  repository: string;
  owner: string;
  clusterPublicUrl: string;
}) {
  return GHA_WORKFLOW.replaceAll(':role_name', JSON.stringify(params.roleName))
    .replaceAll(':bot_name', JSON.stringify(params.botName))
    .replaceAll(':repository_owner', JSON.stringify(params.owner))
    .replaceAll(
      ':repository',
      JSON.stringify(`${params.owner}/${params.repository}`)
    )
    .replaceAll(':cluster_public_url', JSON.stringify(params.clusterPublicUrl));
}

export const GHA_WORKFLOW = `# This file contains a GitHub Actions workflow which enrolls with Teleport in
# order to access a Kubernetes cluster using kubectl or other tools compatible
# with kubeconfig, such as Helm.

# Save this file to your GitHub repository in \`.github/workflows\`. You can edit
# the events that trigger your workflow, such as when pushing to a named branch
# or triggering it manually.

# For more information about using GitHub Actions read the getting started
# guide; https://docs.github.com/en/actions/get-started/quickstart

on: workflow_dispatch
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
      uses: actions/checkout@v3

    - name: Fetch Teleport binaries
      uses: teleport-actions/setup@v1
      with:
        version: 19.0.0-dev

    - name: Fetch credentials using Machine ID
      uses: teleport-actions/auth-k8s@v2
      with:
        proxy: example.teleport.sh:443
        token: github-gravitational-teleport
        # Provide the name of your Kubernetes cluster in Teleport
        kubernetes-cluster: my-kubernetes-cluster
        # Enable the submission of anonymous usage telemetry. This helps us
        # shape the future development of \`tbot\`. You can disable this by
        # omitting this.
        anonymous-telemetry: 1

    # Use kubectl or other compatible tools to interact with your Kubernetes
    # cluster.
    - name: List pods
      run: kubectl get pods -A
`;
