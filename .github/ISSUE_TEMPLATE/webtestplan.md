---
name: Web Test Plan
about: Web UI manual test plan for Teleport major releases
title: 'Teleport Web Test Plan'
labels: testplan
---

## Web UI

## Main

For main, test with a role that has access to all resources.

As you go through testing, click on any links you come across to make sure they work (no 404) and are up to date.

### Trusted Cluster (leafs)

- [ ] Can add a trusted cluster

The following features should allow users to view resources in trusted clusters.
There should be a cluster dropdown for:

- [ ] unified resources
- [ ] new access request > resources dropdown
- [ ] active sessions
- [ ] audit log
- [ ] session recordings
- [ ] web terminal console view (can get to it by ssh into a node, or route to: `/web/cluster/<cluster-name>/console/nodes`)
- [ ] Can edit and delete a trusted cluster

### Navigation

- [ ] Contains `Resources` (unified resources), `Access Management`, `Access Requests`, `Active Sessions`, `Notification Bell` and `user settings menu`

### User Settings Menu

- Verify that clicking on the username, user menu dropdown renders:
  - [ ] Account Settings (actions should require re-authn with a mfa device)
    - [ ] Can CRUD passkeys (passwordless)
      - [ ] Can login with added passkey
    - [ ] Can change passwords
      - [ ] Can login with changed password
      - [ ] Verify that account is locked after several unsuccessful change password attempts
    - [ ] Can CRUD MFA devices (test both otp + hardware key)
      - [ ] Can login with added device
    - Recovery codes
      - [ ] Cloud only: can read and generate new recovery codes
  - [ ] Help & Support
    - [ ] Click on all the links and make sure they work (no 404) and are up to date
    - [ ] Renders cluster information
    - [ ] OSS: Premium support is locked behind a CTA button
  - [ ] Can toggle between light and dark theme
    - [ ] Theme preference is saved upon relogin

### Unified Resources

- [ ] Verify that scrolling to the bottom of the page renders more resources
- [ ] Verify that all resource types are visible if no filters are present
- [ ] Verify that "Search" by (host)name, address, labels works for all resources
- [ ] Verify that clicking on `Add Resource` button correctly sends to the resource discovery page
- [ ] Verify pinned resources are saved upon relogin
- Type Server (aka nodes):
  - [ ] Verify that "Servers" type shows all joined nodes
  - [ ] Verify that "Connect" button shows a list of available logins
  - [ ] Verify that terminal opens when clicking on one of the available logins
  - [ ] Check agent forwarding is correct based on role and proxy mode.
    - Set `forward_agent: true` under the `options` section of your role, and then test that your
      teleport certs show up when you run `ssh-add -l` on the node.
- Type Application:
  - [ ] Verify that the app icons are correctly displayed
  - [ ] Verify that filtering types by Application includes applications in the page
  - [ ] Verify that the `Launch` button for applications correctly send to the app
  - [ ] Verify that the `Launch` button for AWS apps correctly renders an IAM role selection window
- Type Database:
  - [ ] Verify that the database subtype icons are correctly displayed
  - [ ] Verify that filtering types by Databases includes databases in the page
  - [ ] Verify that clicking `Connect` renders the dialog with correct information
- Type Kube:
  - [ ] Verify that filtering types by Kubes includes kubes in the page
  - [ ] Verify that clicking `Connect` renders the dialog with correct information
- Type Desktops:
  - [ ] Verify that filtering types by Desktops includes desktops in the page
  - [ ] Verify that clicking `Connect` renders a login selection and that the logins are completely in view

### Active Sessions

- [ ] Verify that it displays the session when session is active
- [ ] Verify that "OPTIONS" button allows to join a session as an:
  - [ ] Observer
  - [ ] Peer
  - [ ] Moderator
    - [ ] Show instructions on how to terminate the session (t or ctrl + c)
    - [ ] OSS: Moderator is locked behind a CTA button

### Access Management Side Nav

- [ ] Verify that each item on side nav has an icon

#### Session Recordings

- [ ] Verify that it can replay an interactive session
- Session Player:
  - [ ] Verify that when playing, scroller auto scrolls to bottom most content
  - [ ] Verify that error message is displayed (enter an invalid SID in the URL)

#### Audit log

- [ ] Verify that time range button is shown and works
- [ ] Verify event detail dialogue renders when clicking on events `details` button
- [ ] Verify searching by type, description, created works

#### Users

All actions should require re-authn with a webauthn device.

- [ ] Verify that creating and editing a user works
- [ ] Verify that removing a user works
- [ ] Verify resetting a user's password works
- [ ] Verify search by username, roles, and type works

##### Invite, Reset, and Login Forms

For each, test the invite, reset, and login flows

- [ ] Verify that input fields validates
- [ ] Verify with `second_factors` set to `["otp"]`, requires otp
- [ ] Verify with `second_factors` set to `["webauthn"]`, requires hardware key
- [ ] Verify with `second_factors` set to `["webauthn", "otp"]`, requires a MFA device
- [ ] Verify that error message is shown if an invite/reset is expired/invalid
- [ ] Verify that account is locked after several unsuccessful login attempts

#### Auth Connectors

For help with setting up auth connectors, check out the [Quick GitHub/SAML/OIDC Setup Tips]

All actions should require re-authn with a webauthn device.

- [ ] Verify when there are no connectors, empty state renders
- [ ] Verify that creating, editing, and deleting OIDC/SAML/GitHub connectors works
- [ ] Verify that login works for GitHub/SAML/OIDC
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct side hint text and doc link is shown
- [ ] Verify that encrypted SAML assertions work with an identity provider that supports it (Azure).
- [ ] Verify that created GitHub, SAML, OIDC card has their icons
- OSS only:
  - [ ] GitHub is allowed
  - [ ] SAML + OIDC are locked behind CTAs

