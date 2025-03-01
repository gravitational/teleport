## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

Our GitHub Actions workflows will create two PRs when a release is published:

1. A PR against the release branch that updates the default version in our docs.
2. A PR that updates the AWS AMI IDs for the new release. (This job only runs
   for releases on the latest release branch)

The AWS AMI ID PR can be merged right away.

### Major releases only

- [ ] Update support matrix in docs FAQ page
  - Example: https://github.com/gravitational/teleport/pull/50345
- [ ] Update the list of OCI images to monitor and rebuild nightly in
  [`monitor-teleport-oci-distroless.yml` on `master`](https://github.com/gravitational/teleport.e/blob/master/.github/workflows/monitor-teleport-oci-distroless.yml) and
  [`rebuild-teleport-oci-distroless-cron.yml` on `master`](https://github.com/gravitational/teleport.e/blob/master/.github/workflows/rebuild-teleport-oci-distroless-cron.yml)
- [ ] Update `e/.github/workflows/build-buildboxes-cron.yaml` to bump the
  branches on each job (two per job) and to comment out the final job that only
  exists for the pre-release, and bump the versions for the next release.
- [ ] Update `e/.github/workflows/nightly-releases.yaml` on master to bump the 
  branches in the strategy matrix, comment out the final branch that only 
  exists for the pre-release, and bump the version for the next release.
