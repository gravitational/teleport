---
authors: Michael Wilson (michael.wilson@goteleport.com)
state: draft
---

# RFD 124 - Plugin static credentials

### Required Approvers

* Engineering @r0mant, @justinas
* Security @reed
* Product: @klizhentas

## What

Allow storage of plugin static credentials within Teleport in a safe and secure way.

## Why

As we grow our plugin support, we have discovered that the various services we
hope to integrate do not offer a uniform mechanism for configuration. For instance,
Slack uses oauth2, but Okta requires an API token, and Jamf requires a username and
password.

Today, the plugin houses a credentials object that allows for this sort of storage.
However, unlike an oauth authorization code, static credentials pose a much greater risk of
exposure, and rotating credentials can be a time consuming process.  To reduce the risks associated
with storing static credentials within Teleport and to allow for easier credentials rotation,
we would like to create a new resource within Teleport that is handled in an explicitly secure
way.

## Details

### The `PluginStaticCredentials` object

A new object will be needed to house static credentials. At present it will support:

- API tokens
- Usernames and passwords
- OAuth client ID and secret

This object has very strict access control requirements that will be explained in detail
in the **Access Control** section. On creation, the plugin and static credentials will
receive a UUID called the `plugin-label`. The static credentials will have an internal
label that receives this value. Additionally, one plugin may have _many_ credentials
for the purposes of credentials rotation.

```yaml
kind: plugin_static_credentials
version: v1
metadata:
  name: credentials-name
  labels:
    # This is a unique plugin label generated randomly at plugin creation.
    # It cannot be modified by the user.
    teleport.internal/plugin: 48398071-9860-4be6-9f29-ca6ca5bc7155
    # additional labels to uniquely identify the credentials.
    label1: value1
    type: slack
spec:
  credentials:
    # Only one of these credential types must be defined at a time. All values in this
    # are string literals and not file locations. None of the existing credentials objects
    # will be used for this, and all new objects will be created.
    api_token: example-token
    basic_auth:
      username: example-user
      password: example-password
    oauth_client_secret:
      client_id: example-client-id
      client_secret: example-client-secret
```

If a `Plugin` is using the oauth authorization code flow, however, it will not need any
`PluginStaticCredentials` objects.

### Changes to the `Plugin` object

The `Plugin` object's `Credentials` field will have a new type of credential called
`CredentialsRef` that contains a reference to a static plugin label. More than
one static credential may match this label, in which case when starting the plugin, the most
recently created credential will be used.

```yaml
credentials_ref:
  labels:
    # This label is the same label generated at plugin creation. This field will be
    # injected by Teleport at creation.
    teleport.internal/plugin: 48398071-9860-4be6-9f29-ca6ca5bc7155
    env: prod
    type: slack
```

### Access control

`PluginStaticCredentials` can be written to and deleted by `RoleProxy`. At introduction,
creating credentials is a flow that will only happen through the web UI, so only `RoleProxy`
is needed. Future work may determine that we want to open up writing through the CLI.

The object will be only readable by the `RoleAdmin` role (with one exception).
This is because plugins run on the auth server, so it's the only role that needs to be
able to actually read the static credentials.

One `Plugin` object can refer to multiple static credentials and a `Plugin` can't refer
to existing static credentials. Static credentials must be associated with one and only
one plugin.

The one exception here is Teleport Assist, which runs in the proxy. The proxy will be
able to read plugin credentials with the name of `openai-default`, which is the name of
the Assist plugin.

### Lifecycle of the `PluginStaticCredentials` object

#### Creation

A `PluginStaticCredentials` object must be created at the same time that the `Plugin` is created.

#### Deletion

On deletion, the associated `Plugin` will be stopped and related `PluginStaticCredentials`
objects will be deleted when deleting its associated plugin.

### UX

#### Web UI

The web UI will remain largely the same with respect to entering credentials for specific
plugins. This will largely depend on the specific application.

### Audit events

A number of new audit events will be created as part of this effort:

| Event Name | Description |
|------------|-------------|
| `plugin.credentials.create` | Emitted when plugin credentials have been created. |
| `plugin.credentials.delete` | Emitted when plugin credentials have been deleted. |

### Security

* Only the auth server is able to read plugin credentials. The proxy will be able to create
them, and admins will be able to delete them.
* The credentials will be stored in DynamoDB, which is encrypted at rest. This will provide
a layer of protection for the stored credentials.
* The credentials will not be cached, so they won't be sitting in an in-memory cache to be
exfiltrated.

### Implementation plan

#### `PluginStaticCredentials` object and backend service

- The `PluginStaticCredentials` object
- The local service to manage the `PluginStaticCredentials` objects in the backend.

#### Modifications to the `Plugin` object

- A new `PluginStaticCredentialsRefs` type.
- `PluginStaticCredentials` objects are created during `Plugin` creation.
- `PluginStaticCredentials` objects are deleted during `Plugin` deletion.

#### Implement `PluginStaticCredentials` permissions

- gRPC methods in the auth service to support reading the
  `PluginStaticCredentials`, ensuring that reading can only be done by `RoleAdmin`
  and, for the specific case of Teleport Assist, `RoleProxy`.
- Any updates to RBAC to support the creation/deletion of the `PluginStaticCredentials`
  object.

#### Integrate `PluginStaticCredentials` with enterprise plugin manager

- Look up `PluginStaticCredentials` objects when starting plugins that require them in the
plugin manager.

#### Add audit events

- Add audit events for the creation and deletion of `PluginStaticCredentials`.