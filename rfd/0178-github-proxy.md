---
author: Steve Huang (xin.huang@goteleport.com)
state: implemented
---

# RFD 178 - GitHub Proxy

## Required Approvers

- Engineering: @r0mant && @smallinsky
- Product: @klizhentas || @xinding33

## What

This RFD proposes design and implementation of proxying the Git SSH protocol
for GitHub repositories.

## Why

GitHub Enterprise provides a security feature to bring your own SSH certificate
authorities (CA). Once a CA is added, your organization can sign short-lived
client SSH certificates to access organization resources on GitHub. You can
also require your members to use these SSH certificates, which disables Git
access using personal tokens.

The concept of short-lived SSH certificates to access organization resources
aligns well with Teleport, where a Teleport user begins their day with a 'tsh'
session, accessing only what their roleset permits. Teleport can also easily
provide the capability to issue of short-lived client SSH certificates for
GitHub organizations so Teleport customers do not need to implement a separate
system for issuing these certificates. 

Teleport also offers other GitHub-related features, such as [GitHub IAM
integration](https://github.com/gravitational/teleport.e/blob/master/rfd/0021e-github-iam-integration.md)
and GitHub SSO, where this functionality can integrate nicely. Additionally,
proxying GitHub SSH through Teleport provides features like per-session MFA,
audit logging.

Teleport today offers a similar GitHub integration using
[`cert_extensions`](https://github.com/gravitational/teleport/blob/branch/v16/docs/pages/admin-guides/management/guides/ssh-key-extensions.mdx)
in the role options. This proposed GitHub proxy is considered an upgrade to the
existing feature and should replace it.

## Details

### UX - User stories

#### Alice configures GitHub proxy via UI

Alice is a system administrator and she would like to setup the GitHub SSH CA
integration with Teleport.

Alice logs into Teleport via Web UI, and searches "github" on the "Enroll New
Integration" page. Alice selects a "guided" GitHub integration experience:

<img src="./assets/0178-enroll-select.png" width="600" /> 

1. Alice inputs the GitHub organization "my-org", and follows the instruction
   to setup a GitHub OAuth app:

<img src="./assets/0178-enroll-step1.png" width="600" />

2. Next Alice creates integration resource by inputting a name for it. Once the
   integration is created, the SSH CA and fingerprint are displayed with a link
   to the organization's security setting page and instructions to add the CA
   to GitHub.

<img src="./assets/0178-enroll-step3.png" width="600" />

3. Next Alice creates a Teleport role for Teleport users to access.

<img src="./assets/0178-enroll-step4.png" width="400" />

4. Lastly, Alice is presented instructions on how to use `tsh` to setup the Git
   repos.

<img src="./assets/0178-enroll-step5.png" width="600" />

After the enrollment, Alice needs to assign the created role to desired
Teleport users. For example, once she assigns the role to herself, she will
find the Git server in unified resource view:

<img src="./assets/0178-unified-resource-git.png" width="400)" />

Clicking on "Connect" will open a dialog that provides the same instructions to
on how to use the feature with `tsh`.

#### Alice configures GitHub proxy via CLI

Alice is a system administrator and she would like to setup the GitHub SSH CA
integration with CLI. An official guide is provided on
`https://goteleport.com/docs/` to set this up with the following steps.

First, she configures a new OAuth app on GitHub side, with the following info:
- Application Name: <teleport-cluster-name>
- Homepage URL: <teleport-proxy-addr>
- Authorization callback URL:: <teleport-proxy-addr>/v1/webapi/github/callback

Then she creates a GitHub integration using `tctl create`:
```yaml
kind: integration
sub_kind: github
version: v1
metadata:
  name: github-my-org
spec:
  github:
    organization: my-org

  credentials:
    id_secret:
      id: <oauth-app-client-id>
      secret: <oauth-app-client-secret>
```

Next she exports the SSH CA and imports it to her GitHub organization:
```shell
$ tctl auth export --integration github-my-org --type github
ssh-rsa <cert-pem...>

Go to https://github.com/organizations/my-org/settings/security and click "New
CA" in the "SSH certificate authorities" section.

Copy-paste the above certificate there and click "Add CA". The CA should have
the following sha256 fingerprint:
<fingerprint...>
```

Next, she creates a GitHub proxy server for Teleport users to access, using
`tctl create`:
```yaml
kind: git_server
sub_kind: github
spec:
  github:
    integration: github-my-org
    organization: my-org
version: v2
```

To provide access, Alice creates the following role and attach it to desired
Teleport users:
```yaml
kind: role
metadata:
  name: github-my-org-access
spec:
  allow:
    github_permissions:
    - orgs:
      - my-org
 ...
version: v7
```

Alternatively, Alice can map `{{internal.github_orgs}}` or
`{{external.github_orgs}}` traits of the users instead of hardcoding the org in
the role.

#### Bob clones a new Git repository

Bob, a Teleport user that's granted access to the GitHub organization, wants to
clone a repo.

He logins the `tsh`, and run `tsh git ls` to see what access he has:
```shell
$ tsh git ls
Type   Organization Username  URL                                     
------ ------------ --------- -------------------------
GitHub my-org       (n/a)*    https://github.com/my-org

(n/a)*: Usernames will be retrieved automatically upon git commands.
        Alternatively, run `tsh git login --github-org <org>`.

hint: use 'tsh git clone <git-clone-ssh-url>' to clone a new repository
      use 'tsh git config update' to configure an existing repository to use Teleport
      once the repository is cloned or configured, use 'git' as normal
```

He goes on `github.com` and finds the SSH clone url for the repository he wants
to clone. Then he runs the `tsh git clone` command with copied url:
```shell
$ tsh git clone git@github.com:my-org/my-repo.git
```

The first `git` command (including the `clone`) will open a browser window to
trigger the GitHub OAuth flow for Teleport to grab Bob's GitHub ID and
username. Once Bob sees "Login Successful" from the browser and goes back to his
terminal.

The repo is cloned by now, and Bob can `cd` into the directory and perform regular
`git` commands naturally, without using `tsh`. Bob can also find the
"authorized" GitHub username in `tsh status` or `tsh git ls`.

On the second day (as the `tsh` session expires), when Bob tries to `git
fetch` from the repo, the command prompts to login into Teleport. The command
proceeds as usual once Teleport login is successful.

#### Bob configures an existing Git repository

Bob, a Teleport user that's granted access to the GitHub organization, wants to
use Teleport for an existing repo he has cloned before:
```shell
$ cd my-repo
$ tsh git config update
The current git dir is now configured with Teleport for GitHub organization "my-org".

Your GitHub username is "my-git-username".

You can use `git` commands as normal.
```

If one day Bob needs to revert Teleport settings in the Git repo:
```shell
$ tsh git config reset
The Teleport configuration for the current git dir is removed.
```

#### Bob uses the GitHub proxy on a remote machine

Bob, a Teleport user that's granted access to the GitHub organization, wants to
uses the GitHub proxy on a remote machine that cannot open browser windows.

If Bob has never went through the GitHub OAuth flow to prove his GitHub
identity, he has two options:
- Option 1: run `tsh git login --github-org my-org` on his laptop where a browser is
  available. The GitHub identity will be preserved when he logins on the remote
  machine.
- Option 2: when `tsh ssh` (or `ssh`) into the the remote machine, forwards a
  local port say `-L 8080:localhost:8080`. Then on the remote machine, specify
  this port for the callback like `tsh git login --github-org my-org --callback-addr localhost:8080`.
  Bob can finish the OAuth flow using the link provided by `tsh` in his
  laptop's browser.

#### Alice wants to require Bob to use MFA for every `git` command.

Alice, a system administrator, wants to ensure that every single git command
executed by a Teleport user requires MFA, in case their on-disk Teleport
certificates are compromised.

Alice can enable per-session MFA in their Teleport role:
```diff
kind: role
metadata:
  name: github-my-org-access
spec:
  allow:
    github_permissions:
    - orgs:
      - my-org
+   require_session_mfa: true
version: v7
```

Now, when Bob (a Teleport user) runs `git` commands, the command also prompt
for MFA. The `git` command proceeds as usual once MFA challenge is succeeded.

#### Charlie wants to audit GitHub access

Charlie is an auditor and is able to see the audit events from Web UI:

<img src="./assets/0178-audit-event.png" width="600)" />

#### Alice wants to understand the available break glass options

Alice, a system administrator, manages the Teleport cluster by checking
Terraform scripts and values into various GitHub repos. CI/CD then picks the
changes and apply to the Teleport cluster.

A change to the Terraform script may break the Teleport cluster and the GitHub
proxy will not be usable.

Alice has selected the option to disallow personal access tokens and SSH keys
at the organization level and does not want to allow it for security purpose.

Alice still has a few options to access the organization repos when the GitHub
proxy is unavailable:
- Alice can still logs into GitHub through a browser and make changes there if
   necessary.
- Alice can manually sign an user certificate according to [GitHub
  spec](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-git-access-to-your-organizations-repositories/about-ssh-certificate-authorities#issuing-certificates).
  The CA used to generate the user certificate could be a backup of Teleport's
  CA that is exported to a 3rd party secret store. Or, Alice can generate a
  self-signed CA on the spot and import it to the Organization settings
  temporarily.

In addition, trusted GitHub environments like GitHub actions are not affected
by the option to disallow personal access tokens. GitHub allows these services
to continue use existing authentication methods so these services do not need
to go through Teleport.

### Implementation

#### Overview
```mermaid                                
sequenceDiagram
    participant git
    participant tsh
    participant client browser
    participant Proxy
    participant Auth
    participant GitHub
                                
    git->>tsh: sshCommand="tsh git ssh"
    alt no GitHub user ID
      tsh->>Proxy: CreateGitHubAuthRequestForUser
      Proxy->>Auth: CreateGitHubAuthRequestForUser
      tsh->> client browser: open
      client browser->> GitHub: redirect
      GitHub<<->>Proxy: callback
      Proxy<<->>Auth: verify callback and generate new cert with GitHub user ID
      client browser->>tsh: new cert with GitHub user ID
    end
    tsh->>Proxy: SSH transport and RBAC
    Proxy->>Auth: GenerateGitHubUserCert
    Auth->>Proxy: signed cert with "id@github.com" ext
    Proxy->>GitHub: forward SSH
    git <<->>GitHub: git pack-protocol
```

#### The GitHub OAuth flow

Teleport needs user's GitHub ID or username in order to sign an user
certificate to authenticate with GitHub.

One option to achieve this is to allow the administrator to input each user's
GitHub identity information as a trait. However, this was deemed insecure, as
an admin could potentially alter a user's GitHub identity to impersonate
another GitHub user.

Instead, we use the GitHub OAuth flow to retrieve each user's GitHub identity.
This process is similar to the existing GitHub SSO flow; however, while GitHub
SSO creates a new user, this GitHub OAuth flow is designed to update the login
state for an already authenticated user.

To be more specific, the current GitHub SSO flow initiates an unauthenticated
HTTP call to Teleport to create a GitHub auth request. Once the Auth service
verifies the GitHub callback, it creates a new user associated with a GitHub
connector.

In contrast, the new flow uses an authenticated gRPC call to Teleport to create
the GitHub auth request. Once the Auth service verifies the GitHub callback, it
attaches the GitHub identity information directly to the authenticated user who
initiated the request.

This flow is illustrated in the overview diagram above. Here are some details
on the new gRPC call:
```protobuf
service AuthService {
...
  // CreateGithubAuthRequestForUser creates a GithubAuthRequest for an authenticated user.
  rpc CreateGithubAuthRequestForUser(CreateGithubAuthRequestForUserRequest) returns (types.GithubAuthRequest);
}

message CreateGithubAuthRequestForUserRequest {
  // CertRequest is the reissue cert request
  UserCertsRequest cert_request = 1 [(gogoproto.jsontag) = "cert_request,omitempty"];
  // RedirectUrl is the redirect url used by client browser.
  string redirect_url = 2 [(gogoproto.jsontag) = "redirect_url,omitempty"];
}
```

The `UserCertsRequest` will have a new Git server route to indicate the GitHub
organization to access:
```protobuf
message RouteToGitServer {
  // GitHubOrganization is the GitHub organization to embed.
  string GitHubOrganization = 1 [(gogoproto.jsontag) = "github_organization"];
}
```

The Auth service locates the Git server accessible to the authenticated user
based on RBAC permissions for the Git server resource. It then retrieves the
GitHub connector information from the integration resource associated with that
Git server.

After the Auth service verifies the GitHub callback, it updates the user's
"Login State" rather than the user resource. And with the extra GitHub identity
info, the reissued SSH user cert will have the following extensions:
- `github-id@goteleport.com` for GitHub user ID
- `github-login@goteleport.com` for GitHub username

Note that the GitHub identity is preserved in the login state during login
state refresh (like a new login event).

#### GitHub Integration resource

The GitHub integration resource is be a subkind (`github`) of `types.Integration`
and is shared with [GitHub IAM
integration](https://github.com/gravitational/teleport.e/blob/master/rfd/0021e-github-iam-integration.md)
feature.

```yaml
kind: integration
sub_kind: github
version: v1
metadata:
  name: github-my-org
spec:
  github:
    organization: my-org
```

The SSH CAs and GitHub OAuth secrets are stored as
`types.PluginStaticCredentials` in the backend.

The integration resource owns a hidden `types.PluginStaticCredentialsRef`
which can be used to retrieve the actual credentials.

#### GitHub proxy server resource

A new resource kind `git_server` is introduced. On the backend, it uses
`gitServers` as prefix and `types.ServerV2` as the object definition:
```yaml
kind: git_server
sub_kind: github
version: v2
metadata:
  labels:
    teleport.hidden/github_organization: my-org
  name: <uuid>
  revision: <rev-id>
spec:
  github:
    integration: github-my-org
    organization: my-org
  hostname: my-org.github-organization
version: v2
```

Some notable details on the `git_server` object:
- The hostname will always be hardcoded to `<org>.github-organization` for
  routing purpose (explained in the section below).
- The corresponding `integration` must present in the spec.
- A hidden label referencing the `<org>` is automatically added for access
  check purposes.

Corresponding CRUD operations on `git_server` will be added similar to other
presence types like nodes, app servers, etc.

Even though GitHub proxy servers could be singletons (per organization) that
are only served by Proxies, the keepalive capability for `git_server` is
reserved for future expansions like GitLab or self-hosted GitHub where an agent
may be necessary for network access.

#### RBAC on GitHub proxy server

Access to GitHub organizations are granted through `github_permissions.orgs`:

```yaml
kind: role
metadata:
  name: github-my-org-access
spec:
  allow:
    github_permissions:
    - orgs:
      - my-org
version: v7
```

Wildcard `*` can be used for `orgs` by admins and certain built-in roles.

Note that the role spec for GitHub does not follow the
common-resources-label-matching approach like `app_labels`, but shares the same
format for the GitHub IAM integration.

Internally, the access checker converts the `orgs` to labels that can be
matched against the hidden label from the `git_server` resources.

#### SSH transport

Existing [SSH
transport](https://github.com/gravitational/teleport/blob/master/rfd/0100-proxy-ssh-grpc.md)
is used for proxying Git commands. 

No change is necessary on the client side or on the GRPC protocol to support
`git_server`.

Routing is achieved by parsing hostnames in format of
`<my-org>.github-organization` when Proxy receives the dial request. If
multiple `git_server` exist for the same organization, a random server is
selected.

Then the request is forwarded directly to GitHub without going through an
agent, similar to existing OpenSSH node flows. The differences being:
- The target address is **always** `github.com:22`
- The user cert is generated by an Auth call. (And the user cert is cached by
  Proxy for a short period for better performance.)

#### Authentication with GitHub

The Proxy service makes an API call to Auth service to generate a GitHub user
certificate:
```protobuf
// IntegrationService provides methods to manage Integrations with 3rd party APIs.
service IntegrationService {
...
  // GenerateGitHubUserCert signs a SSH certificate for GitHub integration.
  rpc GenerateGitHubUserCert(GenerateGitHubUserCertRequest) returns (GenerateGitHubUserCertResponse);
}

// GenerateGitHubUserCertRequest is a request to sign a client certificate used by
// GitHub integration to authenticate with GitHub enterprise.
message GenerateGitHubUserCertRequest {
  // Integration is the name of the integration;
  string integration = 1;
  // PublicKey is the public key to be signed.
  bytes public_key = 2;
  // UserID is the GitHub user ID.
  string user_id = 3;
  // KeyId is the certificate ID, usually the Teleport username.
  string key_id = 4;
  // Ttl is the duration the certificate will be valid for.
  google.protobuf.Duration ttl = 5;
}

// GenerateGitHubUserCertResponse contains a signed certificate.
message GenerateGitHubUserCertResponse {
  // AuthorizedKey is the signed certificate.
  bytes authorized_key = 1;
}
```

The Auth service generates the certificate according to [GitHub
spec](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-git-access-to-your-organizations-repositories/about-ssh-certificate-authorities#issuing-certificates):
- `id@github.com` extension with the GitHub user ID as the value.
- `ValidBefore` with a short TTL (10 minutes).
- Teleport username as key identity.

#### `tsh git ls` command

The `tsh git ls` provides general information for what organizations an user
can access and provides some instructions on the usage:
```shell
$ tsh git ls
Type   Organization Username        URL                                     
------ ------------ --------------- -------------------------
GitHub my-org       my-git-username https://github.com/my-org

hint: use 'tsh git clone <git-clone-ssh-url>' to clone a new repository
      use 'tsh git config update' to configure an existing repository to use Teleport
      once the repository is cloned or configured, use 'git' as normal
```

#### `tsh git ssh` command

To forward SSH traffic from `git` to Teleport, the Git repo will be configured
with
[`core.sshCommand`](https://git-scm.com/docs/git-config#Documentation/git-config.txt-coresshCommand)
set to `tsh git ssh --github-org <my-org>`. The `core.sshCommand` makes `git` to
call this command instead of `ssh`.

`tsh git ssh` is a hidden command that basically does `tsh ssh
<my-git-username>@<my-org>.github_organization
<git-upload-or-receive-pack-command>`, discarding the original `git@github.com`
target that `git` uses.

#### `tsh git clone` and `tsh git config` commands

In addition, `tsh` provides two helper commands to automatically configures
`core.sshCommand`.

`tsh git clone <git-url>` calls `git clone -c core.sshcommand=... <git-url>` to
make a clone. Before cloning, the GitHub organization is parsed from the
`<git-url>`, and a GitHub proxy server with its logins is retrieved matching
the GitHub organization. If more than one GitHub logins are available, users
can explicitly specify one using `--username` when running `tsh git clone`.

`tsh git config` checks Teleport-related configurations in the current Git dir
by running `git config --local --default "" --get core.sshCommand`.

`tsh git config update` performs `git config --local --replace-all
core.sshCommand ...` in the current dir to update `core.sshCommand`. Before
updating, the GitHub organization is retrieved from `git ls-remote --get-url`.

`tsh git config reset` restores the Git config in the current dir by removing
`core.sshCommand` with command `git config --local --unset-all core.sshCommand`.

#### `tsh git login` command

The GitHub OAuth flow is automatically triggered upon `git` commands. However,
one can manually start the flow with `tsh git login --github-org <org>`.

#### Recordings and audit events

Regular SSH recordings and session events for the GitHub proxy server will be
disabled. "Git Command" events will be emitted instead:

```protobuf
// GitCommand is emitted when a user performance a Git fetch or push command.
message GitCommand {
  // Metadata is a common event metadata
  Metadata Metadata = 1 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // User is a common user event metadata
  UserMetadata User = 2 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // ConnectionMetadata holds information about the connection
  ConnectionMetadata Connection = 3 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // SessionMetadata is a common event session metadata
  SessionMetadata Session = 4 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // ServerMetadata is a common server metadata
  ServerMetadata Server = 5 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // CommandMetadata is a common command metadata
  CommandMetadata Command = 6 [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

  // CommandServiceType is the type of the git request like git-upload-pack or
  // git-receive-pack.
  string command_service_type = 8 [(gogoproto.jsontag) = "command_service_type"];
  // Path is the Git repo path, usually <org>/<repo>.
  string path = 9 [(gogoproto.jsontag) = "path"];
  // Actions defines details for a Git push.
  repeated GitCommandAction actions = 10 [(gogoproto.jsontag) = "actions,omitempty"];
}

// GitCommandAction defines details for a Git push.
message GitCommandAction {
  // Action type like create or update.
  string Action = 1 [(gogoproto.jsontag) = "action,omitempty"];
  // Reference name like ref/main/my_branch.
  string Reference = 2 [(gogoproto.jsontag) = "reference,omitempty"];
  // Old is the old hash.
  string Old = 3 [(gogoproto.jsontag) = "old,omitempty"];
  // New is the new hash.
  string New = 4 [(gogoproto.jsontag) = "new,omitempty"];
```

#### Usage reporting

There is no heartbeats for `git_server` with subkind `github` (yet).

Existing `SessionStartEvent` will be expanded to include git metadata with
`session_type` of `git`:
```grpc
// SessionStartGitMetadata contains additional information about git commands.         
message SessionStartGitMetadata {                                                                  
  // git server subkind ("github").
  string git_type = 1;             
  // command service type ("git-upload-pack" or "git-receive-pack").
  string command_service_type = 2;                                                        
}
```
### Security

#### The GitHub OAuth flow

The design improves security by using the GitHub OAuth flow to verify the end
user's GitHub identity through their browser, reducing the risk of identity
impersonation.

As stated above, the `types.GithubAuthRequest` is created for an
"authenticated" user. And it's created when:
- The authenticated user can access a Git server of the request GitHub
  organization.
- GitHub connector information is retrieved from the integration associated
  with the Git server.

When the flow is successful, the user login state is updated with retrieved
GitHub identity information. The GitHub identity saved in login state are
preserved when login states are refreshed if no new GitHub identity is
provided.

Rest of the GitHub OAuth flow is the same as the existing GitHub SSO flow.

#### Client <-> Proxy transport

As mentioned above, existing SSH transport is used so nothing new here.

#### Proxy <-> GitHub transport

Teleport authenticates GitHub using certificates signed by the SSH CA
configured for the GitHub organization.

Teleport always use `github.com:22` as the target node port when connecting. In
addition, Teleport verifies the server using the publicly known keys or
fingerprints. They can be hard-coded as constants in the Teleport binary.
However, the server will also try fetching them from
`https://api.github.com/meta` first before using the constants.

#### Generating SSH CA for GitHub

The new CA will be generated using the existing key store. For instance, if the
Auth service stores private keys in AWS KMS, the new CA will also follow this
setup.

GitHub accepts `ssh-rsa`, `ecdsa-sha2-nistp256`, `ecdsa-sha2-nistp384`,
`ecdsa-sha2-nistp521`, or `ssh-ed25519` for the CA (at the moment of writing
this RFD). The same key type for `user CA` will be used for generating the SSH
CA. Details on the key types for each suite can be found in [rfd
0136](https://github.com/gravitational/teleport/blob/master/rfd/0136-modern-signature-algorithms.md).

#### RBAC on integration resources

Secrets like GitHub OAuth client secret and private keys for the SSH CAs are
stored in `types.PluginStaticCredentials` separate in the backend. Only Auth
servers have the permissions to retrieve them.

## Future work

### CA rotation

There is no built-in CA rotation functionality for the MVP. A `tctl auth rotate
--integration` or `tctl integration rotate` command can be implemented in the
future.

One can still perform a CA rotation with the MVP with these **manual** steps:
1. Create a new integration for the same organization and import the CA to GitHub.
2. Point the GitHub proxy server to the new integration.
3. Clean up.

### Git protocol v2

The proposed MVP (using `tsh git ssh`) does not support
`GIT_PROTOCOL=version=2`. Since v2 claims to be 30~50% faster, we can
investigate to support this in the future for performance improvement.

### `git` with OpenSSH

An alternative to using `core.sshCommand` is to let `git` use OpenSSH where the
OpenSSH config uses `tsh git ssh` as proxy commands. Then the git repo can be
potentially configured using
[`url.<base>.insteadOf`](https://git-scm.com/docs/git-config#Documentation/git-config.txt-urlltbasegtinsteadOf):
```
[url "ssh://<my-git-username>@<my-org>.github-organization.<proxy-address>/<my-org>/"]
  insteadOf = <original-login>@github.com:<my-org>/
``` 

This is out of scope of the initial MVP but can be potentially implemented with
an `--openssh` flag for `tsh git clone/config` commands.

Benefits of using OpenSSH on the client side includes ControlMaster support,
Git Protocol V2, etc. However, `tsh proxy ssh` (called by OpenSSH) currently
does not support per-session MFA. Technically, using OpenSSH is also an extra
dependency but we do expect clients already have it installed.

### HSM support

PKCS#11 HSM, or any keystore that requires each Auth to have its own key (e.g.
KMS cross regions) is not supported for MVP. This can be supported in the
future by allowing Auth services to watch GitHub integrations and add their own
keys.

One can still achieve HSM support **manually** with the MVP by creating a new
integration on each Auth service and combines all the keys.

### Machine ID

Support for Git servers should be implemented similar to how SSH is supported
today for Machine ID.

As mentioned earlier, since services like GitHub actions are not affected by
this feature (by not using Teleport), Machine ID supported can be added after
the MVP.

### Access Request

Git proxy servers will not support access requests the same way as SSH servers.
The access request/access list controls will be implemented for GitHub IAM
integration instead.