#### Roles

All actions should require re-authn with a webauthn device.

- [ ] Verify that "Create New Role" dialog works
- [ ] Verify that deleting and editing works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text and doc link are shown

#### Enroll New Integration (aka Plugins)

- [ ] All `self-hosted plugins` and `machine id` cards link out to the correct docs
- [ ] All `no-code integrations` renders form
- [ ] All integrations (except for AWS) can only be enrolled once, a checkmark should render and cards are not clickable anymore (unless you delete it)
- Integrations List
  - [ ] Verify the created integrations are shown in the integration list
  - [ ] Integrations can be deleted
- [ ] AWS External Audit Storage renders only for enterprise cloud
- In OSS for `no-code integrations``:
  - [ ] AWS OIDC card is rendered
  - [ ] AWS External Audit Storage is also rendered but locked behind a CTA

#### Enroll new resources using Discover Wizard

Use Discover Wizard to enroll new resources and access them:

- [ ] SSH Server using Teleport Service
- [ ] Self-Hosted PostgreSQL and Mongo
- [ ] Kubernetes
- [ ] Using an AWS OIDC Integration
  - [ ] EC2 Auto Enrollment (SSM)
  - [ ] RDS flow: single database
  - [ ] RDS flow: Auto Enrollment (by VPC)
  - [ ] EKS Clusters
- [ ] Non-guided cards link out to correct docs

#### Access Lists

Not available for OSS

Admin refers to users with access_list RBAC defined:

```
spec:
  allow:
    rules:
    - resources:
      - access_list
      verbs:
      - list
      - create
      - read
      - update
      - delete
```

- [ ] Renders empty state animation highlighting different features of access lsit
- [ ] Admins can create, delete, edit, and review access list
- [ ] Owners (WITHOUT access_list RBAC defined) can still review and edit access lists that they are a owner of
- [ ] Members can only read access list that they are a member of (edit buttons should be disabled)
- [ ] Non members, owners, or admins should not be shown any access list
- [ ] Access list reviews due in two weeks or less are listed under notification bell
- [ ] (Team) and (On-Prem and Cloud Without IGS addon): Renders CTA on empty state or when there are access lists
- [ ] With IGS: Can create unlimited access list

#### Session & Identity Locks

```
spec:
  allow:
    rules:
    - resources:
      - lock
      verbs:
      - list
      - create
      - read
      - update
      - delete
```

- [ ] Existing locks listing page.
  - [ ] It lists all of the existing locks in the system.
  - [ ] Locks without a `Message` are shown with this field as empty.
  - [ ] Locks without an `Expiration` field are shown with this field as "Never".
  - [ ] Clicking the trash can deletes the lock with a spinner.
  - [ ] Table columns are sortable, except for the `Locked Items` column.
  - [ ] Table search field filters the results.
- [ ] Adding a new lock. (+ Add New Lock).
  - [ ] Target switcher shows the locks for the various target types (User, Role, Login, Node, MFA Device, Windows Desktop, Access Request).
  - [ ] Target switcher has "Access Request" in E build but not in OSS.
  - [ ] You can add lock targets from multiple target types.
  - [ ] Adding a target turnst the `Add Target` button into a `Remove` button.
  - [ ] You cannot proceed if you haven't selected targets to lock.
  - [ ] You can clear the selected targets prior to creating locks.
  - [ ] Proceeding to lock opens an animated slide panel from the right.
  - [ ] You can remove lock targets from the slide panel.
  - [ ] Creating a lock with message and TTL correctly create the lock.
  - [ ] Create a lock without message and TTL, they should be optional.

#### Trusted Devices

- [ ] In OSS, locked behind a CTA
- [ ] (Team) and (On-Prem and Cloud Without IGS addon): Limited to 5 devices
- [ ] With IGS: Can add unlimited devices
- [ ] Renders empty state
- [ ] Can add and delete trusted devices

#### Managed Clusters

- [ ] Verify that it displays a list of clusters (root + leafs)
- [ ] Verify that root is marked with a `root` pill
- [ ] Verify that cluster dropdown menu items goes to the correct route

## Application Access

### Required Applications

Create two apps running locally, a frontend app and a backend app. The frontend app should
make an API request to the backend app at its teleport public_addr

<details>
	<summary>You can use this example app if you don't have a frontend/backend setup</summary>
  
  ```go
  package main

  import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
  )

  // change to your cluster addr
  const clusterName = "avatus.sh"

  func main() {
    // handler for the html page. this is the "client".
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
      html := fmt.Sprintf(html, clusterName)
      w.Header().Set("Content-Type", "text/html")
      w.Write([]byte(html))
    })

    // Handler for the API endpoint
    http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
      w.Header().Set("Access-Control-Allow-Origin", fmt.Sprintf("https://client.%s", clusterName))
      w.Header().Set("Access-Control-Allow-Credentials", "true")
      data := map[string]string{"hello": "world"}
      w.Header().Set("Content-Type", "application/json")
      json.NewEncoder(w).Encode(data)
    })

    log.Println("Server starting on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
  }

  const html = `
  <!DOCTYPE html>
  <html lang="en">
  <head>
      <meta charset="UTF-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>API Data Fetcher</title>
  </head>
  <body>
      <div id="result"></div>
      <div id="cors-result"></div>
      <script>
          fetch('https://api.%s/api/data', { credentials: 'include' })
              .then(response => response.json())
              .then(data => {
                  document.getElementById('result').textContent = JSON.stringify(data);
              })
              .catch(error => console.error('Error:', error));
      </script>
  </body>
  </html>
  `
```
</details>

Update your app service to serve the apps like this (update your public addr to what makes sense for your cluster)
```
app_service:
  enabled: "yes"
  debug_app: true
  apps:
    - name: client
      uri: http://localhost:8080
      public_addr: client.avatus.sh
      required_apps:
      - api
    - name: api
      uri: http://localhost:8080
      public_addr: api.avatus.sh
      cors:
        allowed_origins:
          - https://client.avatus.sh
