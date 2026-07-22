## Preflight Checklist

This checklist is to be run prior to cutting the release branch.

- [ ] Update VERSION in Makefile to next dev tag
- [ ] Send out a call to `#teleport-dev` on slack for people to search code for
      DELETE IN and REMOVE IN comments and clean up if appropriate
- [ ] Update the CI buildbox image
  - [ ] Update the `BUILDBOX_VERSION` in `build.assets/images.mk`. Commit and
    merge.
  - [ ] Update `e/.github/workflows/build-buildboxes-cron.yaml` to uncomment
    final pre-release job and ensure it has the correct branch names (two
    places). Commit and merge.
  - [ ] After the `BUILDBOX_VERSION` update in the `build.assets/images.mk` has
    merged to master, build the buildboxes for the next `BUILDBOX_VERSION`. The
    first run will build the "assets" buildbox and the other builds will likely
    fail. Run again after the first has finished to build the others that use
    the "assets" buildbox:

        today=$(LOCALE=C TZ=UTC date +%A)
        gh workflow run --repo gravitational/teleport.e --field assets-day="${today}" build-buildboxes.yaml
        gh workflow run --repo gravitational/teleport.e build-buildboxes.yaml

  - [ ] Update all the CI jobs in `.github/workflows` that use `teleport-buildbox:teleportX`
    and `teleport-buildbox-centos7:teleportX-amd64` and bump X to the new version.
  - [ ] Update all the CI jobs in `e/.github/workflows` that use `teleport-buildbox:teleportX`
    and bump X to the new version.
