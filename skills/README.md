# Teleport Agent Skills

This directory contains Teleport agent skills. Each skill is a self-contained
package that teaches agents such as Claude Code how to perform a specific
Teleport workflow using CLI tools like `tctl` and `tsh`.

## Installation

These skills follow the [Agent Skills specification](https://agentskills.io/specification)
and can be installed into any compatible agent (Claude Code, Cursor, Codex,
Gemini CLI, and others) using Vercel's [`skills`](https://github.com/vercel-labs/skills)
CLI, which discovers and installs skills straight from this repository:

```bash
# Investigate Identity Security Logs
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-investigate

# Review who can access which resources
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-access-review
```

You'll be prompted to pick which agents to install into and whether to install
globally or per-project. Review a skill before use — skills run with your agent's
full permissions.

## Available Skills

### teleport-investigate

Helps search and explore Teleport's Identity Security activity log with
`tctl investigate` — finding who did what, where, and when across users,
resources, and integrations such as AWS, GitHub, and Okta.

Example invocations:

- Were there any failed authentications from India in the last 7 days?
- What did bot CI-deployer do yesterday?
- Show me who accessed the production-database resource this month
- Show me what activity was performed during the following access request <uuid>

### teleport-access-review

Helps review who can reach which resources and whether that access is actually
used, with `tctl access-review` and the `access_path` SQL query language —
access list / ACL recertification, "who can access this resource", "what can
this user access", attesting access for audit, and finding dormant or unused
standing privileges. Pairs with `teleport-investigate` (standing access vs.
historical activity).

Example invocations:

- Who can access the prod-db database?
- Review the Prod Admins access list and flag members who haven't used it in 90 days
- Does alice@example.com have any unused standing access?
- What can the junior-dev role reach in production?
- Attest who can reach prod-db and which grants are dormant
