## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

Our GitHub Actions workflows will create two PRs when a release is published:

1. A PR against the release branch that updates the default version in our docs.
2. A PR that updates the AWS AMI IDs for the new release. (This job only runs
   for releases on the latest release branch)

The AWS AMI ID PR can be merged right away.

The docs version PR should be merged after the `gravitational/teleport-plugins` release
is published, since the PR will include an update to the plugins version as well.

### Major releases only

- [ ] Update support matrix in docs FAQ page
  - Example: https://github.com/gravitational/teleport/pull/4602
- [ ] Update the list of OCI images to monitor and rebuild nightly in
  [`monitor-teleport-oci-distroless.yml` on `master`](https://github.com/gravitational/teleport.e/blob/master/.github/workflows/monitor-teleport-oci-distroless.yml) and
  [`rebuild-teleport-oci-distroless-cron.yml` on `master`](https://github.com/gravitational/teleport.e/blob/master/.github/workflows/rebuild-teleport-oci-distroless-cron.yml)
