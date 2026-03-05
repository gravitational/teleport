---
name: quick-docs-issues
description: Find straightforward docs issues that do not require outside research and, as a result, are well suited for work by an AI agent.
version: 1.0.0
---

# Find quick docs issues

Find up to 5 GitHub issues in the `gravitational/teleport` repo that are good
candidates for Claude-assisted documentation editing work.

## Fetching issues

Use the `gh` CLI to fetch open issues with the `documentation` label, filtering
out issues that already have a linked PR and issues that are already assigned:

```bash
gh issue list \
  --repo gravitational/teleport \
  --label documentation \
  --state open \
  -L 25 \
  --search "-linked:pr no:assignee" \
  --json number,title,body,url \
  --jq '[.[] | {number, title, url, body: (.body | .[0:600])}]'
```

## Selecting candidates

From the returned issues, select up to 5 that meet **all** of the following
criteria:

- **Editing or reorganizing existing docs only.** The fix must be achievable by
  changing text, structure, or examples already present in the docs. Do not
  select issues that require writing a guide from scratch, creating new
  diagrams, or adding an entirely new page with no existing content to build on.

- **No outside research required.** All information needed to make the change
  must be contained in the issue itself or already present in the docs. Skip
  issues where the fix requires testing a feature, reading an external source,
  or determining correct behavior that isn't stated in the issue.

- **Unambiguous scope.** The issue should describe a concrete, bounded change.

## Presenting results

For each selected issue, present:

- The issue number and title as a link
- A one-sentence description of the specific change needed
- Which existing doc file(s) are affected, if identifiable from the issue
