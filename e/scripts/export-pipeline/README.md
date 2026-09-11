# export-pipeline

## Prerequisites

You must install the [git-filter-repo](https://github.com/gravitational/git-filter-repo) tool which is our fork.

The following script will install it to your path and doesn't require running any scripts:
```
curl -o /usr/local/bin/git-filter-repo https://raw.githubusercontent.com/gravitational/git-filter-repo/da90cb247d3c544e7a76f032ed0457666b24bbb4/git-filter-repo
chmod +x /usr/local/bin/git-filter-repo 
```

Git remotes configured:
  - `origin` - points to `teleport-ent`
  - `teleport-oss` - points to the OSS mirror

## Usage

### Full workflow

```bash
# 1. Sanitize, push staging, and create PR
./export-sanitize.sh --branch master --push-stage --create-pr

# 2. Review and merge the PR in GitHub

# 3. Sync to OSS
./export-sync.sh --branch master
```

### Sanitize, Stage, and Create PR

Prepare a sanitized export branch, push it, and create a promotion PR.

```bash
# Sanitize, push staging, and create PR in one shot
./export-sanitize.sh --branch master --push-stage --create-pr
```

This will:
1. Rebase and sanitize the staging branch
2. Strip each surviving commit message to its title
3. Add an `Export-Source-Commit` trailer to each surviving commit for provenance
4. Force-push to `export-staging/master`
5. Create a PR from `export-staging/master` -> `export/master` (base) with the appropriate template

If an open promotion PR already exists for the staging branch, `--create-pr`
reports the existing PR and exits successfully.

### Merge and Sync

After the promotion PR is reviewed and merged in GitHub, sync the approved export branch to OSS and advance the checkpoint.

The checkpoint advances to the newest private source commit identified by an
`Export-Source-Commit` trailer in the approved export history. If one or more
source commits are removed entirely by the filter, the checkpoint safely lags
until a later surviving commit records newer provenance.

These trailers are part of the exported commit messages and make the
corresponding private source commit IDs visible in the OSS history.
Commit-message bodies are removed from source commits during sanitization. The
promotion merge commit is created afterward, so its title and body must not
contain sensitive information.

```bash
# Sync export/master to OSS and advance checkpoint
./export-sync.sh --branch master
```

### Custom Remotes

If your remotes are named differently, pass `--remote` and `--oss-remote`:

```bash
./export-sanitize.sh --branch master --push-stage --create-pr --remote upstream

./export-sync.sh --branch master --remote upstream --oss-remote public-mirror
```
