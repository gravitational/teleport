## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

- [ ] Create PR to update default Teleport version in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/4615
- [ ] Create PR to update default AMI versions in Makefile and AMIs.md under https://github.com/gravitational/teleport/blob/master/assets/aws
  - Example: https://github.com/gravitational/teleport/pull/4608
- [ ] Create PR to update app version and Helm chart version (https://github.com/gravitational/teleport/blob/master/examples/chart)
- [ ] Verify Blog post links point to the correct articles and don't 404.
- [ ] Verify that any version-specific information in the README.md is updated.

### Major/minor releases only

- [ ] Update support matrix in docs FAQ page
- [ ] Create PR to update default Docker image version in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/4615
- [ ] Create PR to update default Teleport image referenced in docker/teleport-quickstart.yml and docker/teleport-ent-quickstart.yml
  - Example: https://github.com/gravitational/teleport/pull/4655
- [ ] Update `CURRENT_VERSION_ROOT` and other previous versions in Drone `teleport-docker-cron` job
  - Example: https://github.com/gravitational/teleport/pull/4602