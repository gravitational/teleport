## Preflight Checklist

This checklist is to be run prior to cutting the release branch.

- [ ] Bump Golang dependencies
- [ ] Review forked dependencies for upstream security patches
- [ ] Bump Web UI dependencies
- [ ] Make a new docs/VERSION folder
- [ ] Update VERSION in Makefile to next dev tag
- [ ] Update TELEPORT_VERSION in assets/aws/Makefile
- [ ] Update mentions of the version in examples/ and README.md
- [ ] Search code for DELETE IN and REMOVE IN comments and clean up if appropriate
- [ ] Update docs/faq.mdx "Which version of Teleport is supported?" section with release date and support info
- [ ] Update the CI buildbox image
  - [ ] Update the `BUILDBOX_VERSION` in `build.assets/Makefile`
  - [ ] Run `make dronegen` and ensure _all_ buildbox references in the resulting yaml refer to the new image
  - [ ] Commit and merge. Drone should build new buildbox images and push to `quay.io`
  - [ ] Once the new images are confirmed in `quay.io`, update the build yaml files under `.cloudbuild` to refer to the new image