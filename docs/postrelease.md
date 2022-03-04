## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

- [ ] Create PR to update default Teleport version in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/7033
- [ ] Create PR to update default AMI versions in Makefile and AMIs.md under https://github.com/gravitational/teleport/blob/master/assets/aws
  - Example command: `TELEPORT_VERSION=6.2.0 make -C assets/aws create-update-pr`
- [ ] Release [Teleport Plugins](https://github.com/gravitational/teleport-plugins)
  - `make update-version VERSION=X.Y.Z`
    - This will update the `teleport/api` version and may lead to linting errors which can be easily resolved.
    - You may need to update `terraform/gen_teleport.yaml` if there are errors. This can be caused by resource
    upgrades (e.g. `RoleV4` -> `RoleV5`). Once the necessary changes are made, run `make -C terraform gen-schema`.
  - Create a new release PR and merge it to master.
  - Pull master locally and run `make update-tag VERSION=X.Y.Z` to push the new tags.

### Major releases only

- [ ] Update support matrix in docs FAQ page
- [ ] Update `CURRENT_VERSION_ROOT` and other previous versions in Drone `teleport-docker-cron` job
  - Example: https://github.com/gravitational/teleport/pull/4602
- [ ] Create PR to update default Teleport image referenced in docker/teleport-quickstart.yml and docker/teleport-ent-quickstart.yml
  - Example: https://github.com/gravitational/teleport/pull/4655
- [ ] Create PR to update default Teleport image referenced in docker/teleport-lab.yml
- [ ] Update the CI buildbox image
  - [ ] Update the `BUILDBOX_VERSION` in `build.assets/Makefile`
  - [ ] Run `make dronegen` and ensure _all_ buildbox references in the resulting yaml refer to the new image
  - [ ] Commit and merge. Drone should build new buildbox images and push to `quay.io`
  - [ ] Once the new images are confirmed in `quay.io`, update the build yaml files under `.cloudbuild` to refer to the new image
