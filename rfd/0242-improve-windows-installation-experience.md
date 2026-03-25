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
(`tsh.exe connect-updater-service`).
This service will run with full system permissions, and its only job will be to install updates that the main app has
already downloaded.

We will set up the service DACL so that any authenticated user can start the service and check its status.
The service must be extremely careful and must not implicitly trust any input data.

#### Security Considerations

Even though the update service will only allow installing binaries signed by the same organization as the service itself
(enforced if the service is signed), an attacker could force it to install an older, vulnerable version of the VNet
service.

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

There is an open question whether the no-downgrade policy should also apply to versions specified via the
`TELEPORT_TOOLS_VERSION` environment variable or the `ToolsVersion` registry value.

The `TELEPORT_TOOLS_VERSION` environment variable was originally designed to pin a specific version for debugging,
testing, or manual update scenarios for CLI tools. It was implemented in Connect primarily for compatibility; however,
changing the version of a desktop application through an environment variable is neither common nor convenient. While
moving update control to the system registry improves the user experience, it does not seem to justify increasing
the complexity of the autoupdate rules by allowing downgrades in certain cases.

For most users, it is sufficient to disable updates entirely by setting the value to `off`.

#### Update Process

1. Download an update.
    - The app downloads the update binary to a user-writable directory.

2. Triggering the service.
    - On "Restart to Update" or app close, Connect spawns `tsh.exe `, passing the file path and
      version as arguments.
    - This happens from a synchronous `app.on('quit')` handler after the `tsh` daemon has exited.

3. Transferring the update to the service.
    - To avoid privilege escalation, the service must not open the update file as SYSTEM; file access must occur with
      standard user privileges.
    - Ideally this would be done via `ImpersonateNamedPipeClient` from the service side, but this Windows API is not
      available in `golang.org/x/sys/windows`.
    - Instead, since a named pipe is open anyway, is can be used to securely transfer update metadata and stream the
      binary itself.
    - Flow:
        - `tsh.exe connect-updater-install-update --path= --update-version=` starts the update service and waits for it.
        - The service starts, opens a named pipe `\\.\pipe\TeleportConnectUpdaterPipe` and accepts only a single
          connection from an authenticated user. Any other client must wait until the current service invocation exits.
            - The pipe will be created with the following security descriptor (pseudocode):
              ```         
              "D:" + // DACL
              "(A;;GA;;;SY)" + // Allow (A);; Generic All (GA);;; SYSTEM (SY)
              "(A;;GA;;;BA)" + // Allow (A);; Generic All (GA);;; Built-in Admins (BA)
              "(A;;GENERIC_READ|FILE_WRITE_DATA;;;AU)" // Allow (A);;GENERIC_READ | FILE_WRITE_DATA ;;;Authenticated Users (AU)
              ```
            - Note granting `GENERIC_READ | FILE_WRITE_DATA` (not GENERIC_WRITE) so authenticated users can read/write
              pipe data
              but [cannot create pipe instances](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights).
            - Internally, `winio.ListenPipe` calls `NtCreateNamedPipeFile` with `FILE_CREATE` flag, which ensures the
              call succeeds only if the pipe name does not already exist, preventing pipe-name squatting.
        - The user process sends metadata (version and size) as JSON and then streams the update binary over the pipe.
    - The service will start with some restrictions: max 30s lifetime, and 1 GB size limit for data transferred over the
      pipe.

4. Staging the update file.
    - Ensure `%ProgramData%\TeleportConnectUpdater` directory exists.
        - First try to create a directory with the following security descriptor, granting access only to SYSTEM and
          Administrators (pseudocode):
          ```  
          "O:SY" + // Owner SYSTEM
          "D:P" + // 'P' blocks permissions inheritance from the parent directory
          "(A;OICI;GA;;;SY)" + // Allow System Full Access
          "(A;OICI;GA;;;BA)" // Allow Built-in Administrators Full Access
           ```
        - If `ERROR_ALREADY_EXISTS` error is returned, open a file handle and verify it's a directory (and not a reparse
          point). Reapply the security descriptor to ensure no unauthorized permissions persist.
        - Attempt to delete all existing contents (files and subdirectories) to prevent excessive disk usage.
        - Directory cleanup will be implemented using `os.RemoveAll`; its behavior has been checked to ensure it does
          not traverse reparse points and therefore does not delete data outside the target directory.
    - Because the directory may have been pre-created by an unprivileged user, deleting existing files (including
      planted DLLs) may fail if the owner keeps an open handle. To mitigate this, the service writes the update binary
      to a new, unique, system-protected subdirectory:
       ```
       %ProgramData%\TeleportConnectUpdater\<GUID>
       ```
   This provides isolation from any malicious DLL that could be loaded from the same directory, resulting in an LPE
   vector.

5. Validation.
    - Read the `ToolsVersion` value from `HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Teleport\TeleportConnect`.
        - If the value is set to `off`, the service exits early.
        - If the value is a semver string, the service checks if update version matches it.
    - Ensure the passed update version is newer than the service (`tsh.exe`) version.
    - Download and verify the SHA256 checksum for the target Teleport Connect version to prevent the service from
      installing *any* binary if the service is unsigned.
    - If the service is signed, verify that the update's signature subject matches the service's signature subject.

6. Installation.
    - Execute the update and exit.

#### Updates Configuration in System Registry

Update settings will be managed via the Windows Registry. Configuration can be applied at either the machine level
(HKLM) or the user level (HKCU).

* Settings in `HKEY_LOCAL_MACHINE` take priority over `HKEY_CURRENT_USER`.
* `TELEPORT_TOOLS_VERSION` and `TELEPORT_CDN_BASE_URL` env variables will be deprecated. This prevents standard users
  from overriding versioning policies.
    * Overall, using the registry instead of env variables will provide much better UX. The env vars are difficult to
      use for desktop apps.
    * If the legacy environment variables are set (and there is no HKLM config), they will prevent per-machine updates
      from installing silently (the standard UAC installer will be executed). The new Update Service will only read
      configuration from HKLM.

Registry Paths:

* `HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Teleport\TeleportConnect`
* `HKEY_CURRENT_USER\SOFTWARE\Policies\Teleport\TeleportConnect` - must be used only in the context of per-user
  installations,
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