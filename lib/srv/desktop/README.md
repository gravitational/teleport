# Windows Desktop Access VM Setup

## How to set up Windows Server for Desktop Access

These instructions install a Domain Controller and Certificate Servers on a Windows Server that
are required for Teleport Desktop Access to connect.

### Install Windows Server 

Download and install a trial version of Windows Server from

https://www.microsoft.com/en-us/evalcenter

|Server Version| Direct URL
|---|----
| Windows 2012 R2 | https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2012-r2 |
| Windows 2016 | https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2019 |
| Window 2019 | https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2019 |
| Windows 2022 | https://www.microsoft.com/en-us/evalcenter/download-windows-server-2022 |

Windows Server 2012 is the oldest version we support.

See [this appendix](#appendix-virtualbox-notes) if using VirtualBox.

### Set up Domain Controller with Active Directory Domain Services

#### AD DS

First, we need to install Active Directory Domain Services (AD DS). Save the following file as `domain-controller.ps1`,
replacing `$domain` with your desired domain name.  The `Default` setting for `ForestMode` and `DomainMode` will match to the server version.  
See [Msft Powershell parameters instructions](https://docs.microsoft.com/en-us/powershell/module/addsdeployment/install-addsforest#parameters) for all options.

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
    -ForestMode 'Default' `
    -DomainMode 'Default' `
    -DomainName $domain `
    -DomainNetbiosName $netbiosDomain `
    -SafeModeAdministratorPassword (Read-Host "Enter Your Password" -AsSecureString) `
    -NoRebootOnCompletion `
    -Force

Restart-Computer -Force
```

#### Setup Active Directory Certificate Services

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
