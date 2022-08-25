# Windows Desktop Access Beta

## How to set up Desktop Access on Windows

### Install Windows Server 2012

Download and instal a trial version of Windows Server 2012 R2 from:
https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2012-r2

Windows Server 2012 is the oldest version we support.

See [this appendix](#appendix-virtualbox-notes) if using VirtualBox.

### Set up Active Directory

#### AD DS

First, we need to install Active Directory Domain Services (AD DS). Save the following file as `domain-controller.ps1`,
replacing `$domain` with your desired domain name.

```powershell
$ErrorActionPreference = "Stop"

$domain = 'example.com'
$netbiosDomain = ($domain -split '\.')[0].ToUpperInvariant()

echo 'Installing the AD services and administration tools...'
Install-WindowsFeature AD-Domain-Services,RSAT-AD-AdminCenter,RSAT-ADDS-Tools

echo 'Installing AD DS (be patient, this may take a while to install)...'
Import-Module ADDSDeployment
Install-ADDSForest `
    -InstallDns `
    -CreateDnsDelegation:$false `
    -ForestMode 'Win2012R2' `
    -DomainMode 'Win2012R2' `
    -DomainName $domain `
    -DomainNetbiosName $netbiosDomain `
    -SafeModeAdministratorPassword (Read-Host "Enter Your Password" -AsSecureString) `
    -NoRebootOnCompletion `
    -Force

Restart-Computer -Force
```

#### AD CS

Next, we'll install Active Directory Certificate Services (AD CS) to enable TLS
on LDAP connections. While AD CS is not strictly required, it is the easiest way
to generate a keypair and ensure that the server supports LDAPS.

Save the following file as `certificate-services.ps1`

```powershell
$ErrorActionPreference = "Stop"

Add-WindowsFeature Adcs-Cert-Authority -IncludeManagementTools
Install-AdcsCertificationAuthority -CAType EnterpriseRootCA -HashAlgorithmName SHA384 -Force
Restart-Computer -Force
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
