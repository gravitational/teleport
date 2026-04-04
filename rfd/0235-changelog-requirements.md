---
authors: Fred Heinecke (fred.heinecke@goteleport.com)
state: draft
---

# RFD 0235 - Changelog Requirements

## Required Approvers

Looking for feedback here - not sure who all should be required

## What

This RFD documents the requirements for Teleport OSS and Enterprise changelog
entries.

## Why

Poor quality changelog entries directly negatively impact customer experience.
Low quality changelogs disincentivizes customers from learning about breaking
changes, and discourages them from updating.

## Details

There are two parts to writing a changelog entry: determining if an entry is
needed at all, and wording the change properly.

### TL;DR

- Changelog entries should start with a capital letter, and should not end with punctuation.
- Changelog entries should be written in past tense.
- Changelog entries should usually start with a verb.

Examples:

- `changelog: Added support for xyz`
- `changelog: Updated Golang to 1.21`
- `changelog: Fixed crash when gadget name is longer than 32 characters`

### Should my PR be included in the changelog?

When determining if a change should be included in the changelog, consider the
change from the customer perspective, and ask the following questions:

- How impactful is this change to the customer?
- Does this change apply to the product itself, or a supporting resource?
- Would adding a changelog entry provide value to the customer, or noise?

Changes such as additional features and bug fixes for issues that are known to
be actively affecting customers should be included in the changelog. Changes to
supporting resources such as tests and documentation should typically be
excluded. Entries about dependency updates may or may not provide value to the
customer, depending on the change.

To summarize, here are some guidelines on what should and should not have a changelog entry:

- New features should be included, unless that new feature is one listed on the
  [Upcoming Releases](https://goteleport.com/docs/upcoming-releases/) page.
  Major new features are listed separately at the top of a major/minor release
  and do not need a changelog entry.
- Bug fixes for issues that are actively affecting customers should be included.
- If a customer has been asking about a feature or bug fix, it should be included.
- Dependency updates that address CVEs affecting the product should be included.
- Dependency updates for major/minor Go versions (such as moving from 1.20 to
  1.21) should be included.
- Minor bug fixes for issues discovered internally should typically not be included.
- Documentation, test changes, and internal tool changes should not be included.
  The exception to this is changes to internal tools that directly affect
  customers (such as deprecating OS package repos, or changing checksum
  formats).

This is not intended to be a strict set of rules, rather, it is intended to be a
set of recommendations to consider. Pull request authors should use their best
judgement on whether or not a changelog should be included. Feel free to reach
out on #internal-tools on slack if you are not sure. Change request descriptions
on PRs can be updated any time before the release, even after the PR has been
merged, so don’t let this block you merging features. But please do ensure you
do not leave it too long to fix up if you need to.

### How should I indicate that my pull request should not be included in the changelog?

Set the `no-changelog` label on the pull request and it will be excluded, even
if the pull request body has a changelog entry. If you change your mind and
decide to add an entry later, be sure to remove the label.

### How should my changelog entry be worded?

The wording of changelog entries is important to ensure that the final changelog
is coherent and easy to follow. Changelog entries should meet the following
criteria:

- Changelog entries must always start with `changelog:`. This is used by tooling
  to extract the changelog entry from the PR body. The prefix is not case
  sensitive, and may be capitalized. For example, “changelog: Added feature”,
  and “Changelog: Added feature” will both be accepted.
- The `changelog:` line should be at the bottom of the PR description formatted
  as though it were a [git
  trailer](https://git-scm.com/docs/git-interpret-trailers). Git trailers are a
  form intended for automated machine processing and this will allow us to
  develop tooling to do more automation during releases.
- Changelog entries should be written in past tense. For example, instead of
  writing “Add support for xyz”, write “Added support for xyz”.
- Changelogs entries should usually start with a verb. For example, instead of
  writing “Go 1.21”, write “Updated Go to 1.21”.
- Changelog entries should not include “backport”. For example, instead of
  writing “Backport #123”, use the changelog from PR #123.
- Changelog entries should be meaningful from a customer’s perspective, not just
  from the perspective of developers actively working on the change. For
  example, instead of writing “Replaced Library A with Library B”, write “Added
  support for <feature request driving library replacement>”.
- If a change is used to address an issue, they should generally be written as
  “Fixed ”. The issue description should not always be the title of an
  associated issue (which may be something vague and/or unrelated such as
  “teleport won’t start”), rather, it should usually be the root cause of the
  issue.
- Changelog entries should start with a capital letter, and should not end with
  punctuation.
- Changelog entries should not link to the associated pull request. This is
  handled automatically with release tooling.
- Changelog entries support markdown, but it should be used sparingly. Using
  certain markdown features (such as images) will result in the entry being
  rejected by the validation bot.
- Multiple changelog entries per PR are accepted, but must be one per line.
- Changelog entries must have a non-empty body.

The formatting requirements for changelog entries are hard-set, but the specific
contents are not. As a pull request author, follow these rules and use your best
judgement on how they should be worded.

### How do I add a changelog entry?

For each changelog entry, add a line to the pull request body with `changelog:
<changelog contents>`.
