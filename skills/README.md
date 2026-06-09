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

### teleport-discovery

Enroll cloud resources (Azure VMs) into Teleport using Auto-Discovery. Provides
a guided workflow to generate a Terraform configuration to create an OIDC
integration. Use for checking status of the Discovery Service or troubleshooting
resource enrollment.

Example invocations:

- Enroll my Azure resources into Teleport
- Why are my VMs not enrolling into teleport?
