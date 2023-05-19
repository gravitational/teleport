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
in the **Access Control** section.

```yaml
kind: plugin_static_credentials
version: v1
metadata:
  name: plugin-name # name must match the name of the plugin.
spec:
  last_rotated: "2023-05-09T16:50:52.497874Z"
  credentials:
    # Only one of these credential types must be defined at a time. All values in this
    # are string literals and not file locations.
    api_token: example-token
    basic_auth:
      username: example-user
      password: example-password
    oauth_client_secret:
      client_id: example-client-id
      client_secret: example-client-secret
```

If a plugin requires static credentials, there is a 1-to-1 mapping between `Plugin`
objects and `PluginStaticCredentials` objects. If a `Plugin` is using oauth, however,
it will not need a `PluginStaticCredentials` and none will be created. The object will
inherit the same name as its associated plugin.

### Changes to the `Plugin` object

The `Plugin` object's `Credentials` field will have a new type of credential called
`PluginNeedsStaticCredentials` with a single boolean `Enabled` flag. If this flag is set,
this will indicate to Teleport that the plugin must have a corresponding
`PluginNeedsStaticCredentials` object in order to start.

### Access control

`PluginStaticCredentials` can be written to and deleted by any user with RBAC permissions
to do so. For these operations, the object is straightforward and reflects the typical
way that Teleport resources are handled.

However, the object will be only readable by the `RoleAdmin` role (with one exception).
This is because plugins run on the auth server, so it's the only role that needs to be
able to actually read the static credentials.

The one exception here is Teleport Assist, which runs in the proxy. The proxy will be
able to read plugin credentials with the name of `openai-default`, which is the name of
the Assist plugin.

### Lifecycle of the `PluginStaticCredentials` object

#### Creation

A `PluginStaticCredentials` object can be created after a `Plugin` object has been
created. If a `Plugin` is marked as needing static credentials as described above, the
`Plugin` won't start until these have been provided.

When creating a plugin, Teleport will first perform a lookup to ensure that there is a
plugin with the same name as the new `PluginStaticCredentials` object being created
and that the plugin has `PluginNeedsStaticCredentials` with the `Enabled` flag set to
true. If this plugin doesn't exist, the creation of the `PluginStaticCredentials` object
will fail.

#### Deletion

On deletion, the associated `Plugin` will be stopped. Additionally, `PluginStaticCredentials`
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

* Only the auth server is able to read plugin credentials. Any user with proper RBAC permissions
  will be able to create and delete credentials, but not read them.

### Implementation plan

#### `PluginStaticCredentials` object and backend service

- The `PluginStaticCredentials` object
- The local service to manage the `PluginStaticCredentials` objects in the backend.

#### Modifications to the `Plugin` object

- A new `PluginNeedsStaticCredentials` type.
- Modify the `PluginStaticCredentials` local service to only allow creation of
  `PluginStaticCredentials` objects if this new credentials type is enabled for
  the corresponding `Plugin`.

#### Implement `PluginStaticCredentials` permissions

- gRPC methods in the auth service to support reading, writing, and deleting the
  `PluginStaticCredentials`, ensuring that reading can only be done by `RoleAdmin`
  and, for the specific case of Teleport Assist, `RoleProxy`.
- Any updates to RBAC to support the creation/deletion of the `PluginStaticCredentials`
  object.

#### Integrate `PluginStaticCredentials` with enterprise plugin manager

- Only start plugins with a credentials type of `PluginNeedsStaticCredentials` if there's
  a corresponding `PluginStaticCredentials` object to go along with it.
- Monitor the creation/deletion of static credentials objects and trigger plugin start/stop if the
  credentials have been removed for a given plugin.

#### Add audit events

- Add audit events for the creation and deletion of `PluginStaticCredentials`.