## Post-release Checklist

This checklist is to be run after cutting a release.

### All releases

- [ ] Create PR to update default Teleport version in Teleport docs
  - Example: https://github.com/gravitational/teleport/pull/7033
- [ ] Create PR to update default AMI versions in Makefile and AMIs.md. This can
  be done by manually triggering the
  [GitHub Action](https://github.com/gravitational/teleport/actions/workflows/update-ami-ids.yaml)

### Major releases only

- [ ] Update support matrix in docs FAQ page
- [ ] Update `branchMajorVersion` const in Dronegen `/dronegen/container_images.go`, then run `make dronegen`
  - Example: https://github.com/gravitational/teleport/pull/4602
- [ ] Create PR to update default Teleport image referenced in docker/teleport-quickstart.yml and docker/teleport-ent-quickstart.yml
  - Example: https://github.com/gravitational/teleport/pull/4655
- [ ] Create PR to update default Teleport image referenced in docker/teleport-lab.yml
