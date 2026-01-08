---
authors: Grzegorz Zdunek (grzegorz.zdunek@goteleport.com)
state: draft
---

# RFD 0242 - Improve Windows installation experience

## Required Approvers

- Engineering: @ravicious && @avatus && @zmb3

## What

1. Provide an installation path that does not require elevated privileges, ensuring users in restricted environments
   can install Teleport Connect.
2. Transition automatic updates to a background process that requires administrative privileges only during the initial
   per-machine setup, rather than prompting users for credentials on every subsequent version release.

## Why

When we shipped VNet for Windows in early FY2026, the installer for Teleport Connect was switched from a per-user
install (which does not require admin permissions) to a per-machine install (which does require admin permissions).
This has created friction for users without local admin rights.

The application was moved to a per-machine installation to support the VNet system service (`TeleportVNetService`
implemented by `tsh.exe vnet-service`). This service manages the TUN device configuration and related networking tasks.

To prevent privilege escalation, the service binary must reside in a secure directory that grants write access only to
administrators. The per-machine installation path (`C:\Program Files\Teleport Connect\bin\tsh.exe`) was ideal for this
purpose.
Furthermore, because the per-machine installer requests elevated privileges during setup, it can register the service
with the Service Control Manager (SCM) seamlessly, without requiring additional user prompts.

## Dual Mode Installer

To accommodate standard users who lack the permissions to write to Program Files and install system services,
the app will provide a dual-mode installer:

|                                           | Per-User                            | Per-Machine                |
|-------------------------------------------|-------------------------------------|----------------------------|
| Install directory                         | %LocalAppData%                      | %ProgramFiles%             |
| Elevation required                        | No (Standard user)                  | Yes (Administrator/UAC)    | 
| VNet support                              | Disabled                            | Enabled                    |
| Update mechanism                          | Default user-space process          | Privileged Windows service |
| System registry hive for updater settings | HKCU / HKLM (HKLM takes precedence) | HKLM                       |

### Per-User Mode

We will reintroduce the unprivileged installation path for users who do not need VNet functionality.
By setting `oneClick: false`, `perMachine: false`, and `selectPerMachineByDefault: true` in the `electron-builder`
configuration, users can choose between "Install for me" (unprivileged) and "Install for all users" (privileged).
The "Install for all users" option will be preselected, so users will end up with VNet support by default.

