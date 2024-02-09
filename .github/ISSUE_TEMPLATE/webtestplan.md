---
name: Web Test Plan
about: Web UI manual test plan for Teleport major releases
title: "Teleport Web Test Plan"
labels: testplan
---

## Web UI

## Main
For main, test with a role that has access to all resources.

- [ ] Interact with a cluster using the Web UI
  - [ ] Connect to a Teleport node
  - [ ] Connect to a OpenSSH node
  - [ ] Check agent forwarding is correct based on role and proxy mode.
     - Set `forward_agent: true` under the `options` section of your role, and then test that your
       teleport certs show up when you run `ssh-add -l` on the node.

#### Top Nav
- [ ] Verify that cluster selector displays all (root + leaf) clusters
- [ ] Verify that user name is displayed
- [ ] Verify that user menu shows logout, help&support, and account settings (for local users)

#### Side Nav
- [ ] Verify that each item has an icon
- [ ] Verify that Collapse/Expand works and collapsed has icon `>`, and expand has icon `v`
- [ ] Verify that it automatically expands and highlights the item on page refresh

### Unified Resources
- [ ] Verify that scrolling to the bottom of the page renders more resources
- [ ] Verify that all resource types are visible if no filters are present
- [ ] Verify that "Search" by (host)name, address, labels works for all resources
#### Servers aka Nodes
- [ ] Verify that "Servers" type shows all joined nodes
- [ ] Verify that "Connect" button shows a list of available logins
- [ ] Verify that terminal opens when clicking on one of the available logins
- [ ] Verify that clicking on `Add Resource` button correctly sends to the resource discovery page
#### Applications
- [ ] Verify that the app icons are correctly displayed
- [ ] Verify that filtering types by Application includes applications in the page
- [ ] Verify that the `Launch` button for applications correctly send to the app
- [ ] Verify that the `Launch` button for AWS apps correctly renders an IAM role selection window 
#### Databases
- [ ] Verify that the database subtype icons are correctly displayed
- [ ] Verify that filtering types by Databases includes databases in the page
- [ ] Verify that clicking `Connect` renders the dialog with correct information

#### Kubes
- [ ] Verify that filtering types by Kubes includes kubes in the page
- [ ] Verify that clicking `Connect` renders the dialog with correct information

#### Desktops
- [ ] Verify that filtering types by Desktops includes desktops in the page
- [ ] Verify that clicking `Connect` renders a login selection and that the logins are completely in view
#### Active Sessions
- [ ] Verify that "empty" state is handled
- [ ] Verify that it displays the session when session is active
- [ ] Verify that "Description", "Session ID", "Users", "Nodes" and "Duration" columns show correct values
- [ ] Verify that "OPTIONS" button allows to join a session

#### Audit log
- [ ] Verify that time range button is shown and works
- [ ] Verify that clicking on `Session Ended` event icon, takes user to session player
- [ ] Verify event detail dialogue renders when clicking on events `details` button
- [ ] Verify searching by type, description, created works


#### Users
- [ ] Verify that users are shown
- [ ] Verify that creating a new user works
- [ ] Verify that editing user roles works
- [ ] Verify that removing a user works
- [ ] Verify resetting a user's password works
- [ ] Verify search by username, roles, and type works

#### Auth Connectors
For help with setting up auth connectors, check out the [Quick GitHub/SAML/OIDC Setup Tips]

- [ ] Verify when there are no connectors, empty state renders
- [ ] Verify that creating OIDC/SAML/GITHUB connectors works
- [ ] Verify that editing  OIDC/SAML/GITHUB connectors works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text is shown on the right side
- [ ] Verify that encrypted SAML assertions work with an identity provider that supports it (Azure).
- [ ] Verify that created GitHub, saml, oidc card has their icons
#### Roles
- [ ] Verify that roles are shown
- [ ] Verify that "Create New Role" dialog works
- [ ] Verify that deleting and editing works
- [ ] Verify that error is shown when saving an invalid YAML
- [ ] Verify that correct hint text is shown on the right side

#### Managed Clusters
- [ ] Verify that it displays a list of clusters (root + leaf)
- [ ] Verify that every menu item works: nodes, apps, audit events, session recordings, etc.

#### Help & Support
- [ ] Verify that all URLs work and correct (no 404)

## Access Requests

Access Request is a Enterprise feature and is not available for OSS.

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

### Creating Access Requests (Search Based)
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
      - searcheable resources
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
- [ ] After assuming `allow-roles-and-nodes`, verify that assuming `allow-users-short-ttl` allows you to see users screen, and denies access to nodes
  - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it expires
  - [ ] Verify `switching back` goes back to your default static role
  - [ ] Verify after re-assuming `allow-users-short-ttl` role, the user is automatically logged out after the expiry is met (4 minutes)

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

