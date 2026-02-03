# Skill: Finding RFD Release Versions

Determine which Teleport release version an RFD was first included in.

## Quick Process

### 1. Find the RFD Author
```bash
gh pr list --search "RFD 0223" --state merged --json author
# Use the GitHub username, not email
```

### 2. Check Author's Implementation Timeline
```bash
gh pr list --author "username" --state merged \
  --search "created:YYYY-MM-DD..YYYY-MM-DD" --limit 20 \
  --json number,title,mergedAt,baseRefName | \
  jq -r '.[] | "\(.mergedAt | split("T")[0]) [\(.baseRefName)] #\(.number) \(.title)"'
```

Look for:
- RFD PR → protobufs → backend → UI → docs progression
- **Backport PR** with `[v18]` prefix targeting `branch/v18`

### 3. Find the Release

**Path A: Backport PR exists** (most common for v18.X.X releases)
```bash
# Get backport merge commit
gh pr view <backport-pr> --json mergeCommit

# Find first release tag
git tag --contains <commit-sha> | grep -E "^v18\.[0-9]+\.[0-9]+$" | sort -V | head -1
```

**Path B: No backport PR** (for new major/minor releases like v18.0.0)
```bash
# Feature on master gets cut into new release branch
# Check if commit is in the release
git merge-base --is-ancestor <commit-sha> v18.0.0 && echo "YES" || echo "NO"
```

**Exception**: Features merged to `master` around the time of a new major/minor release (like v18.0.0) may not need backports - they get cut directly into the new release branch.

### 4. Verify in Release Notes (Critical!)
```bash
gh release view v18.3.0 --json body | jq -r '.body' | grep -i "feature-name" -A 3
```

**Major features** get dedicated sections (`### Feature Name`) in **minor releases** (vX.Y.0).
**Small features** get bullet points in patch releases (vX.Y.Z).

## Edge Cases

### Master-Only (Not Released Yet)
- No `[v18]` backport found
- `git tag --contains` returns no v18 tags
- Merged < 2 months ago
- **Action**: Mark as unreleased, expected in v19.0.0

### No Release Notes Mention
- Commit is in release (verify with `git merge-base --is-ancestor`)
- But no release notes mention
- **Common for**: Infrastructure features, security enhancements, opt-in features
- **Verify**: Check test plans in `.github/ISSUE_TEMPLATE/testplan.md`

### Patch vs Minor Release
- If first tag is a patch release (v18.2.10), check if the next minor release (v18.3.0) has a "big shout out"
- **Use the minor release** with dedicated section for major features

### Incremental Rollouts
- Features rolled out across multiple patches without single announcement
- **Document as "v18.x"** instead of specific version
- Examples: Bot Details (RFD 0217), MWI Terraform Provider (RFD 0215)

### Dual Major Version Releases (Rare)
- Some features get backported to BOTH v17 and v18 (customer-driven)
- Check release notes for BOTH versions
- Example: VNet SSH in v17.6.0 and v18.1.0
- **Document the FIRST release** and note both in RFD frontmatter: `state: implemented (v17.6.0, v18.1.0)`

### Git Tags Without GitHub Releases
- Some git tags exist but have no GitHub release (e.g., v17.5.0)
- If `gh release view vX.Y.0` fails, try next patch: `gh release view vX.Y.1`
- The feature is still "in" that version, just announced in next patch

## Quick Example

```bash
# RFD 0223 - Kubernetes Health Checks
gh pr list --search "RFD 223" --state merged --json author  # → rana
gh pr list --author "rana" --search "created:2025-09-01..2025-11-01"  # → #60492 [v18]
gh pr view 60492 --json mergeCommit  # → 93ceddbc...
git tag --contains 93ceddbc | grep "^v18" | sort -V | head -1  # → v18.3.0
gh release view v18.3.0 --json body | grep -i "kubernetes health"  # → Found!
# Result: v18.3.0 (Oct 29, 2025)
```

## Checklist

- [ ] Found backport PR or verified in release branch
- [ ] Identified release version
- [ ] Verified in release notes or test plans
- [ ] Noted any exceptions (master-only, no release notes, etc.)

## Tools

```bash
# Search PRs
gh pr list --author "user" --search "[v18] keyword"

# Get commit
gh pr view <PR> --json mergeCommit

# Find release
git tag --contains <SHA> | grep "^v18" | sort -V | head -1

# Verify inclusion
git merge-base --is-ancestor <SHA> v18.0.0

# Check release notes
gh release view v18.3.0 --json body

# Check test plans
grep -i "feature" .github/ISSUE_TEMPLATE/testplan.md
```
