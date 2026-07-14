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
# Auto-discover and enroll cloud infrastructure
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-discovery

# Session recording review
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-session-review

# Access list review
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-acl-review

# Investigate Identity Security Logs
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-investigate

# Request just-in-time access to resources
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-access-request
```

You'll be prompted to pick which agents to install into and whether to install
globally or per-project. Review a skill before use — skills run with your agent's
full permissions.

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

Install:

```bash
npx skills add https://github.com/gravitational/teleport/tree/master/skills/teleport-session-review
```

Example invocations:

- Review my recent Teleport session recordings
- Search session recordings for sessions that touched production databases
- What happened in session &lt;id&gt;?
- Find risky or high-severity sessions from last week
- Download the recording for session &lt;id&gt;

### teleport-investigate

Helps search and explore Teleport's Identity Security activity log with
`tctl investigate` — finding who did what, where, and when across users,
resources, and integrations such as AWS, GitHub, and Okta.

Example invocations:

- Were there any failed authentications from India in the last 7 days?
- What did bot CI-deployer do yesterday?
- Show me who accessed the production-database resource this month
- Show me what activity was performed during the following access request <uuid>

### teleport-discovery

Enroll cloud resources (AWS EC2 instances, AWS EKS clusters, and Azure VMs) into
Teleport using Auto-Discovery. Provides a guided workflow to generate a Terraform
configuration to create an OIDC integration. Use for checking status of the
Discovery Service or troubleshooting resource enrollment.

Example invocations:

- Enroll my AWS EC2 instances into Teleport
- Set up auto-discovery for my EKS clusters
- Enroll my Azure VMs into Teleport
- Why are my resources not enrolling into Teleport?

### teleport-access-request

Helps request just-in-time access to Teleport resources with `tsh`. Finds
requestable resources (`tsh request search`), previews the logins or AWS role
ARNs a resource would grant versus what must be requested (`tsh request
preview`), and creates access requests scoped to a subset of those principals
with inline resource constraints (`tsh request create --resource
'/cluster/node/web-1|logins=root,admin'`).

Example invocations:

- Request access to the web-1 server
- What can I request access to?
- Which logins can I request on the prod database?
- Preview access for /main/app/aws-console
- Request read-only access to the AWS console app
