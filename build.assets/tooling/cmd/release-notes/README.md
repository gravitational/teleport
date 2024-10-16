# release-notes

A release notes generator for Teleport releases.

## Usage

```shell
usage: release-notes <version> <changelog>


Flags:
  --[no-]help  Show context-sensitive help (also try --help-long and --help-man).

Args:
  <version>    Version to be released
  <changelog>  Path to CHANGELOG.md
```

This script is expected to be run along side the `gh` CLI to create a release.

```shell
release-notes $VERSION CHANGELOG.md | gh release create \
    -t "Teleport $VERSION" \
    --latest=false \
    --target=$BRANCH \
    --verify-tag \
    -F - \

```