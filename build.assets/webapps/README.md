# webapps jobs

## update-teleport-webassets.sh

This script:

- clones the `teleport` repo
- checks out the provided named branch (after checking that it exists)
- raises a PR against the `teleport` repo to update the submodule commit references

The `webassets` repo is automatically updated for each commit merged to `webapps`.

Run using a command like:

`./update-teleport-webassets.sh -w teleport-v10 -t branch/v10`

| Argument | Description |
| - | - |
| `-w` | `webassets` source branch name to pull `webassets` from (often `master`) |
| `-t` | `teleport` target branch name to raise a PR against (often `master`) |

### Extra notes

You will need to have the `gh` utility installed on your system for the script to work. You can download it from https://github.com/cli/cli/releases/latest

