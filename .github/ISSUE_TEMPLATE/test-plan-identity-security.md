---
name: Identity Security Test Plan
about: Manual test plan for Teleport Identity Security
title: "Teleport Identity Security X Test Plan"
labels: testplan identity-security
---

## Manual Testing Plan

Below are the items that should be manually tested with each release of Teleport
Identity security.
These tests should be run on both a fresh installation of the version to be released
as well as an upgrade of the previous version of Teleport.

Before running tests, set up both environments:

- Self-hosted: Deploy the [Access Graph service](https://goteleport.com/docs/identity-security/access-graph/)
- Cloud: Create a Teleport Cloud tenant with Identity Security enabled

- [ ] Access Graph (Self-hosted/Cloud)
  - [ ] Standing Privileges
    - [ ] Ignores Review/Request paths
    - [ ] Count standing privileges matches expected number
    - [ ] Aggregates privileges from Teleport roles, cloud integrations, and third-party IdPs
  - [ ] Graph Nodes
    - [ ] SSH Nodes, Kubernetes Clusters, Apps, Databases are displayed
    - [ ] Actions have the user traits displayed
    - [ ] User groups have their traits assigned
    - [ ] Deny actions are displayed and take precedence over allow paths
    - [ ] Temporary actions are displayed and removed after expiration
    - [ ] Database object-level permissions (INSERT, SELECT, UPDATE) shown for supported protocols
  - [ ] Teleport Access Paths
    - [ ] Roles are correctly mapped from users to their resources
    - [ ] Roles are interpolated depending on user traits
    - [ ] Access Lists are imported
    - [ ] Review/Request paths are displayed
  - [ ] Graph Explorer
    - [ ] Global search (search bar and `/` key) works
    - [ ] Right-click filtering narrows graph view correctly
    - [ ] Drill-down into specific access routes per role or identity
    - [ ] Detailed node inspection via drawer panels
    - [ ] Multi-system integration: access controls from cloud providers, repos, and IdPs unified

- [ ] SQL Editor (Self-hosted/Cloud)
  - [ ] Filter by access type (`WHERE kind = 'ALLOWED'` and `WHERE kind = 'DENIED'`)
  - [ ] Filter by identity (`WHERE identity = 'bob'`)
  - [ ] Filter by identity and resource (`WHERE identity = 'bob' AND resource = 'postgres'`)
  - [ ] Filter by resource labels (`WHERE resource_labels @> '{"key": "value"}'`)
  - [ ] Query `ssh_keys` view (`SELECT * FROM ssh_keys`)
  - [ ] Query `access_path` view with source filter (`SELECT * FROM access_path WHERE source='gitlab'`)

- [ ] Crown Jewels (Self-hosted/Cloud)
  - [ ] Create a Crown Jewel
  - [ ] Edit role or user that has access to the Crown Jewel
  - [ ] Ensure Crown Jewel diff is created
    - [ ] Visual diff shows `-` for removed access paths and `+` for newly added ones
    - [ ] Audit event is emitted with correct metadata (affected resource, change ID, timestamp, event code TAG001I)
    - [ ] Audit events can be exported via event handlers to external SIEM/logging systems

- [ ] Session Summaries (Self-hosted/Cloud)
  Do the following steps using the `tctl` and WebUI flows; [docs](https://goteleport.com/docs/identity-security/session-summaries/#step-25-configure-the-inference-model)
  - [ ] Create `inference_secret`
    - [ ] OpenAI
  - [ ] Create `inference_model`
    - [ ] OpenAI
    - [ ] OpenAI-compatible LLM gateway (e.g., LiteLLM)
    - [ ] Bedrock
      - [ ] Local Credentials
      - [ ] Integration
  - [ ] Create `inference_policy`
    - [ ] Verify session selection filters work (session kind, resource labels, user roles)
  - [ ] Teleport Cloud
    - [ ] Provides a default model
      - [ ] `tctl get inference_model teleport-cloud-default`
      - [ ] Bedrock region is set to `{{env.bedrock_region}}`
      - [ ] Bedrock model is set to `{{env.bedrock_model_id}}`
    - [ ] Forbids creating Bedrock model without integration
    - [ ] Forbids updates to the default `inference_model`
  - [ ] Summarize
    - [ ] SSH
      - [ ] Check the summary
      - [ ] Check the timeline
      - [ ] Ensure the session fallback to the old method if the terminal doesn't support paste mode
      - [ ] Check metadata
      - [ ] Check thumbnail
    - [ ] Kube
      - [ ] Check the summary
      - [ ] Check the timeline
      - [ ] Ensure the session fallback to the old method if the terminal doesn't support paste mode
      - [ ] Check metadata
      - [ ] Check thumbnail
    - [ ] Database
      - [ ] Check the summary
    - [ ] Windows
      - [ ] Check the session plays
    - [ ] For Kube, SSH, database and Windows:
      - [ ] Ensure session player is operational if:
        - [ ] Misses metadata (delete <session-id>.metadata file from session recordings)
        - [ ] Misses timeline (kubectl exec using busybox container)
        - [ ] Misses thumbnail (delete <session-id>.thumbnail file from session recordings)
      - [ ] Player streams everything
  - [ ] Audit events are emitted with risk levels (Critical, High, Medium, Low)
  - [ ] Prometheus metrics track summarization performance by model name and error codes
  - [ ] Sessions exceeding model context window are skipped gracefully


### Integrations

- [ ] SSH Keys Scan (Self-hosted/Cloud)
  - [ ] Nodes do not report `authorized_keys`
  - [ ] Enable SSH Keys Scan `tctl edit access_graph_settings`
  - [ ] Ensure nodes report keys in `/home/*/.ssh/authorized_keys` (can take up to 1h)
    - [ ] Use the SQL editor and `select * from ssh_keys`
      - [ ] Nodes are connected to `host_users`
      - [ ] `host_users` have one or more authorized keys assigned
  - [ ] Scan a laptop
    - [ ] Enrol the device using device trust
    - [ ] `tsh scan keys --proxy=teleport.example.com --dirs=/dir1,/dir2`
  - [ ] Ensure users have their devices and they have `private_keys` assigned
    - [ ] Use the SQL editor and `select * from ssh_keys`
  - [ ] Ensure `private_key` and `authorized_key` are connected when they match

- [ ] GitHuh (Self-hosted/Cloud)
  - [ ] Connect a GitHub organization using the WebUI flow (GitHub App installation)
    - [ ] Imports audit logs (authentication, admin, security events)
    - [ ] Displays GitHub access paths in Graph Explorer
      - [ ] `select * from access_path where source='github'`


- [ ] GitLab (Self-hosted/Cloud)
  - [ ] Connect a GitLab account using the WebUI flow
    - [ ] Imports Users
    - [ ] Imports Roles
    - [ ] Imports Projects
    - [ ] Displays GitLab access paths
      - [ ] `select * from access_path where source='gitlab'`

- [ ] EntraID (Self-hosted/Cloud)
  - [ ] Connect an EntraID account using the WebUI flow (OIDC provider method)
    - [ ] Imports Users
    - [ ] Imports Groups as Access Lists
    - [ ] Displays EntraID access paths
      - [ ] `select * from access_path where source='entraid'`
    - [ ] Connect with External [SSO providers](https://goteleport.com/docs/identity-security/integrations/entra-id/#step-33-analyze-entra-id-directory-in-teleport-graph-explorer) (generate the cache file)
  - [ ] Connect an EntraID account using the [tctl](https://goteleport.com/docs/identity-security/integrations/entra-id/) flow (system credentials method)
    - [ ] Imports Users
    - [ ] Imports Groups as Access Lists
    - [ ] Displays EntraID access paths
      - [ ] `select * from access_path where source='entraid'`
  - [ ] When EntraID is SSO provider for AWS, SSO-based access grants are visualized in Graph Explorer

- [ ] Okta (Self-hosted/Cloud)
  - [ ] Connect an Okta account using the WebUI flow
    - [ ] Imports Users
    - [ ] Imports Groups as Access Lists
    - [ ] Imports Applications
    - [ ] Imports Roles and Role Assignments
    - [ ] Imports API Tokens


- [ ] NetIQ (Self-hosted/Cloud)
  - [ ] Connect a NetIQ account using `tctl plugins install netiq`
    - [ ] Imports Users
    - [ ] Imports Groups
    - [ ] Imports Resources
    - [ ] Imports Business roles, Permission roles, IT roles
    - [ ] Displays NetIQ access paths in Graph Explorer

- [ ] AWS (Self-hosted/Cloud)
  - [ ] Enable AWS Integration
    - [ ] Access Graph displays the AWS access paths
    - [ ] IAM Policies, Groups, Users, Roles are imported
    - [ ] EC2 instances are displayed
    - [ ] EKS clusters are displayed
    - [ ] RDS databases are displayed
    - [ ] S3 Buckets are displayed
    - [ ] KMS Keys are displayed
  - [ ] Enable Teleport Discovery and make it discover a node
    - [ ] Access graph connects the IAM of the discovered server to the Teleport Node
      - [ ] Access Path from the teleport user starts in Teleport and extends to AWS


- [ ] Azure (Self-hosted/Cloud)
  - [ ] Enable Azure Integration
    - [ ] Access Graph displays the Azure access paths
    - [ ] Users, Groups, and Service Principals are imported
    - [ ] Role Definitions and Role Assignments are imported
    - [ ] Virtual Machines are displayed
    - [ ] Nested group memberships are reflected correctly


### Identity Activity Center

- [ ] Teleport Audit Logs forwarding (Self-hosted)
  - [ ] Configure Auth Service to enable `access_graph.audit_log`
  - [ ] Teleport cluster audit events appear in the Investigate tab
  - [ ] Historical import respects the `start_date` parameter
- [ ] Investigate tab
  - [ ] Displays Teleport audit events (certificate issuance, MFA challenges, etc.)
  - [ ] Displays events from integrations (GitHub, AWS, Okta) alongside Teleport events
  - [ ] Cross-platform event correlation works (filter by user, time range, event type)
  - [ ] Long-term retention: events older than default cluster retention are still queryable

- [ ] GitHub
  - [ ] Connect a GitHub organization using the WebUI flow (GitHub App installation)
    - [ ] Imports audit logs (authentication, admin, security events)
    - [ ] Audit logs are stored for long-term retention
    - [ ] Alerts are generated for security events (protected branch changes, secret scanning, etc.)

- [ ] AWS
  - [ ] CloudTrail integration (near-real-time log processing via SNS -> SQS)
    - [ ] CloudTrail events appear in Identity Activity Center
    - [ ] AWS alert detections fire from CloudTrail events
  - [ ] EKS Audit Logs integration
    - [ ] EKS audit log events appear in Identity Activity Center

- [ ] Okta
  - [ ] Audit log streaming (Okta System Log polled every minute)
    - [ ] Authentication events appear in Identity Activity Center
    - [ ] Administrative events appear in Identity Activity Center
    - [ ] Application access events appear in Identity Activity Center

- [ ] Alerts
  - [ ] Alerts dashboard is accessible with severity filter (Critical, High, Medium, Low)
  - [ ] AWS detections fire correctly:
    - [ ] Root account activity
    - [ ] CloudTrail or GuardDuty deletion
    - [ ] Disabled EBS encryption
    - [ ] IAM user creation
    - [ ] Key management changes
  - [ ] GitHub detections fire correctly:
    - [ ] Org security setting update
    - [ ] Protected branch policy change
    - [ ] Repository visibility change
  - [ ] Okta detections fire correctly:
    - [ ] Admin MFA disablement
    - [ ] MFA factor reset
    - [ ] Rate limit violation
  - [ ] Teleport detections fire correctly:
    - [ ] Root SSH session
    - [ ] Local account auth without MFA
    - [ ] Role or connector modification
  - [ ] Impossible travel detection fires for geographically impossible logins