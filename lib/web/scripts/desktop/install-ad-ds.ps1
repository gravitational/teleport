$ErrorActionPreference = "Stop"

$DOMAIN = Read-Host "Enter your domain name"
$NET_BIOS_DOMAIN = ($DOMAIN -split '\.')[0].ToUpperInvariant()

echo 'Installing the AD services and administration tools...'
Install-WindowsFeature AD-Domain-Services,RSAT-AD-AdminCenter,RSAT-ADDS-Tools

echo 'Installing AD DS (be patient, this may take a while to install)...'
Import-Module ADDSDeployment
Install-ADDSForest `
    -InstallDns `
    -CreateDnsDelegation:$false `
    -ForestMode 'Win2012R2' `
    -DomainMode 'Win2012R2' `
    -DomainName $DOMAIN `
    -DomainNetbiosName $NET_BIOS_DOMAIN `
    -SafeModeAdministratorPassword (Read-Host "Enter your password" -AsSecureString) `
    -NoRebootOnCompletion `
    -Force

Restart-Computer -Force