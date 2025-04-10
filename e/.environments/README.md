# Environments

The environment values in this directory hierarchy are loaded by env-loader:
https://github.com/gravitational/shared-workflows/tree/main/tools/env-loader

TODO: Describe stage and prod environments

## build

TODO: Describe build environment and variables

## publish

TODO: Describe publish environment and variables

## oci-rebuild environment

The `oci-rebuild` environment is a hybrid environment for running daily rebuilds
of the Teleport distroless OCI images. The version of Teleport built is the same
as the latest tag for each release, but the base dependencies may have changed.
For example, the base distroless image may have upgraded a library in the image
due to a security issue.

This rebuild needs to be done in a hybrid fashion as the OCI images in the pending
registry are immutable so cannot be updated. However, public ECR images are not
immutable, so we rebuild in the build environment and push straight to
`public.ecr.aws` signed with the publishing keys - the publish environment. This
is done only for versions of Teleport that have already been released. It is not
possible to create new release versions this way.

## Vars

The vars in the `oci-image-build.yml` workflow are the only ones needed in this
environment. The `<baseenv>` label below in environment names is either `stage`
or `prod` as this image rebuilding can be run in either stage or prod.

### Vars for input (build)
* `ECR_READ_ONLY_ROLE`: The AWS role needed to log into public ECR read-only to
  avoid rate limits when pulling any base images. Same as `<baseenv>/build`
* `ARTIFACT_DOWNLOAD_AWS_ROLE`: The AWS role needed to download built artifacts
  (binaries) from the pending bucket to put into the final OCI image. Same as
  `<baseenv>/build`.
* `ARTIFACT_SOURCE_BUCKET`: The AWS S3 bucket containing the artifacts to
  download. Same as `<baseenv>/build`.

### Vars for output (publish)
* `ECR_PENDING_REPO_BASE`: The base part of the image in the target
  registry/repository. Same as `OCI_PUBLIC_REPO_BASE` in `<baseenv>`
  `common.yaml`.
* `OCI_ECR_AWS_ROLE`: The AWS role needed to push to the registry/repo above.
  Same as `ECR_OCI_PUSH_ROLE_ARN` in `<baseenv>/publish`.
* `OCI_PENDING_SIGNING_KEY_ARN`: The key ARN to sign the generate images. Same
  as `OCI_PUBLIC_SIGNING_KEY_ARN` in `<baseenv>/publish`.
