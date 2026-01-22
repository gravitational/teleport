---
authors: Grzegorz Zdunek (grzegorz.zdunek@goteleport.com)
state: draft
---

# RFD 0242 - Improve Windows installation experience

## Required Approvers

- Engineering: @ravicious && (@avatus || @zmb3)

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

During the runtime, the app will detect the per-machine mode by checking if it's been installed to the path specified in
`HKLM\Software\22539266-67e8-54a3-83b9-dfdca7b33ee1\InstallLocation`. This registry entry is created by electron-builder
when installing the app. The UUID is stable, generated from the `appId` and electron-builder's internal ID.
It will be also hardcoded in `nsis.guid` in electron-builder config.

Based on this check, the following update logic can be applied:

1. If the app path matches the key in HKLM, try to use the privileged updater. If the updater service doesn't exist,
   fallback to the UAC prompt.
2. If the app path doesn't match the registry value (or there's no HKLM key for the app), update per user.

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
> Only the per-machine option comes with VNet, Teleport's VPN-like experience for accessing TCP applications and SSH
> servers.

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

We will set up the service DACL so that any authenticated user can start the service.
The service must be extremely careful and must not implicitly trust any input data.

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

To reduce the risk of malicious version changes, client tool updates in Connect will only permit upgrades (currently
downgrades are allowed).
This restriction will be enforced in both the user-space application and the privileged updater.
Since most Connect users are logged into one (or at most two) clusters, frequent version switching is uncommon,
and this change should be largely unnoticeable to them.

Important: this measure does not fully eliminate the risk, as Teleport maintains two release branches simultaneously.
As a result, a version having higher major number doesn't necessarily have to be a newer one, for example, the latest
N-1.x.x release may be more recent than an N.0.0 version published months earlier.
However, this change does make the auto-update system more predictable: once the application is on the latest version,
it cannot downgrade to an older release.

Additionally, in case of a serious vulnerability, a patch release could
modify [autoupdate thresholds for the app](https://github.com/gravitational/teleport/blob/07aa75612674e1f19a0f40ae61c9e370f25a876e/web/packages/teleterm/src/services/appUpdater/clientToolsUpdateProvider.ts#L163-L166)
(and for the service) to disallow updates from a version N-1.x.x to release below certain N.x.x version.

#### Update Process

1. The app downloads the update binary to a standard, user-writeable directory.
1. When the user clicks "Restart to Update" or closes the app, Connect triggers the privileged updater service, passing
   the file path and the managing cluster hostname.
    * Installing the update on app close is triggered from a synchronous `app.on('quit')` event handler. When this
      handler is invoked, the tsh daemon has already quit. Because of that, it's not feasible to perform an
      authentication between Teleport Connect ⇔ Updater Service (like we have between Teleport Connect ⇔ VNet
      Service).
      The app will only invoke `sc start TeleportUpdateService install-connect-update --path=...` and exit
      immediately.
1. The service copies the binary to a system-protected path (e.g., `%ProgramData%\TeleportConnect\Updates\<GUID>`) to
   prevent tampering during the validation phase. This directory will be locked down to Administrators and LocalSystem
   access only.
    * The service must ensure it is not copying a file that the user doesn't have access to.
    * An attacker could use the service to simply copy arbitrary files into our secure `%ProgramData%` location. For
      example, they could first copy a malicious version.dll (loaded by almost every executable out there), then copy a
      properly signed executable and have it executed. The malicious DLL could then be loaded from the same directory,
      resulting in another LPE vector. This can be mitigated by creating a unique subdirectory for each service
      invocation.
    * The `%ProgramData%` is user-writable by default so ensure the directory wasn't pre-created before and there aren't
      any planted DLLs suitable for DLL hijacking.
    * To prevent the directory size from expanding indefinitely, it will be cleared before copying the update file.
1. The service checks the signature to ensure the binary is signed by the same organization and extracts the version
   string directly from the signature.
    * If the service itself is not signed, it will not require an update file to be signed.
1. The service verifies that the update version is higher than its own (`tsh.exe` binary) version.
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
* `HKEY_CURRENT_USER\SOFTWARE\Policies\TeleportConnect` - must be used only in the context of per-user installations,
  will not affect per-machine behavior.

| Setting      | Type   | Description                                                                                                                 |
|--------------|--------|-----------------------------------------------------------------------------------------------------------------------------|
| ToolsVersion | REG_SZ | Pins the application to a specific `X.Y.Z` version or disables updates entirely (`off`). Replaces `TELEPORT_TOOLS_VERSION`. |
| CdnBaseUrl   | REG_SZ | Specifies a custom build source or CDN mirror in a private network. Replaces  `TELEPORT_CDN_BASE_URL`.                      | 

## Alternatives

As mentioned earlier, the VNet service could be separated from Teleport Connect. The app would install in user-space,
while the VNet service would be treated as a per-machine, on-demand component.

Pros:

* Eliminates the need for dual-mode installer, which is not great for first time users, since we ask them to make a
  decision about the VNet component that they likely have no idea about.

Cons:

* We would need to create and maintain another package, like `Teleport VNet.exe` that would consist of `tsh.exe`,
  `wintun.dll` and `msgfile.dll`.
* IT admins would have to manage and deploy two separate pieces of software instead of one.
    * Technically, this could still be a single deployment if `Teleport VNet.exe` were executed as part of the Teleport
      Connect installation.
* If the app remained in the per-user scope while the service moved to the per-machine scope, they would be updated
  independently.
    * This could introduce issues in multi-user environments: one user might be running app version 1.0.0 while
      another is on 1.0.1. The user on version 1.0.1 could trigger a VNet service auto-update, which would then prevent
      the 1.0.0 user from starting VNet (unless we provide some compatibility between the app the service).
    * On the other hand, updating a shared per-machine instance via the privileged updater in multi-user setups is also
      not ideal. However, since only upgrades are allowed, both the application and the VNet service would eventually
      update to the same latest version, avoiding version mismatches.