# Access Types

Leaf reference for choosing user-facing access type. Internally, an access-type
list is also called a preset because Teleport creates supporting roles.

Do not use internal terms with users:

- Say `standing`, not `long-term`.
- Say `Access Request`, not `short-term`.
- Use `access-request` only as the `tctl --access-type` flag value.
- Avoid saying `preset`.

## Existing Lists

When displaying or reasoning about an existing access list's user-facing access
type from `acl ls` or `acl get` JSON, derive it only from
`metadata.labels["teleport.internal/access-list-preset"]`:

- `long-term`: standing
- `short-term`: Access Request
- absent or empty: custom

Do not use `spec.type` as the access type.

## Supporting Roles

For an access-type list, `metadata.labels["teleport.internal/access-list-preset-roles"]`
names its auto-created supporting roles. Treat only roles named in that label as
supporting roles for the list:

- A role whose name contains `standard` owns every non-AWS-IC resource grant in
  RESOURCE_KINDS.md's Kind Map: labels and principals for SSH, databases,
  Kubernetes, applications, Windows desktops, and GitHub orgs. Application
  identities, including AWS role ARNs, Azure identities, GCP service accounts,
  and MCP tools, belong here too.
- A role whose name contains `awsic` owns only `--aws-ic-assignments`.
- Reviewer and requester roles control the Access Request workflow; they do not
  own resource grants.

Use this mapping only after confirming the role name appears in the preset-role
label. Do not classify an unrelated role as supporting merely because its name
contains one of these words.

## Standing

Flag: `--access-type=standing`

Members get access automatically on login. Use for ongoing, low-friction access:
staging, internal tools, read-only dashboards, or base team access.

## Access Request

Flag: `--access-type=access-request`

Members file an Access Request and owners approve before access is granted for
the request TTL. Use for production, admin, sensitive, incident, JIT, approval,
or short-lived access.

## Ambiguous Requests

If the user gives resources but not the access type:

- Recommend Access Request for prod, admin, sensitive, incident, JIT, or approval
  language.
- Recommend standing for staging, internal tools, read-only, or base team access.
- Ask the user to choose standing or Access Request before drafting final
  approval.
- Do not treat the recommendation as confirmed until the user chooses.

Present per SKILL.md's Presentation section (short discrete choice). Option
content:

- **Standing** - members get access automatically on login (good for staging,
  internal tools, read-only)
- **Access Request** - members must request access and owners approve each
  time (good for production, admin, sensitive)
