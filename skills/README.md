# Teleport Agent Skills

This directory contains Teleport agent skills. Each skill is a self-contained
package that teaches agents such as Claude Code how to perform a specific
Teleport workflow using CLI tools like `tctl` and `tsh`.

## Available Skills

### teleport-acl-review

Helps perform bulk reviews of Teleport access lists that are due for periodic
audit. Categorizes lists into low-risk that agent can auto-review and those
that require human review.

Example invocations:

- Review my Teleport access lists
- Which access lists need review?
- Audit my Teleport ACLs

### teleport-session-review

Helps browse, search, and investigate Teleport session recordings. Lists recent
recordings (`tctl recordings ls`), runs semantic and keyword search over session
summaries (`tctl recordings search`), presents a risk-triage table, and — with
confirmation — downloads a recording or hands you a playback link.

Example invocations:

- Review my recent Teleport session recordings
- Search session recordings for sessions that touched production databases
- What happened in session &lt;id&gt;?
- Find risky or high-severity sessions from last week
- Download the recording for session &lt;id&gt;
