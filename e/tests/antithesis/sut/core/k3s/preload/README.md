# k3s preload images

Place OCI or Docker image archives in this directory to preload them into the
k3s containerd store. k3s imports files mounted at
`/var/lib/rancher/k3s/agent/images` during startup.
