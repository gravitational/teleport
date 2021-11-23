# Windows Desktop Access Beta

## How to set up Desktop Access on Windows

### Install Windows Server 2012

Download and instal a trial version of Windows Server 2012 R2 from:
https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2012-r2

Windows Server 2012 is the oldest version we support.

See [this appendix](#appendix-virtualbox-notes) if using VirtualBox.

### Set up Active Directory

#### AD DS

First, we need to install Active Directory Domain Services (AD DS). From Server
Manager, in the top-right select `Manage > Add Roles and Features`.

In the wizard, select:

- Before you Begin: click `Next`
- Installation Type: `Role-based or feature-based installation`
- Server Selection: click `Next` (should be only one server - current one)
- Server Roles: select `Active Directory Domain Services`, in the popup click `Add Features`
- Features: click `Next`
- AD DS: click `Next`
- Confirmation: click `Install`, wait for completion
- Results: click `Promote this server to a domain controller` blue link

Another wizard (AD DS configuration) will open:

- Deployment Configuration: select `Add a new forest`, type in the Root domain
  name. Any DNS-like name will work, for example `example.com`, **write down
  this name for later**. Click `Next`.
- Domain Controller Options: type in a `Directory Services Restore Mode (DSRM) password` and click `Next`.
- DNS Options: click `Next`
- Additional Options: wait for the NetBIOS name to be generated, **write it
  down for later**, click `Next.`
- Paths: click `Next`.
- Review Options: click `Next`.
- Prerequisites Check: wait for the check to pass and click `Install`.
- Results: after ~10s, VM will restart

#### AD CS

Next, we'll install Active Directory Certificate Services (AD CS) to enable TLS
on LDAP connections. While AD CS is not strictly required, it is the easiest way
to generate a keypair and ensure that the server supports LDAPS. From Server
Manager, in the top-right select `Manage > Add Roles and Features`. (Note: you
won't be able to install both AD DS and AD CS at the same time, they need to be
separate).

Open a PowerShell prompt and run

```powershell
Add-WindowsFeature Adcs-Cert-Authority -IncludeManagementTools
Install-AdcsCertificationAuthority -CAType EnterpriseRootCA
```

Restart the VM after configuring AD CS.

## Follow The Docs

Now follow the [Getting Started](https://goteleport.com/docs/desktop-access/introduction/) documentation on the Teleport website to complete the installation.

## Appendix: VirtualBox notes

Some advice to make the setup easier with VirtualBox.

First, [install VirtualBox Guest
Additions](https://www.virtualbox.org/manual/ch04.html).

Next, create a shared folder via VM settings in VirtualBox. This will let you
easily transfer files between host and VM.

Finally, switch the Network Adapter to Bridged Adapter mode. This lets you
connect to the VM from your host. To get the VM IP, run `ipconfig.exe` from
PowerShell.