## Terminal
- [ ] Verify that top nav has a user menu (Main and Logout)
- [ ] Verify that switching between tabs works with `ctrl+[1...9]` (alt on linux/windows)

#### Node List Tab
- [ ] Verify that Cluster selector works (URL should change too)
- [ ] Verify that "Connect" button shows a list of available logins
- [ ] Verify that "Hostname", "Address" and "Labels" columns show the current values
- [ ] Verify that "Search" by hostname, address, labels work
- [ ] Verify that new tab is created when starting a session

#### Session Tab
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

## Session Player
- [ ] Verify that it can replay a session
- [ ] Verify that when playing, scroller auto scrolls to bottom most content
- [ ] Verify when resizing player to a small screen, scroller appears and is working
- [ ] Verify that error message is displayed (enter an invalid SID in the URL)

## Invite and Reset Form
- [ ] Verify that input validates
- [ ] Verify that invite works with 2FA disabled
- [ ] Verify that invite works with OTP enabled
- [ ] Verify that invite works with U2F enabled
- [ ] Verify that invite works with WebAuthn enabled
- [ ] Verify that error message is shown if an invite is expired/invalid

## Login Form and Change Password
- [ ] Verify that input validates
- [ ] Verify that login works with 2FA disabled
- [ ] Verify that changing passwords works for 2FA disabled
- [ ] Verify that login works with OTP enabled
- [ ] Verify that changing passwords works for OTP enabled
- [ ] Verify that login works with U2F enabled
- [ ] Verify that changing passwords works for U2F enabled
- [ ] Verify that login works with WebAuthn enabled
- [ ] Verify that changing passwords works for WebAuthn enabled
- [ ] Verify that login works for Github/SAML/OIDC
- [ ] Verify that redirect to original URL works after successful login
- [ ] Verify that account is locked after several unsuccessful login attempts
- [ ] Verify that account is locked after several unsuccessful change password attempts

## Multi-factor Authentication (mfa)
Create/modify `teleport.yaml` and set the following authentication settings under `auth_service`

```yaml
authentication:
  type: local
  second_factor: optional
  require_session_mfa: yes
  webauthn:
    rp_id: example.com
```

#### MFA invite, login, password reset, change password
- [ ] Verify during invite/reset, second factor list all auth types: none, hardware key, and authenticator app
- [ ] Verify registration works with all option types
- [ ] Verify login with all option types
- [ ] Verify changing password with all option types
- [ ] Change `second_factor` type to `on` and verify that mfa is required (no option `none` in dropdown)

#### MFA require auth
Go to `Account Settings` > `Two-Factor Devices` and register a new device

Using the same user as above:
- [ ] Verify logging in with registered WebAuthn key works
- [ ] Verify connecting to a ssh node prompts you to tap your registered WebAuthn key
- [ ] Verify in the web terminal, you can scp upload/download files

#### MFA Management

- [ ] Verify adding first device works without requiring re-authentication
- [ ] Verify re-authenticating with a WebAuthn device works
- [ ] Verify re-authenticating with a U2F device works
- [ ] Verify re-authenticating with a OTP device works
- [ ] Verify adding a WebAuthn device works
- [ ] Verify adding a U2F device works
- [ ] Verify adding an OTP device works
- [ ] Verify removing a device works
- [ ] Verify `second_factor` set to `off` disables adding devices

#### Passwordless

- [ ] Pure passwordless registrations and resets are possible
- [ ] Verify adding a passwordless device (WebAuthn)
- [ ] Verify passwordless logins

## Cloud
From your cloud staging account, change the field `teleportVersion` to the test version.
```
$ kubectl -n <namespace> edit tenant
```

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
- [ ] Verify receiving email for locked account when max attempts reached

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
- [ ] Verify that a user has access only to: "Servers", "Applications", "Databases", "Kubernetes", "Active Sessions", "Access Requests" and "Manage Clusters"
- [ ] Verify there is no `Add Server, Application, Databases, Kubernetes` button in each respective view
- [ ] Verify only `Servers`, `Apps`, `Databases`, and `Kubernetes` are listed under `options` button in `Manage Clusters`

Note: User has read/create access_request access to their own requests, despite resource settings

Add the following under `spec.allow.rules` to enable read access to the audit log:
```
  - resources:
      - event
      verbs:
      - list
```
- [ ] Verify that the `Audit Log` and `Session Recordings` is accessible
- [ ] Verify that playing a recorded session is denied

Add the following to enable read access to recorded sessions
```
  - resources:
      - session
      verbs:
      - read
```
- [ ] Verify that a user can re-play a session (session.end)

Add the following to enable read access to the roles

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
- [ ] Verify that a user can access the "Trust" screen
- [ ] Verify that a user cannot create/delete/update a trusted cluster.

