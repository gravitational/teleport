## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

- [ ] Create PR to update default Teleport version in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/7033
- [ ] Create PR to update default AMI versions in Makefile and AMIs.md under https://github.com/gravitational/teleport/blob/master/assets/aws
  - Example command: `TELEPORT_VERSION=6.2.0 make -C assets/aws create-update-pr`

### Major releases only

- [ ] Update support matrix in docs FAQ page
- [ ] Update `CURRENT_VERSION_ROOT` and other previous versions in Drone `teleport-docker-cron` job
  - Example: https://github.com/gravitational/teleport/pull/4602
- [ ] Create PR to update default Teleport image referenced in docker/teleport-quickstart.yml and docker/teleport-ent-quickstart.yml
  - Example: https://github.com/gravitational/teleport/pull/4655
- [ ] Create PR to update default Teleport image referenced in docker/teleport-lab.yml
