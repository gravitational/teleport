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
However, unlike oauth tokens, static credentials pose a much greater risk of exposure,
and rotating credentials can be a time consuming process.  To reduce the risks associated
with storing static credentials within Teleport and to allow for easier credentials rotation,
we would like to create a new resource within Teleport that is handled in an explicitly secure
way.

## Details

### The `PluginStaticCredentials` object

A new object will be needed to house static credentials. At present it will support:

- API tokens
- Usernames and passwords

It should be noted that this object should *only* be readable by the auth service to prevent
exfiltration of these credentials.

```yaml
kind: plugin_static_credentials
version: v1
metadata:
  name: plugin-name
spec:
  lastRotated: "2023-05-09T16:50:52.497874Z"
  credentials:
    # Only one of these credential types must be defined at a time.
    api_token: example-token
    basic_auth:
      username: example-user
      password: example-password
```

If a plugin requires static credentials, there is a 1-to-1 mapping between `Plugin`
objects and `PluginStaticCredentials` objects. If a `Plugin` is using oauth, however,
it will not need a `PluginStaticCredentials` and none will be created. The object will
inherit the same name as its associated plugin.

### Lifecycle of the `PluginStaticCredentials` object

#### Creation

A `PluginStaticCredentials` object can be created after a `Plugin` object has been
created. If a `Plugin` is marked as needing static credentials, the `Plugin` won't
start until these have been provided.

#### Rotation

A `PluginStaticCredentials` object can be updated, which we'll refer to as rotation.
When the credentils are rotated, the `Plugin` associated with the credentials will be
restarted with the new credentials.

#### Deletion

`PluginStaticCredentials` objects can be deleted. On deletion, the associated `Plugin`
will be stopped. Additionally, `PluginStaticCredentials` objects will be deleted when
deleting its associated plugin.

### UX

#### Web UI

The web UI will remain largely the same with respect to entering credentials for specific
plugins. This will largely depend on the specific application.

#### Teleport CLI

* Creation of plugin credentials will be done with `tctl plugins create plugin_static_credentials.yaml`.
* Rotation of plugin credentials will be done with `tctl plugins rotate plugin_static_credentials.yaml`
* Plugins can be deleted with `tctl plugins rm plugin_static_credentials/plugin-name`

### Audit events

A number of new audit events will be created as part of this effort:

| Event Name | Description |
|------------|-------------|
| `PLUGIN_CREDENTIALS_CREATED` | Emitted when plugin credentials have been created. |
| `PLUGIN_CREDENTIALS_ROTATED` | Emitted when plugin credentials have been rotated. |
| `PLUGIN_CREDENTIALS_DELETED` | Emitted when plugin credentials have been deleted. |

### Security

* Only the auth server is able to read plugin credentials. Users will be able to create
  credentials, rotate, and delete credentials, but not read them.

### Implementation plan

#### `PluginStaticCredentials` object

The new `PluginStaticCredentials` object should be created with any backend and gRPC modifications
required.

#### Implement `PluginStaticCredentials` permissions

Proper permissions for the `PluginStaticCredentials` objects should be added.

#### Implement `PluginStaticCredentials` CLI tooling

CLI tooling for the `PluginStaticCredentials` should be implemented.

#### Integrate `PluginStaticCredentials` with enterprise plugin manager

The plugin manager should be able to look up the `PluginStaticCredentials` objects as needed. This
may require some additional modifications to the `Plugin` object.

#### Add audit events

Audit events for the various user facing operations should be added.