## Locks
Checking that you can view, create, and delete locks.

- [ ] Existing locks listing page.
  - [ ] It lists all of the existing locks in the system.
  - [ ] Locks without a `Locked By` and `Start Date` are still shown with those fields empty.
  - [ ] Clicking the trash can deletes the lock with a spinner.
  - [ ] Table columns are sortable.
  - [ ] Table search field filters the results.
- [ ] Adding a new lock. (+ Add New Lock).
  - [ ] Target switcher shows the locks for the various target types (User, Role, Login, Node, MFA Device, Windows Desktop, Access Request).
  - [ ] Target switcher has "Access Request" in E build but not in OSS.
  - [ ] You can add lock targets from multiple target types.
  - [ ] Adding a target disables that "add button".
  - [ ] You cannot proceed if you haven't selected targets to lock.
  - [ ] You can clear the selected targets prior to creating locks.
  - [ ] Proceeding to lock opens an animated slide panel from the right.
  - [ ] You can remove lock targets from the slide panel.
  - [ ] Creating a lock with message and TTL correctly create the lock.
  - [ ] Create a lock without message and TTL, they should be optional.

## Enroll new resources using Discover Wizard
Use Discover Wizard to enroll new resources and access them:

- [ ] SSH Server
- [ ] Self-Hosted PostgreSQL
- [ ] AWS RDS PostgreSQL
- [ ] Kubernetes
- [ ] AWS EKS cluster
- [ ] Windows Desktop Active Directory

## Teleport Connect

