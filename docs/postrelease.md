## Post-release Checklist

This checklist is to be run after cutting a release.

- [ ] Create PR to update default Teleport version, hash and Docker image tags in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/4615
- [ ] Create PR to update default Teleport image referenced in docker/teleport-quickstart.yml and docker/teleport-ent-quickstart.yml (if this is a new major/minor release)
  - Example: https://github.com/gravitational/teleport/pull/4655
- [ ] Create PR to update default AMI versions in Makefile and AMIs.md under https://github.com/gravitational/teleport/blob/master/assets/aws
  - Example: https://github.com/gravitational/teleport/pull/4608
- [ ] Create PR to update default Teleport version in Helm charts (https://github.com/gravitational/teleport/blob/master/examples/chart)
- [ ] Update `CURRENT_VERSION_ROOT` and other previous versions in Drone `teleport-docker-cron` job (for a major release)
  - Example: https://github.com/gravitational/teleport/pull/4602