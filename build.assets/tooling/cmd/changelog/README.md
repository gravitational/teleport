# changelog

This script generates a changelog for a release.

## Usage

```
usage: changelog [<flags>]

Flags:
  --[no-]help              Show context-sensitive help (also try --help-long and --help-man).
  --base-branch=BASEBRANCH  The base release branch to generate the changelog for. It will be of the form branch/v* ($BASE_BRANCH)
  --base-tag=BASETAG        The tag/version to generate the changelog from. It will be of the form vXX.Y.Z, e.g. v15.1.1 ($BASE_TAG)
```

It can optionally take two input variables: BASE_BRANCH: The base release
branch to generate the changelog for. It will be of the form "branch/v*".
BASE_TAG: The tag/version to generate the changelog from. It will be of the
form "vXX.Y.Z", e.g. "v15.1.1"


If neither are provided, the values will be automatically determined if
possible:
* The current branch will be used as the base branch if it matches the
  pattern branch/v*
* If the current branch is forked from a base branch, the base branch will be
  used. e.g. if you create release/15.1.1 from branch/v15, the branch/v15
  will be the base branch.
* The base tag will be determined by running "make print-version" from the
  root of the repo.


Enterprise PR changelogs will be listed after the OSS changelogs. You need to
determine if it is suitable to include them. If you do, remove the markdown
link from each changelog when adding the changelog to CHANGELOG.md. These
links won't work for the general public. Keep the links when adding the
changelog to the release PR so that the enterprise PRs will link to the
release PR.

If you reword changelogs, it is best to go to the source PR and change it
there and then regenerate the changelog.


Caveats:
* If you update the "e" ref in your release PR, and you also run `make
  changelog` from the release PR branch, if you have already updated the
  version in the makefile, you will need to run `make changelog
  BASE_TAG=X.Y.Z` as this script will determine the base tag to be the
  current release version not the last released version.


One preferred way of using this script is to run `make changelog` from the
base branch and save it: `make changelog > /tmp/changelog`. If any PRs are
merged to the base branch after you have created your release PR but before
you have merged it, you can see any new entries with:

```
git checkout branch/vNN
diff -u /tmp/changelog $(make changelog)
```
If there are changes, you can update your changelog and rebase your branch:
```
git pull on branch/vNN
make changelog > /tmp/changelog
git checkout release/XX.Y.Z
git rebase branch/vNN
<include /tmp/changelog in CHANGELOG.md>
git add CHANGELOG.md && git commit --amend --no-edit && git push -f
```

Ensure you update the PR body with the new changelog, doable on the command
line with `gh`:
```
gh pr edit --body-file /tmp/changelog
```