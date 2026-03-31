---
name: teleport-acl-audit-report
description: Generate a quarterly PDF audit report of Teleport access list review history for auditors and compliance teams. Use this skill whenever the user asks to generate an access list audit report, export ACL review history to PDF, produce a compliance report for access lists, create a quarterly audit PDF, or document who reviewed which access lists and what was changed. Trigger on phrases like "generate audit report", "export access list reviews to PDF", "quarterly ACL report", "audit PDF", "compliance report for access lists", or any request to produce a PDF documenting Teleport access review activity.
---

# Teleport Access List Audit PDF Report

This skill generates a formatted PDF report from Teleport's access list audit
history — suitable for sharing with auditors or compliance reviewers.

## Prerequisites

1. **`tctl` must be available.** Find it:
   - `which tctl`
   - Common paths: `/usr/local/bin/tctl`, `/opt/homebrew/bin/tctl`, `~/go/bin/tctl`

   If not found, ask the user for the path and set it for subsequent commands.

2. **`reportlab` must be installed.** Check with:
   ```bash
   python3 -c "import reportlab" 2>&1
   ```
   If missing, install it:
   ```bash
   pip3 install reportlab
   ```

## Output filename

The default output filename is `acl_audit_report.pdf` in the current directory.
If the user specified a different filename or path, use that instead.

## Generate the report

Run the bundled script, passing `tctl` output to it:

```bash
python3 <skill-dir>/scripts/acl_audit_report.py [output_filename.pdf]
```

The script internally calls `tctl acl audit summary --format=json` to fetch all
audit history, then builds the PDF. It will print the output path when done.

If `tctl` is not on `$PATH`, set the path before running:

```bash
PATH="<tctl-dir>:$PATH" python3 <skill-dir>/scripts/acl_audit_report.py [output_filename.pdf]
```

## After generation

Once the script succeeds:

1. Open the PDF so the user can see it:
   ```bash
   open <output_filename.pdf>
   ```
2. Tell the user where the file was saved.
3. Briefly summarize what the report contains (period covered, number of access
   lists reviewed, members removed, unique reviewers) — this info is printed by
   the script as it runs.

## Error handling

- **No audit data returned**: the cluster has no review history yet. Tell the user.
- **`tctl` auth error**: the user's session may have expired — suggest `tsh login`.
- **`reportlab` import error**: install it with `pip3 install reportlab`.