- Auth methods
   - Verify that the app supports clusters using different auth settings
     (`auth_service.authentication` in the cluster config):
      - [ ] `type: local`, `second_factor: "off"`
      - [ ] `type: local`, `second_factor: "otp"`
         - [ ] Test per-session MFA items listed later in the test plan.
      - [ ] `type: local`, `second_factor: "webauthn"`,
         - [ ] Test per-session MFA items listed later in the test plan.
      - [ ] `type: local`, `second_factor: "webauthn"`, log in passwordlessly with hardware key
      - [ ] `type: local`, `second_factor: "webauthn"`, log in passwordlessly with touch ID
      - [ ] `type: local`, `second_factor: "optional"`, log in without MFA
      - [ ] `type: local`, `second_factor: "optional"`, log in with OTP
      - [ ] `type: local`, `second_factor: "optional"`, log in with hardware key
      - [ ] `type: local`, `second_factor: "on"`, log in with OTP
         - [ ] Test per-session MFA items listed later in the test plan.
      - [ ] `type: local`, `second_factor: "on"`, log in with hardware key
      - [ ] `type: local`, `second_factor: "on"`, log in with passwordless auth
      - [ ] Verify that the passwordless credential picker works.
         - To make the picker show up, you need to add the same MFA device with passwordless
           capabilities to multiple users.
      - [Authentication connectors](https://goteleport.com/docs/setup/reference/authentication/#authentication-connectors):
         - For those you might want to use clusters that are deployed on the web, specified in
           parens. Or set up the connectors on a local enterprise cluster following [the guide from
           our wiki](https://gravitational.slab.com/posts/quick-git-hub-saml-oidc-setup-6dfp292a).
         - [ ] GitHub (asteroid)
            - [ ] local login on a GitHub-enabled cluster
         - [ ] SAML (platform cluster)
         - [ ] OIDC (e-demo)
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
- State restoration from disk
   - [ ] Verify that the app asks about restoring previous tabs when launched and restores them
         properly.
   - [ ] Verify that the app opens with the cluster that was active when you closed the app.
   - [ ] Verify that the app remembers size & position after restart.
   - [ ] Verify that [reopening a cluster that has no workspace
     assigned](https://github.com/gravitational/webapps.e/issues/275#issuecomment-1131663575) works.
   - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport
     Connect/tsh` doesn't crash the app.
   - [ ] Verify that reopening the app after removing `~/Library/Application Support/Teleport
     Connect/app_state.json` but not the `tsh` dir doesn't crash the app.
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
- Cluster resources (servers, databases, k8s, apps)
   - [ ] Verify that the app shows the same resources as the Web UI.
   - [ ] Verify that search is working for the resources list.
   - [ ] Verify that pagination is working for the resources list.
   - [ ] Verify that pagination works in tandem with search, that is verify that search results are
         paginated too.
   - [ ] Verify that you can connect to these resources.
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
      - To setup a test environment, follow the steps laid out in `Created Access Requests (Role
        Based)` from the Web UI testplan and then verify the tasks below.
      - [ ] Verify that under requestable roles, only `allow-roles-and-nodes` and
        `allow-users-with-short-ttl` are listed
      - [ ] Verify you can select/input/modify reviewers
      - [ ] Verify you can view the request you created from request list (should be in a pending
        state)
      - [ ] Verify there is list of reviewers you selected (empty list if none selected AND
        suggested_reviewers wasn't defined)
      - [ ] Verify you can't review own requests
   - **Creating Access Requests (Search Based)**
      - To setup a test environment, follow the steps laid out in `Created Access Requests (Search
        Based)` from the Web UI testplan and then verify the tasks below.
      - [ ] Verify that a user can see resources based on the `searcheable-resources` rules
      - [ ] Verify you can select/input/modify reviewers
      - [ ] Verify you can view the request you created from request list (should be in a pending
        state)
      - [ ] Verify there is list of reviewers you selected (empty list if none selected AND
        suggested_reviewers wasn't defined)
      - [ ] Verify you can't review own requests
      - [ ] Verify that you can't mix adding resources from different clusters (there should be a
        warning dialogue that clears the selected list)
      - [ ] Verify that you can't mix roles and resources into the same request.
   - **Viewing & Approving/Denying Requests**
      - To setup a test environment, follow the steps laid out in `Viewing & Approving/Denying
        Requests` from the Web UI testplan and then verify the tasks below.
      - [ ] Verify you can view access request from request list
      - [ ] Verify you can approve a request with message, and immediately see updated state with
        your review stamp (green checkmark) and message box
      - [ ] Verify you can deny a request, and immediately see updated state with your review stamp
        (red cross)
      - [ ] Verify deleting the denied request is removed from list
   - **Assuming Approved Requests (Role Based)**
      - [ ] Verify that assuming `allow-roles-and-nodes` allows you to see roles screen and ssh into
        nodes
      - [ ] After assuming `allow-roles-and-nodes`, verify that assuming `allow-users-short-ttl`
        allows you to see users screen, and denies access to nodes
      - [ ] Verify a switchback banner is rendered with roles assumed, and count down of when it
        expires
      - [ ] Verify `switching back` goes back to your default static role
      - [ ] Verify after re-assuming `allow-users-short-ttl` role, the user is automatically logged
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
   - Headless auth modal in Connect can be triggered by calling `tsh ls --headless --user=<username>
     --proxy=<proxy>`. The cluster needs to have webauthn enabled for it to work.
   - [ ] Verify the basic operations (approve, reject, ignore then accept in the Web UI).
   - [ ] Make a headless request then cancel the command. Verify that the modal in Connect was
     closed automatically.
   - [ ] Make a headless request then accept it in the Web UI. Verify that the modal in Connect was
     closed automatically.
   - [ ] Make two concurrent headless requests for the same cluster. Verify that Connect shows the
     second one after closing the modal for the first request.
   - [ ] Make two concurrent headless requests for two different clusters. Verify that Connect shows
     the second one after closing the modal for the first request.
- tshd-initiated communication
   - [ ] Create a db connection, wait for the cert to expire. Attempt to connect to the database
     through CLI. While the login modal is shown, make a headless request. Verify that after logging
     in again, the app shows the modal for the headless request.
- Per-session MFA
   - The easiest way to test it is to enable [cluster-wide per-session
     MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa/#cluster-wide).
   - [ ] Verify that connecting to a Kube cluster prompts for MFA.
      - [ ] Re-execute `kubectl exec --stdin --tty shell-demo -- /bin/bash` mentioned above to
        verify that Kube access is working with MFA.
   - [ ] Verify that Connect prompts for MFA during Connect My Computer setup.
- Connect My Computer
   - [ ] Verify the happy path from clean slate (no existing role) setup: set up the node and then
     connect to it.
   - Kill the agent while its joining the cluster and verify that the logs from the agent process
     are shown in the UI.
      - The easiest way to do this is by following the agent cleanup daemon logs (`tail -F ~/Library/Application\ Support/Teleport\
        Connect/logs/cleanup.log`) and then `kill -s KILL <agent PID>`.
      - [ ] During setup.
      - [ ] After setup in the status view. Verify that the page says that the process exited with
        SIGKILL.
   - [ ] Open the node config, change the proxy address to an incorrect one to simulate problems
     with connection. Verify that the app kills the agent after the agent is not able to join the
     cluster within the timeout.
   - [ ] Verify autostart behavior. The agent should automatically start on app start unless it was
     manually stopped before exiting the app.
- [ ] Verify that logs are collected for all processes (main, renderer, shared, tshd) under
  `~/Library/Application\ Support/Teleport\ Connect/logs`.
- [ ] Verify that the password from the login form is not saved in the renderer log.
- [ ] Log in to a cluster, then log out and log in again as a different user. Verify that the app
  works properly after that.
- [ ] Clean the Application Support dir for Connect. Start the latest stable version of the app.
  Open every possible document. Close the app. Start the current alpha. Reopen the tabs. Verify that
  the app was able to reopen the tabs without any errors.
