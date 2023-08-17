## Debugging tips for the kube-agent-updater

### Running locally the updater against a remote Kubernetes cluster

Running locally let you attach a debugger while still working against a real
cluster. This can be used to reproduce most complex issues and troubleshoot
specific cases.

- Validate your current context works
  ```shell
  kubectl cluster-info
  ```
- Open a proxy to the api-server, then let the shell open and running
  ```shell
  kubectl proxy
  ```
- open a new terminal, create a new temporary directory and create your new kubeconfig
  ```shell
  export KUBECONFIG="$(mktemp)"
  kubectl config set-credentials myself --username=foo
  kubectl config set-cluster local-server --server=http://localhost:8001
  kubectl config set-context default-context --cluster=local-server --user=myself
  kubectl config use-context default-context
  echo "$KUBECONFIG"
  ```
- run the controller with the `KUBECONFIG` environment variable set
