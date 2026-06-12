## Preflight Checklist

This checklist is to be run prior to cutting the release branch.

- [ ] Bump Web UI dependencies
- [ ] Make a new docs/VERSION folder
- [ ] Update VERSION in Makefile to next dev tag
- [ ] Update TELEPORT_VERSION in assets/aws/Makefile
- [ ] Update `base-ref` to point to the new release branch in `.github/workflows/dependency-review.yaml`
- [ ] Update mentions of the version in examples/ and README.md
- [ ] Search code for DELETE IN and REMOVE IN comments and clean up if appropriate
- [ ] Update docs/faq.mdx "Which version of Teleport is supported?" section with release date and support info
- [ ] Update the CI buildbox image
  - [ ] Update the `BUILDBOX_VERSION` in `build.assets/images.mk`. Commit and merge.
  - [ ] Update `e/.github/workflows/build-buildboxes-cron.yaml` to uncomment final pre-release
    job and ensure it has the correct branch names (two places). Commit and merge.
