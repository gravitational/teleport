# OS package repo tool

This tool handles the publishing of OS packages (debs, RPMs) to OS package repos
(APT, YUM).

## FAQ

### What do I do when jobs are failing because the distributed lock is unavailable?

The distributed lock is implemented via [Kubernetes leases with leadership
election](https://kubernetes.io/docs/concepts/architecture/leases/). If a job
crashes (i.e. sigkill), loses connection to the k8s API, or the machine has a
kernel panic, future jobs will be unable to receive the leadership lock.

If this happens, you need to:
1. Log into the Kubernetes cluster that contains the leader lease

   ```console
   $ tsh login
   ...
   $ tsh kube login gha-eks-prod # Or gha-eks-dev
   Logged into Kubernetes cluster "gha-eks-prod". Try 'kubectl version' to test the connection.
   ```
2. Determine what job currently holds the lock.

    ```console
    # The HOLDER of the lease contains the hostname of the runner that has acquired it 
    $ kubectl -n gha-runners get leases
    NAME                                                HOLDER                                                                                                 AGE
    actions-runner-controller                           actions-runner-controller-6844bdb66d-svjxx_aaea8a90-478c-4fdf-830f-f6da4da9f096                        662d
    gha-runner-scale-set-controller                     gha-runner-scale-set-controller-844857b6d4-lgql7_cd670737-f62d-414f-a973-4c2b17cdd242                  396d
    gha-runner-scale-set-controller-gha-rs-controller   gha-runner-scale-set-controller-gha-rs-controller-959489c98zsgb_15936cfb-bf43-4968-ba89-153d003ed6c9   430d
    oprt-apt-apt_release_teleport_dev                   apt-prod-runner-7hjrn-826dt-65e38b20-6dd5-4e65-a7f8-eb32d5dd8cfb                                       2d
    $ POD_NAME="apt-prod-runner-7hjrn"
    ```
3. Determine **why** the lock has not been released. This is critically
   important to prevent data corruption.
4. Terminate the job that currently holds the lock, if it is still running. You
   will need to ensure that the job is not in a critical section, e.g. uploading
   assets to S3.
   1. Terminate the job on GitHub
   2. Wait a few minutes, and see if the underlying runner pod terminates
      gracefully. If not, run `kubectl delete -n gha-runners delete pod "${POD_NAME}"`.
5. Delete the lease:

   ```console
   $ kubectl -n gha-runners get leases
   NAME                                                HOLDER                                                                                                 AGE
   actions-runner-controller                           actions-runner-controller-6844bdb66d-svjxx_aaea8a90-478c-4fdf-830f-f6da4da9f096                        662d
   gha-runner-scale-set-controller                     gha-runner-scale-set-controller-844857b6d4-lgql7_cd670737-f62d-414f-a973-4c2b17cdd242                  396d
   gha-runner-scale-set-controller-gha-rs-controller   gha-runner-scale-set-controller-gha-rs-controller-959489c98zsgb_15936cfb-bf43-4968-ba89-153d003ed6c9   430d
   oprt-apt-apt_release_teleport_dev                   apt-prod-runner-7hjrn-826dt-65e38b20-6dd5-4e65-a7f8-eb32d5dd8cfb                                       2d
   # Future jobs should recreate this automatically. Make sure that the lease name matches the hung lease.
   $ kubectl -n gha-runners delete lease oprt-apt-apt_release_teleport_dev   
   lease "oprt-apt-apt_release_teleport_dev" deleted
   ```
6. Restart the release.
