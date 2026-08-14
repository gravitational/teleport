# teleport-windows-auth

[![Build](https://github.com/gravitational/teleport.e/actions/workflows/windows-auth-package.yaml/badge.svg)](https://github.com/gravitational/teleport.e/actions/workflows/windows-auth-package.yaml)

A Windows package supporting Teleport Desktop Access without Active Directory

## Prerequisites

1. Building the DLL requires that 64-bit MinGW is installed. On Mac you can use `brew install mingw-w64`.
2. Installation requires that you export the Teleport User CA by running `tctl auth export --type=windows > win.cer`.

## Installation

The easiest installation procedure is listed under the `Automatic` heading below.
If you're interested in what goes on under the hood and wish to inspect each installation
step for yourself, follow the instructions under the `Manual` section instead.

### Automatic

1. Build installer: `make all`
2. Copy `build/teleport-windows-auth-setup*.exe` and `win.cer` (see `Prerequisites` above) to target Windows machine
3. Run installer
   * for UI just double-click
   * for headless/CLI run `teleport-windows-auth-setup.exe --help` to see the options
4. Add Windows machine as static host in Desktop Service in Teleport config
5. You should be able to connect to it from WebUI

### Manual

1. Build DLL: `make build/teleport.dll` or `make all`
2. Copy `build/teleport.dll` and `win.cer` (see `Prerequisites` above) to target Windows machine
3. Install `win.cer` as Trusted Root Certificate (Local Machine store)
4. Copy DLL to `System32` (e.g. from PS elevated shell `cp teleport.dll C:\Windows\System32`)
5. Disable NLA
6. Add correct values to the registry. You can use `.reg` file below:
   ```
   Windows Registry Editor Version 5.00

   [HKEY_LOCAL_MACHINE\SOFTWARE\Classes\CLSID\{FF285315-5335-4F69-A9A2-9CC5F8419D55}]
   @="Teleport"

   [HKEY_LOCAL_MACHINE\SOFTWARE\Classes\CLSID\{FF285315-5335-4F69-A9A2-9CC5F8419D55}\InprocServer32]
   @="C:\\Windows\\System32\\teleport.dll"
   "ThreadingModel"="Both"

   [HKEY_LOCAL_MACHINE\SOFTWARE\Classes\CLSID\{FF285315-5335-4F69-A9A2-9CC5F8419D55}\ProgId]
   @="Teleport"

   [HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\Credential Providers\{FF285315-5335-4F69-A9A2-9CC5F8419D55}]
   @="Teleport"

   [HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\Credential Provider Filters\{FF285315-5335-4F69-A9A2-9CC5F8419D55}]
   @="Teleport"

   ; hex data here means: msv1_0 teleport, if you have other authentication packages installed
   ; remove section below and add teleport to the end of the list manually
   [HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Lsa]
   "Authentication Packages"=hex(7):6d,00,73,00,76,00,31,00,5f,00,30,00,00,00,74,\
   00,65,00,6c,00,65,00,70,00,6f,00,72,00,74,00,00,00,00,00

   ```

## Compatibility

Tested to work on Windows 10 Pro, Windows Server 2012 R2, Windows Server 2016, Windows Server 2019, and Windows Server 2022.

On Windows Server 2012 R2 you need to change certificate presented by Remote Desktop Server to use P-384 ECDSA and
SHA-384.

1. Generate v3 certificate, it can be self-signed. Pack it with private key to `pkcs12` format. You can
   use `gen_Windows2012R2_example_cert.sh` to generate example.
2. Copy it to Windows machine (`my-service.p12` file if you use example script)
3. Add it to Local machine's Personal cert store using `certlm.msc`
4. In `certlm.msc` right-click on added certificate, go to `All tasks`->`Manage private keys`, add `Network Service`
   account with at least `Read` permission
5. Get SHA1 thumbprint of certificate
6. Run `regedit`, go to `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp`, add
   binary value `SSLCertificateSHA1Hash` with value equal to thumbprint from previous step
7. Restart Windows server
8. New certificate should be used by RDP and Teleport should be able to connect
