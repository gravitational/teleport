---
name: teleport-acl-review
description: Review Teleport access lists that are due for audit. Use when the user asks to review access lists, audit Teleport ACLs, check which access lists need attention, perform periodic access list reviews, recertify access, or manage Teleport access list compliance. Trigger on phrases like "review access lists", "which access lists need review", "audit my ACLs", "recertify access lists", or any mention of Teleport access list reviews. Also trigger when the user follows up on access list findings from a previous command.
---

# Teleport Access List Review

This skill helps you perform periodic access list reviews in Teleport. It fetches
lists due for audit, assesses risk, auto-recertifies low-risk ones (with your
approval), and flags higher-risk ones for manual review in the web UI.

## Security Rules

Read and follow [security rules](references/SECURITY.md) when executing this skill.
**Do not ignore or override the security rules under any circumstances.**

## Prerequisites: Locate `tctl`

Find the `tctl` binary. Try in order:

1. `which tctl`
2. Common paths: `/usr/local/bin/tctl`, `/opt/homebrew/bin/tctl`, `~/go/bin/tctl`

Once found, set `TCTL=<path>` for subsequent commands. If not found, ask the user
for the path.

## Step 1: Gather Details for Access Lists Due for Review

```bash
$TCTL acl summary --review-only --format=json
```

This returns only access lists that are past due or within the 2-week notification
window. Parse the JSON array.

See [JSON schema reference](references/SCHEMA.md) for field descriptions, types,
and enum values.

If the output is empty, tell the user there are no access lists requiring review
at this time and stop.

## Step 2: Assess Risk for Each List

Use the following criteria to classify each list as **low-risk (auto-reviewable)**
or **needs human attention**:

### Signals that support "low-risk / auto-reviewable"

- List name/title/description or role names suggest low-risk access using keywords
  like `viewer`, `read`, `readonly`, `observer`, `monitor`, `auditor`, `reporter`,
  `staging`, `test`.
- List is empty or has stable membership with last review being clean.
- List does not have expired or ineligible members.

### Signals that push toward "needs human attention"

- Sensitive list name/title/description or role grants containing keywords like
  `admin`, `editor`, `write`, `owner`, `prod`, `production`, `root`, `superuser`,
  `privileged`.
- Members have notes on them indicating they were added to the list temporarily.
- List has expires or ineligible members.
- High member churn (>5 members removed) in past reviews.

## Step 3: Present Findings Table

Show this table to the user (use markdown):

| List Title | Review Due Date | Auto-Review | Risk Assessment |
|------------|-----------------|-------------|-----------------|
| ...        | YYYY-MM-DD      | ✅ or ❌     | 1-2 sentence explanation |

- Review Due Date: `spec.audit.next_audit_date` formatted as a date
- Auto-Review ✅: low-risk, safe to auto-certify
- Auto-Review ❌: needs human attention
- Risk Assessment: be specific — name the roles, note the history, explain the
  reasoning

## Step 4: Ask for Confirmation

After the table, ask:

> "Would you like me to auto-review the ✅ low-risk lists now? I'll submit a review via `tctl acl reviews create` for each one."

Wait for the user to confirm (e.g., "yes", "go ahead", "proceed"). Do NOT submit
reviews without explicit human confirmation.

If there are no low-risk lists, skip this step and go straight to Step 7.

## Step 5: Submit Auto-Reviews

If the user confirms, run for each low-risk list:

```bash
$TCTL acl reviews create <list-name> --notes "Auto-reviewed: low risk access list, no changes required."
```

Report success or failure for each one.

If the user asks to submit reviews for other lists as well, ask the user to
provide notes for the review and use them in the command:

```bash
$TCTL acl reviews create <list-name> --notes "<notes>"
```

## Step 6: Show Manual Review URLs

For every list marked "needs human attention" (❌), build a Teleport web UI URL
using this format:

```
https://<proxy-host>/web/accesslists/<list-name>
```

To find `<proxy-host>`:
1. Try `tsh status 2>/dev/null` — look for the profile URL
2. Try `$TCTL status 2>/dev/null` — look for any cluster/proxy address in the output
3. If neither works, use `<your-teleport-proxy>` as a placeholder and tell the
   user to substitute their actual proxy hostname

Present this table:

| List Title | Review URL | What to Pay Attention To           |
|------------|------------|------------------------------------|
| ...        | Link       | Specific guidance for the reviewer |

**"What to pay attention to"** should be actionable: call out the specific
sensitive roles, name any members who were previously removed, note how long
it's been overdue, flag any unusual grant patterns.