```

Launch your cluster and make sure you are logged out of your api by going to `https://api.avatus.sh/teleport-logout`

- [ ] Launch the client app and you should see `{"hello":"world"}` response
- [ ] You should see no CORS issues in the console

## Access Requests

Not available for OSS

- [ ] (Team) and (On-Prem and Cloud Without IGS addon): Limited to 5 monthly requests
- [ ] With IGS: Unlimited requests

### Access Request Notification Routing Rule (cloud only)

- [ ] Test the default integration rule notifies using default settings
- [ ] Test creating a custom rule that overwrites the default rule (default recipients should not be notified)
- [ ] Test update rule to a different recipient (the previous recipient should not be notified)
- [ ] Test email recipient notification works if applicable (eg slack)
- [ ] Test a rule with multiple recipients and channels (eg slack)
- [ ] Test multiple rules with different condition notifies correctly
- [ ] Test deleting all rules, notifications fallbacks to the default (deleted rules should not be notified)
- [ ] Test pre-defined predicate expressions from default editor work as match condition

### Creating Access Requests (Role Based)

Create a role with limited permissions `allow-roles-and-nodes`. This role allows you to see the Role screen and ssh into all nodes.

```
kind: role
metadata:
  name: allow-roles-and-nodes
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - role
      verbs:
      - list
      - read
  options:
    max_session_ttl: 8h0m0s
version: v5

```

Create another role with limited permissions `allow-users-with-short-ttl`. This role session expires in 4 minutes, allows you to see Users screen, and denies access to all nodes.

```
kind: role
metadata:
  name: allow-users-with-short-ttl
spec:
  allow:
    rules:
    - resources:
      - user
      verbs:
      - list
      - read
  deny:
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 4m0s
version: v5
```

Create a user that has no access to anything but allows you to request roles:

```
kind: role
metadata:
  name: test-role-based-requests
spec:
  allow:
    request:
      roles:
      - allow-roles-and-nodes
      - allow-users-with-short-ttl
      suggested_reviewers:
      - random-user-1
      - random-user-2
version: v5
```