> Note: IT admins generally don't like apps that install to `%LocalAppData%`, [since they are more difficult to manage
> through centralized management tools](https://old.reddit.com/r/sysadmin/comments/ea96ut/apps_installing_to_appdata/).
> We considered migrating from the current NSIS installer to the MSIX format that works better with enterprise
> deployment tools, however, MSIX doesn't allow any customization of the installation process.
> Adopting this format would require decoupling the VNet service from the application, leaving us without a clear
> mechanism on how to update the standalone service.

#### UX and Discoverability

Since VNet is exclusive to the per-machine installation, the installer will explicitly state it:
> To use VNet, install the app per-machine.

If a user attempts to run VNet in a per-user installation, the app will verify the lack of the system service
and display an error:
> VNet system service not found.
> To use VNet, install Teleport Connect in a per-machine mode. Administrator privileges will be required.

### Per-Machine Mode

In this mode, the installer will install both the application and the VNet service.

To install per-machine updates silently, we will introduce a special Update Service, implemented by tsh.exe
(`tsh.exe connect-update-service`).
This service will run with full system permissions, and its only job will be to install updates that the main app has
already downloaded.

We will set up the service DACL so that any authenticated user can start, stop, or check its status.
The service will never blindly run a passed file; it must verify the digital signature before executing the update.

#### Security Considerations

Even tough the update service will only allow installing binaries singed by the same organization as the service itself,
an attacker could force it to install an older, vulnerable version of the VNet service.

This is possible because of two reasons: Teleport Client Tool Managed Updates rely on clusters specifying a required
client version and Teleport Connect is multi-cluster client.
An attacker could trick a user into adding a malicious cluster to the app and setting it as the one managing updates,
effectively granting that remote cluster control over the local per-machine service version.

This risk is a direct consequence of our update model. Since different clusters may require specific client versions, we
delegate the decision of which cluster to "follow" for updates to the end user (for details see
https://github.com/gravitational/teleport/blob/75b56b1c67bd7eccc1074738ece671adccf21ea2/rfd/0144-client-tools-updates.md#multi-cluster-support).
While a similar risk exists for per-user installations, the impact is isolated to the user's local space.
With a per-machine service, a standard user's decision to follow a malicious cluster could compromise the security of
the entire machine.

#### Mitigation

To mitigate the risk of malicious version changes, the privileged service can restrict update sources to authorized
clusters only:

* Teleport Cloud clusters under the `*.teleport.sh` domain are trusted by default, as Teleport manages their
  `client_tools_version` directly.
* Updates from self-hosted environments are blocked unless the cluster hostname is present in a system-level allowlist
  located in the Windows Registry (in HKLM hive).

To prevent unprivileged users from bypassing these restrictions, the allowlist is protected: only users with local
admin privileges (via the app, confirmed by a UAC prompt) or IT Admins (via Windows Group Policy) can modify the
registry entries.

The main drawback of this approach is that it increases complexity of the Teleport Managed Updates (with system registry
configuration). On top of that, it only applies to per-machine Teleport Connect for Windows installations.
This means extra work for IT staff; to keep updates working in self-hosted environments, they will need to set up
Windows Group Policy rules in addition to the normal app install.

#### Update Process

1. Teleport Connect (user-space) verifies that the managing cluster is on the allowlist; if not, an error is displayed
   to the user.
1. The app downloads the update binary to a standard, user-writeable directory.
1. When the user clicks "Restart to Update" or closes the app, Connect triggers the privileged updater service, passing
   the file path and the managing cluster hostname.
    * Installing the update on app close is triggered from a synchronous `app.on('quit')` event handler. When this
      handler is invoked, the tsh daemon has already quit. Because of that, it's not feasible to perform an
      authentication between Teleport Connect ⇔ Updater Service (like we have between Teleport Connect ⇔ VNet
      Service).
      The app will only invoke `sc start TeleportUpdateService install-connect-update --path=... --cluster=...` and exit
      immediately.
1. The service moves the binary to a system-protected path (e.g., `C:\Windows\Temp`) to prevent tampering during the
   validation phase.
1. The service re-validates that the passed cluster is either a Teleport Cloud instance or present in the system
   registry
   allowlist.
1. The service checks the signature to ensure the binary is signed by the same organization and extracts the version
   string directly from the signature.
1. The service pings the passed cluster to fetch its `client_tools_version` and confirms it matches the version
   of the staged binary.
1. The service executes the update and exits.

#### Updates Configuration in System Registry

Update settings will be managed via the Windows Registry. Configuration can be applied at either the machine level
(HKLM) or the user level (HKCU).

* Settings in `HKEY_LOCAL_MACHINE` take priority over `HKEY_CURRENT_USER`.
* `TELEPORT_TOOLS_VERSION` and `TELEPORT_CDN_BASE_URL` env variables will be ignored. This prevents standard users
  from overriding versioning policies.
    * Overall, using the registry instead of env variables will provide much better UX. The env vars are difficult to
      use for desktop apps.

Registry Paths:

* `HKEY_LOCAL_MACHINE\SOFTWARE\Policies\TeleportConnect`
* `HKEY_CURRENT_USER\SOFTWARE\Policies\TeleportConnect`

| Setting                  | Type         | Description                                                                                                                                                                                |
|--------------------------|--------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| ToolsVersion             | REG_SZ       | Pins the application to a specific `X.Y.Z` version or disables updates entirely (`off`). Replaces `TELEPORT_TOOLS_VERSION`.                                                                |
| CdnBaseUrl               | REG_SZ       | Specifies a custom build source or CDN mirror in a private network. Replaces  `TELEPORT_CDN_BASE_URL`.                                                                                     | 
| AuthorizedUpdateClusters | REG_MULTI_SZ | A list of Teleport cluster hostnames (e.g., `teleport.example.com`) authorized to provide per-machine updates. This is exclusive to per-machine installations and must be defined in HKLM. |

## Alternatives

As mentioned earlier, the VNet service could be separated from Teleport Connect. The app would install in user-space,
while the VNet service would be treated as a per-machine, on-demand component.

Pros:

* Eliminates the need for dual-mode installer, which is not great for first time users.

Cons:

* IT admins would have to manage and deploy two separate pieces of software instead of one.
* This doesn't fix the update issue. To update the service, we would still have to choose between the UAC prompts or
  leave the system open to the "service downgrade" security risk mentioned earlier.