- [ ] Verify that under requestable roles, only `allow-roles-and-nodes` and `allow-users-with-short-ttl` are listed
- [ ] Verify you can select/input/modify reviewers
- [ ] Verify you can view the request you created from request list (should be in pending states)
- [ ] Verify there is list of reviewers you selected (empty list if none selected AND suggested_reviewers wasn't defined)
- [ ] Verify you can't review own requests

### Creating Access Requests (Resource Based)

Create a role with access to searcheable resources (apps, db, kubes, nodes, desktops). The template `searcheable-resources` is below.

```
kind: role
metadata:
  name: searcheable-resources
spec:
  allow:
    app_labels:  # just example labels
      label1-key: label1-value
      env: [dev, staging]
    db_labels:
      '*': '*'   # asteriks gives user access to everything
    kubernetes_labels:
      '*': '*'
    node_labels:
      '*': '*'
    windows_desktop_labels:
      '*': '*'
version: v5
```

Create a user that has no access to resources, but allows you to search them:

```
kind: role
metadata:
  name: test-search-based-requests
spec:
  allow:
    request:
      search_as_roles:
      - searcheable-resources
      suggested_reviewers:
      - random-user-1
      - random-user-2
version: v5
```

- [ ] Verify that a user can see resources based on the `searcheable-resources` rules
- [ ] Verify you can select/input/modify reviewers
- [ ] Verify you can view the request you created from request list (should be in pending states)
- [ ] Verify there is list of reviewers you selected (empty list if none selected AND suggested_reviewers wasn't defined)
- [ ] Verify you can't review own requests
- [ ] Verify that you can't mix adding resources from different clusters (there should be a warning dialogue that clears the selected list)

### Viewing & Approving/Denying Requests

Create a user with the role `reviewer` that allows you to review all requests, and delete them.

```
kind: role
version: v3
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles: ['*']
```

- [ ] Verify you can view access request from request list
- [ ] Verify you can approve a request with message, and immediately see updated state with your review stamp (green checkmark) and message box
- [ ] Verify you can deny a request, and immediately see updated state with your review stamp (red cross)
- [ ] Verify deleting the denied request is removed from list

### Assuming Approved Requests (Role Based)

- [ ] Verify that assuming `allow-roles-and-nodes` allows you to see roles screen and ssh into nodes
- [ ] After assuming `allow-roles-and-nodes`, verify that assuming `allow-users-with-short-ttl` allows you to see users screen, and denies access to nodes
  - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
  - [ ] Verify that you can access nodes after `Drop Request` on `allow-users-with-short-ttl` while `allow-roles-and-nodes` is still assumed
  - [ ] Verify after re-assuming `allow-users-with-short-ttl` role that the next action (i.e. opening a new tab with unified resources) triggers a relogin modal after the expiry is met (4 minutes)

### Assuming Approved Requests (Search Based)

- [ ] Verify that assuming approved request, allows you to see the resources you've requested.

### Assuming Approved Requests (Both)

- [ ] Verify assume buttons are only present for approved request and for logged in user
- [ ] Verify that after clicking on the assume button, it is disabled in both the list and in viewing
- [ ] Verify that after re-login, requests that are not expired and are approved are assumable again

## Access Request Waiting Room

#### Strategy Reason

Create the following role:

```
kind: role
metadata:
  name: waiting-room
spec:
  allow:
    request:
      roles:
      - <some other role to assign user after approval>
  options:
    max_session_ttl: 8h0m0s
    request_access: reason
    request_prompt: <some custom prompt to show in reason dialogue>
version: v3
```

- [ ] Verify after login, reason dialogue is rendered with prompt set to `request_prompt` setting
- [ ] Verify after clicking `send request`, pending dialogue renders
- [ ] Verify after approving a request, dashboard is rendered
- [ ] Verify the correct role was assigned

#### Strategy Always

With the previous role you created from `Strategy Reason`, change `request_access` to `always`:

- [ ] Verify after login, pending dialogue is auto rendered
- [ ] Verify after approving a request, dashboard is rendered
- [ ] Verify after denying a request, access denied dialogue is rendered
- [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
- [ ] Verify switchback button says `Logout` and clicking goes back to the login screen

#### Strategy Optional

With the previous role you created from `Strategy Reason`, change `request_access` to `optional`:

- [ ] Verify after login, dashboard is rendered as normal

## Access Lists

Not available for OSS

- Creating new Access List:
  - [ ] Verify that traits/roles are not be required in order to create
  - [ ] Verify that one can be created with members and owners
  - [ ] Verify the web cache is updated (new list should appear under "Access Lists" page without reloading)
- Deleting existing Access List:
  - [ ] Verify the web cache is updated (deleted list should disappear from "Access Lists" page without reloading)
  - [ ] Verify that an Access List used as a member or owner in other lists cannot be deleted (should show a warning)
- Reviewing Access List:
  - [ ] Verify that after reviewing, the web cache is updated (list cards should show any member/role changes)
- Updating (renaming, removing members, adding members):
  - [ ] Verify the web cache is updated (changes to name/members appear under "Access Lists" page without reloading)
- [ ] Verify Access List search is preserved between sub-route navigation (clicking into specific List and navigating back)
- Can manage members/owners for an existing Access List:
  - [ ] Verify that existing Users:
    - [ ] Can be enrolled as members and owners
    - [ ] Enrolled as members or owners can be removed
  - [ ] Verify that existing Access Lists:
    - [ ] Can be enrolled as members and owners
    - [ ] Enrolled as members or owners can be removed
  - [ ] Verify that an Access List cannot be added as a member or owner:
    - [ ] If it is already a member or owner
    - [ ] If it would result in a circular reference (ACL A -> ACL B -> ACL A)
    - [ ] If the depth of the inheritance would exceed 10 levels
    - [ ] If it includes yourself (and you lack RBAC)
  - [ ] Verify that non-existing Members and Owners can be enrolled in an existing List (e.g., SSO users)
- Inherited grants are properly calculated and displayed:
  - [ ] Verify that members of a nested Access List:
    - [ ] Added as a member to another Access List inherit its Member grants
    - [ ] Added as an owner to another Access List inherit its Owner grants
    - [ ] That do not meet Membership Requirements in a Nested List do not inherit any Grants from Parent Lists
    - [ ] That do not meet the Parent List's Membership/Ownership Requirements do not inherit its Member/Owner Grants
  - [ ] Verify that owners of Access Lists added as Members/Owners to other Access Lists do *not* inherit any Grants
  - [ ] Verify that inherited grants are updated on reload or navigating away from / back to Access List View/Edit route
  - [ ] Verify that 'View More' exists and can be clicked under the 'Inherited Member Grants' section if inherited grants overflows the container

## Web Terminal (aka console)

- [ ] Verify that switching between tabs works with `ctrl+[1...9]` (alt on linux/windows)
- Update your user role to `require_session_mfa` and:
  - [ ] Verify connecting to a ssh node prompts you to tap your registered WebAuthn key
  - [ ] Verify you can still scp upload/download files

#### Terminal Node List Tab

- [ ] Verify that Cluster selector works (URL should change too)
- [ ] Verify that "Connect" button shows a list of available logins
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values
- [ ] Verify that "Search" by hostname, address, labels work
- [ ] Verify that new tab is created when starting a session

#### Terminal Session Tab

- [ ] Verify that session and browser tabs both show the title with login and node name
- [ ] Verify that terminal resize works
  - Install midnight commander on the node you ssh into: `$ sudo apt-get install mc`
  - Run the program: `$ mc`
  - Resize the terminal to see if panels resize with it
- [ ] Verify that session tab shows/updates number of participants when a new user joins the session
- [ ] Verify that tab automatically closes on "$ exit" command
- [ ] Verify that SCP Upload works
- [ ] Verify that SCP Upload handles invalid paths and network errors
- [ ] Verify that SCP Download works
- [ ] Verify that SCP Download handles invalid paths and network errors

## Cloud

From your cloud staging account, change the field `teleportVersion` to the test version.

```
$ kubectl -n <namespace> edit tenant
```

#### Dashboard Tenants (self-hosted license)

- [ ] Can see download page
- [ ] Can download license
- [ ] Can download teleport binaries

#### Recovery Code Management

- [ ] Verify generating recovery codes for local accounts with email usernames works
- [ ] Verify local accounts with non-email usernames are not able to generate recovery codes
- [ ] Verify SSO accounts are not able to generate recovery codes

#### Invite/Reset

- [ ] Verify email as usernames, renders recovery codes dialog
- [ ] Verify non email usernames, does not render recovery codes dialog

#### Recovery Flow: Add new mfa device

- [ ] Verify recovering (adding) a new hardware key device with password
- [ ] Verify recovering (adding) a new otp device with password
- [ ] Verify viewing and deleting any old device (but not the one just added)
- [ ] Verify new recovery codes are rendered at the end of flow

#### Recovery Flow: Change password

- [ ] Verify recovering password with any mfa device
- [ ] Verify new recovery codes are rendered at the end of flow

#### Recovery Email

- [ ] Verify receiving email for link to start recovery
- [ ] Verify receiving email for successfully recovering
- [ ] Verify email link is invalid after successful recovery

## RBAC

Create a role, with no `allow.rules` defined:

```
kind: role
metadata:
  name: rbac
spec:
  allow:
    app_labels:
      '*': '*'
    logins:
    - root
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 8h0m0s
version: v3
```

- [ ] Verify that the user has no `Access` top-level navigation item.
- [ ] Verify that the `Audit` top-level navigation item only contains `Active Sessions`.
- [ ] Verify that on Enterprise, the user has no `Policy` top-level navigation item, while the admin does.
- [ ] Verify that on Enterprise, the `Identity` top-level navigation item only contains `Access Requests` and `Access Lists`.
- [ ] Verify that on Enterprise, the `Add New` top-level navigation item only contains `Resource` and `Access List`.
- [ ] Verify that on OSS, the user has no `Identity` top-level navigation item.
- [ ] Verify that on OSS, the `Add New` top-level navigation item only contains `Resource`.
- [ ] Verify the `Enroll New Resource` button is disabled on the Resources screen.

Note: User has read/create access_request access to their own requests, despite resource settings

Add the following under `spec.allow.rules` to enable read access to the audit log:

```
    - resources:
      - event
      verbs:
      - list
```

- [ ] Verify that the `Audit Log` is accessible

Add the following to enable list access to session recordings:

```
    - resources:
      - session
      verbs:
      - list
```

- [ ] Verify that `Session Recordings` is accessible
- [ ] Verify that playing a recorded session is denied

Change the session permissions to enable read access to recorded sessions:

```
    - resources:
      - session
      verbs:
      - list
      - read
```

- [ ] Verify that a user can re-play a session

Add the following to enable read access to the roles:

```
    - resources:
      - role
      verbs:
      - list
      - read
```

- [ ] Verify that a user can see the roles
- [ ] Verify that a user cannot create/delete/update a role

Add the following to enable read access to the auth connectors

```
    - resources:
      - auth_connector
      verbs:
      - list
      - read
```

- [ ] Verify that a user can see the list of auth connectors.
- [ ] Verify that a user cannot create/delete/update the connectors

Add the following to enable read access to users

```
    - resources:
      - user
      verbs:
      - list
      - read
```

- [ ] Verify that a user can access the "Users" screen
- [ ] Verify that a user cannot reset password and create/delete/update a user

Add the following to enable read access to trusted clusters

```
    - resources:
      - trusted_cluster
      verbs:
      - list
      - read
```

- [ ] Verify that a user can access the "Trusted Root Clusters" screen
- [ ] Verify that a user cannot create/delete/update a trusted cluster.

## Teleport Connect

- Auth methods
  - Verify that the app supports clusters using different auth settings
    (`auth_service.authentication` in the cluster config):
    - [ ] `type: local`, `second_factors: ["otp"]`
      - [ ] Test per-session MFA items listed later in the test plan.
    - [ ] `type: local`, `second_factors: ["webauthn"]`,
      - [ ] Test per-session MFA items listed later in the test plan.
    - [ ] `type: local`, `second_factors: ["webauthn"]`, log in passwordlessly with hardware key
    - [ ] `type: local`, `second_factors: ["webauthn"]`, log in passwordlessly with touch ID
    - [ ] `type: local`, `second_factors: ["webauthn", "otp"]`, log in with OTP
      - [ ] Test per-session MFA items listed later in the test plan.
    - [ ] `type: local`, `second_factors: ["webauthn", "otp"]`, log in with hardware key
    - [ ] `type: local`, `second_factors: ["webauthn", "otp"]`, log in with passwordless auth
    - [ ] Verify that the passwordless credential picker works.
      - To make the picker show up, you need to add the same MFA device with passwordless
        capabilities to multiple users.
    - [Authentication connectors](https://goteleport.com/docs/setup/reference/authentication/#authentication-connectors):
      - For those you might want to use clusters that are deployed on the web, specified in
        parens. Or set up the connectors on a local enterprise cluster following [the guide from
        our wiki](https://www.notion.so/goteleport/Quick-SSO-setup-fb1a64504115414ca50a965390105bee).
      - [ ] GitHub (asteroid)
      - [ ] SAML (platform cluster)
      - [ ] OIDC (e-demo)
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux
- Shell
  - [ ] Verify that the shell is pinned to the correct cluster (for root clusters and leaf
        clusters).
    - That is, opening new shell sessions in other workspaces or other clusters within the same
      workspace should have no impact on the original shell session.
  - [ ] Verify that the local shell is opened with correct env vars.
    - `TELEPORT_PROXY` and `TELEPORT_CLUSTER` should pin the session to the correct cluster.
    - `TELEPORT_HOME` should point to `~/Library/Application Support/Teleport Connect/tsh`.
    - `PATH` should include `/Applications/Teleport Connect.app/Contents/Resources/bin`.
  - [ ] Verify that the working directory in the tab title is updated when you change the directory
        (only for local terminals).
  - [ ] Verify that terminal resize works for both local and remote shells.
    - Install midnight commander on the node you ssh into: `$ sudo apt-get install mc`
    - Run the program: `$ mc`
    - Resize Teleport Connect to see if the panels resize with it
  - [ ] Verify that the tab automatically closes on `$ exit` command.
  - [ ] Verify that [Connect doesn't leave orphaned processes](https://github.com/gravitational/teleport/issues/27125).
    - Open a local shell session in Connect.
    - Open Activity Monitor, then View -> All Processes, Hierarchically.
    - Locate Teleport Connect. There should be Teleport Connect Helper process with a shell
      process under it.
    - Double-click that shell process to open a window with process details.
    - Close Teleport Connect without closing the tab first.
    - Verify that the separate Activity Monitor window for that process now says "terminated".
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux
- Kubernetes access
  - [ ] Open a new kubernetes tab, run `echo $KUBECONFIG` and check if it points to the file within Connect's app data directory.
  - [ ] Close the tab and open it again (to the same resource). Verify that the kubeconfig path didn't change.
  - [ ] Run `kubectl get pods -A` and verify that the command succeeds. Then create a pod with
        `kubectl apply -f https://k8s.io/examples/application/shell-demo.yaml` and exec into it with
        `kubectl exec --stdin --tty shell-demo -- /bin/bash`. Verify that the shell works.
    - For execing into a pod, you might need to [create a `ClusterRoleBinding` in
      k8s](https://goteleport.com/docs/kubernetes-access/register-clusters/static-kubeconfig/#kubernetes-authorization)
      for [the admin role](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles).
      Then you need to add the k8s group (which maps to the k8s admin role in
      `ClusterRoleBinding`) to `kubernetes_groups` of your Teleport role.
    - [ ] Repeat the above check for a k8s cluster connected to a leaf cluster.
  - Verify that the kubeconfig file is removed when the user:
    - [ ] Removes the connection
    - [ ] Logs out of the cluster
    - [ ] Closes Teleport Connect
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux
- State restoration from disk
  - [ ] Verify that the app asks about restoring previous tabs when launched and restores them
        properly.
  - [ ] Verify that the app opens with the cluster that was active when you closed the app.
  - [ ] Verify that the app remembers size & position after restart.
    - Verify that this works on:
      - [ ] macOS
      - [ ] Windows
      - [ ] Linux
  - [ ] Verify that [reopening a cluster that has no workspace
        assigned](https://github.com/gravitational/webapps.e/issues/275#issuecomment-1131663575) works.
  - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport Connect/tsh` doesn't crash the app.
  - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport Connect/app_state.json` but not the `tsh` dir doesn't crash the app.
  - [ ] Verify that logging out of a cluster and then logging in to the same cluster doesn't
        remember previous tabs (they should be cleared on logout).
  - [ ] Open a db connection tab. Change the db name and port. Close the tab. Restart the app. Open
        connection tracker and choose said db connection from it. Verify that the newly opened tab uses
        the same db name and port.
  - [ ] Log in to a cluster. Close the DocumentCluster tab. Open a new DocumentCluster tab. Restart
        the app. Verify that the app doesn't ask you about restoring previous tabs.
- Connections picker
  - [ ] Verify that the connections picker shows new connections when ssh & db tabs are opened.
  - [ ] Check if those connections are available after the app restart.
  - [ ] Check that those connections are removed after you log out of the root cluster that they
        belong to.
  - [ ] Verify that reopening a db connection from the connections picker remembers last used port.
- Cluster resources
  - [ ] Verify that the app shows the same resources as the Web UI.
  - [ ] Verify that search is working for the resources list.
  - [ ] Verify that pagination is working for the resources list.
  - [ ] Verify that search results are paginated too.
  - [ ] Verify that you can connect to these resources.
    - Verify that this works on:
      - [ ] macOS
      - [ ] Windows
      - [ ] Linux
  - [ ] Verify that clicking "Connect" shows available logins and db usernames.
    - Logins and db usernames are taken from the role, under `spec.allow.logins` and
      `spec.allow.db_users`.
  - [ ] Repeat the above steps for resources in leaf clusters.
- Tabs
  - [ ] Verify that tabs have correct titles set.
  - [ ] Verify that changing tab position works.
- Shortcuts
  - [ ] Verify that switching between tabs works on `Cmd+[1...9]`.
  - [ ] Verify that other shortcuts are shown after you close all tabs.
  - [ ] Verify that the other shortcuts work and each of them is shown on hover on relevant UI
        elements.
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux
- Workspaces & cluster management
  - [ ] Verify that logging in to a new cluster adds it to the identity switcher and switches to
        the workspace of that cluster automatically.
  - [ ] Verify that the state of the current workspace is preserved when you change the workspace
        (by switching to another cluster) and return to the previous workspace.
  - [ ] Click "Add another cluster", provide an address to a cluster that was already added. Verify
        that Connect simply changes the workspace to that of that cluster.
  - [ ] Click "Add another cluster", provide an address to a new cluster and submit the form. Close
        the modal when asked for credentials. Verify that the cluster was still added and is visible in
        the profile selector.
- Search bar
  - [ ] Verify that you can connect to all three resources types on root clusters and leaf
        clusters.
  - [ ] Verify that picking a resource filter and a cluster filter at the same time works as
        expected.
  - [ ] Verify that connecting to a resource from a different root cluster switches to the
        workspace of that root cluster.
  - Shut down a root cluster.
    - [ ] Verify that attempting to search returns "Some of the search results are incomplete" in
          the search bar.
    - [ ] Verify that clicking "Show details" next to the error message and then closing the modal
          by clicking one of the buttons or by pressing Escape does not close the search bar.
  - Log in as a user with a short TTL. Make sure you're not logged in to any other cluster. Wait for
    the cert to expire. Enter a search term that usually returns some results.
    - [ ] Relogin when asked. Verify that the search bar is not collapsed and shows search
          results.
    - [ ] Close the login modal instead of logging in. Verify that the search bar is not collapsed
          and shows "No matching results found".
- Resilience when resources become unavailable
  - DocumentCluster
    - For each scenario, create at least one DocumentCluster tab for each available resource kind.
    - For each scenario, first do the action described in the bullet point, then refetch list of
      resources by entering the search field and pressing enter. Verify that no unrecoverable
      error was raised (that is, the app still works). Then restart the app and verify that it was
      restarted gracefully (no unrecoverable error on restart, the user can continue using the
      app).
      - [ ] Stop the root cluster.
      - [ ] Stop a leaf cluster.
      - [ ] Disconnect your device from the internet.
  - DocumentGateway
    - [ ] Verify that you can't open more than one tab for the same db server + username pair.
          Trying to open a second tab with the same pair should just switch you to the already
          existing tab.
    - [ ] Create a db connection tab for a given database. Then remove access to that db for that
          user. Go back to Connect and change the database name and port. Both actions should not
          return an error.
    - [ ] Open DocumentCluster and make sure a given db is visible on the list of available dbs.
          Click "Connect" to show a list of db users. Now remove access to that db. Go back to Connect
          and choose a username. Verify that a recoverable error is shown and the user can continue
          using the app.
    - [ ] Create a db connection, close the app, run `tsh proxy db` with the same port, start the
          app. Verify that the app doesn't crash and the db connection tab shows you the error
          (address in use) and offers a way to retry creating the connection.
      - Verify that this works on:
        - [ ] macOS
        - [ ] Windows
        - [ ] Linux
- File transfer
  - Download
    - [ ] Verify if Connect asks for a path when downloading the file.
    - [ ] Verify that invalid paths and network errors are handled.
    - [ ] Verify if cancelling the download works.
  - Upload
    - [ ] Verify if uploading single/multiple files works.
    - [ ] Verify that invalid paths and network errors are handled.
    - [ ] Verify if cancelling the upload works.
- Refreshing certs
  - To test scenarios from this section, create a user with a role that has TTL of `1m`
    (`spec.options.max_session_ttl`).
  - Log in, create a db connection and run the CLI command; wait for the cert to expire, make
    another connection to the local db proxy.
    - [ ] Verify that the window received focus and a modal login is shown.
      - Verify that this works on:
        - [ ] macOS
        - [ ] Windows
        - [ ] Linux
    - Verify that after successfully logging in:
      - [ ] The cluster info is synced.
      - [ ] The first connection wasn't dropped; try executing `select now();`, the client should
            be able to automatically reinstantiate the connection.
      - [ ] The database proxy is able to handle new connections; click "Run" in the db tab and
            see if it connects without problems. You might need to resync the cluster again in case
            they managed to expire.
    - [ ] Verify that closing the login modal without logging in shows an appropriate error.
  - Log in, create a db connection, then remove access to that db server for that user; wait for
    the cert to expire, then attempt to make a connection through the proxy; log in.
    - [ ] Verify that psql shows an appropriate access denied error ("access to db denied. User
          does not have permissions. Confirm database user and name").
  - Log in, open a cluster tab, wait for the cert to expire. Switch from a servers view to
    databases view.
    - [ ] Verify that a login modal was shown.
    - [ ] Verify that after logging in, the database list is shown.
  - Log in, set up two db connections. Wait for the cert to expire. Attempt to connect to the first
    proxy, then without logging in proceed to connect to the second proxy.
    - [ ] Verify that an error notification was shown related to another login attempt being in
          progress.
- Access Requests
  - **Creating Access Requests (Role Based)**
    - To setup a test environment, follow the steps laid out in `Creating Access Requests (Role Based)` from the Web UI testplan and then verify the tasks below.
    - [ ] Verify that under requestable roles, only `allow-roles-and-nodes` and
    `allow-users-with-short-ttl` are listed
    - [ ] Verify you can select/input/modify reviewers
    - [ ] Verify you can view the request you created from request list (should be in a pending
    state)
    - [ ] Verify there is list of reviewers you selected (empty list if none selected AND
    suggested_reviewers wasn't defined)
    - [ ] Verify you can't review own requests
  - **Creating Access Requests (Search Based)**
    - To setup a test environment, follow the steps laid out in `Creating Access Requests (Resource Based)` from the Web UI testplan and then verify the tasks below.
    - [ ] Verify that a user can see resources based on the `searcheable-resources` rules
    - [ ] Verify you can select/input/modify reviewers
    - [ ] Verify you can view the request you created from request list (should be in a pending
    state)
    - [ ] Verify there is list of reviewers you selected (empty list if none selected AND
    suggested_reviewers wasn't defined)
    - [ ] Verify you can't review own requests
    - [ ] Verify that you can mix adding resources from the root and leaf clusters.
    - [ ] Verify that you can't mix roles and resources into the same request.
    - [ ] Verify that you can request resources from both the unified view and the search bar.
    - Change `show_resources` to `accessible_only` in [the UI config](https://goteleport.com/docs/reference/resources/#ui-config) of the root cluster.
      - [ ] Verify that you can now only request resources from the new request tab.
  - **Viewing & Approving/Denying Requests**
    - To setup a test environment, follow the steps laid out in `Viewing & Approving/Denying Requests` from the Web UI testplan and then verify the tasks below.
    - [ ] Verify you can view access request from request list
    - [ ] Verify you can approve a request with message, and immediately see updated state with
          your review stamp (green checkmark) and message box
    - [ ] Verify you can deny a request, and immediately see updated state with your review stamp
          (red cross)
    - [ ] Verify deleting the denied request is removed from list
  - **Assuming Approved Requests (Role Based)**
    - [ ] Verify that assuming `allow-roles-and-nodes` allows you to see roles screen and ssh into
          nodes
    - [ ] After assuming `allow-roles-and-nodes`, verify that assuming `allow-users-with-short-ttl`
          allows you to see users screen, and denies access to nodes
    - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it
          expires
    - [ ] Verify `switching back` goes back to your default static role
    - [ ] Verify after re-assuming `allow-users-with-short-ttl` role, the user is automatically logged
          out after the expiry is met (4 minutes)
  - **Assuming Approved Requests (Search Based)**
    - [ ] Verify that assuming approved request, allows you to see the resources you've requested.
  - **Assuming Approved Requests (Both)**
    - [ ] Verify assume buttons are only present for approved request and for logged in user
    - [ ] Verify that after clicking on the assume button, it is disabled in both the list and in
          viewing
    - [ ] Verify that after re-login, requests that are not expired and are approved are assumable
          again
- Configuration
  - [ ] Verify that clicking on More Options icon `⋮` > Open Config File opens the `app_config.json` file in your editor.
    - Verify that this works on:
      - [ ] macOS
      - [ ] Windows
      - [ ] Linux
  - Change a config property and restart the app. Verify that the change has been applied.
    - [ ] Change a keyboard shortcut.
    - [ ] Change `terminal.fontFamily`.
  - Provide the same keyboard shortcut for two actions.
    - [ ] Verify that a notification is displayed saying that a duplicate shortcut was found.
  - Provide an invalid value for some property (for example, set `"keymap.tab1": "ABC"`).
    - [ ] Verify that a notification is displayed saying that the property has an invalid value.
  - Make a syntax error in the file (for example, set `"keymap.tab1": not a string`).
    - [ ] Verify that a notification is displayed saying that the config file was not loaded correctly.
    - [ ] Verify that your config changes were not overridden.
- Headless auth
  - Headless auth modal in Connect can be triggered by calling `tsh ls --headless --user=<username> --proxy=<proxy>`. The cluster needs to have webauthn enabled for it to work.
  - [ ] Verify the basic operations (approve, reject, ignore then accept in the Web UI).
  - [ ] Make a headless request then cancel the command. Verify that the modal in Connect was
        closed automatically.
  - [ ] Make a headless request then accept it in the Web UI. Verify that the modal in Connect was
        closed automatically.
  - [ ] Make two concurrent headless requests for the same cluster. Verify that Connect shows the
        second one after closing the modal for the first request.
  - [ ] Make two concurrent headless requests for two different clusters. Verify that Connect shows
        the second one after closing the modal for the first request.
- Per-session MFA
  - The easiest way to test it is to enable [cluster-wide per-session
    MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa/#cluster-wide).
  - [ ] Verify that connecting to a Kube cluster prompts for MFA.
    - [ ] Re-execute `kubectl exec --stdin --tty shell-demo -- /bin/bash` mentioned above to
          verify that Kube access is working with MFA.
  - [ ] Verify that Connect prompts for MFA during Connect My Computer setup.
- Hardware key support
  - You will need a YubiKey 4.3+ and Teleport Enterprise. 
    The easiest way to test it is to enable [cluster-wide hardware keys enforcement](https://goteleport.com/docs/admin-guides/access-controls/guides/hardware-key-support/#step-12-enforce-hardware-key-support)
    (set `require_session_mfa: hardware_key_touch_and_pin` to get both touch and PIN prompts).
  - [ ] Log in. Verify that you were asked for both PIN and touch.
  - [ ] Connect to a database. Verify you were prompted for touch (a PIN prompt can appear too).
  - [ ] Change the PIN (leave the PIV PIN field empty during login to access this flow).
  - [ ] Close the app, disconnect the YubiKey, then reopen the app. Verify the app shows an error about the missing key.
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux

- Connect My Computer
  - [ ] Verify the happy path from clean slate (no existing role) setup: set up the node and then
        connect to it.
  - Kill the agent while its joining the cluster and verify that the logs from the agent process
    are shown in the UI.
    - The easiest way to do this is by following the agent cleanup daemon logs (`tail -F ~/Library/Application\ Support/Teleport\ Connect/logs/cleanup.log`) and then `kill -s KILL <agent PID>`.
    - [ ] During setup.
    - [ ] After setup in the status view. Verify that the page says that the process exited with
          SIGKILL.
  - [ ] Open the node config, change the proxy address to an incorrect one to simulate problems
        with connection. Verify that the app kills the agent after the agent is not able to join the
        cluster within the timeout.
  - [ ] Verify autostart behavior. The agent should automatically start on app start unless it was
        manually stopped before exiting the app.
  - Verify that all items from this section work on:
    - [ ] macOS
    - [ ] Windows
    - [ ] Linux
- VNet
  - VNet doesn't work with local clusters made available under custom domains through entries in
    `/etc/hosts`. It's best to use a "real" cluster. nip.io might work, but it hasn't been confirmed
    yet.
  - Verify that VNet works for TCP apps within:
    - [ ] a root cluster
    - [ ] [a custom DNS zone](https://goteleport.com/docs/application-access/guides/vnet/) of a root cluster
    - [ ] a leaf cluster
    - [ ] a custom DNS zone of a leaf cluster
  - [ ] Verify that setting [a custom IPv4 CIDR range](https://goteleport.com/docs/application-access/guides/vnet/#configuring-ipv4-cidr-range) works.
  - [ ] Verify that Connect asks for relogin when attempting to connect to an app after cert expires.
    - Be mindful that you need to connect to the app at least once before the cert expires for
      Connect to properly recognize it as a TCP app.
  - Start VNet, then stop it.
    - [ ] Verify that the VNet panel doesn't show any errors related to VNet being stopped.
  - Start VNet. While its running, kill the admin process.
    - The easiest way to find the PID of the admin process is to open Activity Monitor, View →
      All Processes, Hierarchically, search for `tsh` and find tsh running under kernel_task →
      launchd → tsh, owned by root. Then just `sudo kill -s KILL <tsh pid>`.
    - [ ] Verify that the admin process _leaves_ files in `/etc/resolver`. However, it's possible to
      start VNet again, connect to a TCP app, then shut VNet down and it results in the files being
      cleaned up.
  - [ ] Start VNet in a clean macOS VM. Verify that on the first VNet start, macOS shows the prompt
    for enabling the background item for tsh.app. Accept it and verify that you can connect to a TCP
    app through VNet.
- Misc
  - [ ] Verify that logs are collected for all processes (main, renderer, shared, tshd) under
        `~/Library/Application\ Support/Teleport\ Connect/logs`.
  - [ ] Verify that the password from the login form is not saved in the renderer log.
  - [ ] Log in to a cluster, then log out and log in again as a different user. Verify that the app
        works properly after that.
  - [ ] Clean the Application Support dir for Connect. Start the latest stable version of the app.
        Open every possible document. Close the app. Start the current alpha. Reopen the tabs. Verify that
        the app was able to reopen the tabs without any errors.
