$ErrorActionPreference = "Stop"

$TELEPORT_CA_CERT_PEM = '{{.CACertPEM}}'
$TELEPORT_CA_CERT_SHA1 = '{{.CACertSHA1}}'
$TELEPORT_CA_CERT_BLOB_BASE64 = '{{.CACertBase64}}'
$TELEPORT_CRL_PEM = '{{.CRLPEM}}'
$TELEPORT_PROXY_PUBLIC_ADDR = '{{.ProxyPublicAddr}}'
$TELEPORT_PROVISION_TOKEN = '{{.ProvisionToken}}'

$DB_ADDRESS = '{{.DBAddress}}'
$COMPUTER_NAME = ($DB_ADDRESS -split '\.')[0].ToLower()

$DOMAIN_NAME=(Get-ADDomain).DNSRoot
$DOMAIN_DN=$((Get-ADDomain).DistinguishedName)

# # Step 1: Configure GPO to enable Teleport access.
$ACCESS_GPO_NAME="Teleport DB access"
try {
  $ACCESS_GPO = Get-GPO -Name $ACCESS_GPO_NAME
}
catch [System.ArgumentException]
{
  $ACCESS_GPO = New-GPO -Name $ACCESS_GPO_NAME
  $ACCESS_GPO | New-GPLink -Target $DOMAIN_DN
}

$CERT = [System.Convert]::FromBase64String($TELEPORT_CA_CERT_BLOB_BASE64)
Set-GPRegistryValue -Name $ACCESS_GPO_NAME -Key "HKEY_LOCAL_MACHINE\Software\Policies\Microsoft\SystemCertificates\Root\Certificates\$TELEPORT_CA_CERT_SHA1" -ValueName "Blob" -Type Binary -Value $CERT

$TeleportPEMFile = $env:TEMP + "\teleport.pem"
Write-Output $TELEPORT_CA_CERT_PEM | Out-File -FilePath $TeleportPEMFile

$TeleportCRLFile = $env:TEMP + "\teleport-crl.pem"
Write-Output $TELEPORT_CRL_PEM | Out-File -FilePath $TeleportCRLFile

certutil -dspublish -f $TeleportPEMFile RootCA
certutil -dspublish -f $TeleportPEMFile NTAuthCA
certutil -dspublish -f $TeleportCRLFile TeleportDB
certutil -pulse

$ACCESS_SECURITY_TEMPLATE=@'
[Unicode]
Unicode=yes
[Version]
signature="$CHICAGO$"
[Service General Setting]
"SCardSvr",2,""
'@

$COMMENT_XML=@'
<?xml version='1.0' encoding='utf-8'?>
<policyComments xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" revision="1.0" schemaVersion="1.0" xmlns="http://www.microsoft.com/GroupPolicy/CommentDefinitions">
  <policyNamespaces>
    <using prefix="ns0" namespace="Microsoft.Policies.TerminalServer"></using>
  </policyNamespaces>
  <comments>
    <admTemplate></admTemplate>
  </comments>
  <resources minRequiredRevision="1.0">
    <stringTable></stringTable>
  </resources>
</policyComments>
'@

$ACCESS_POLICY_GUID=$ACCESS_GPO.Id.Guid.ToUpper()
$ACCESS_GPO_PATH="$env:SystemRoot\SYSVOL\sysvol\$DOMAIN_NAME\Policies\{$ACCESS_POLICY_GUID}\Machine\Microsoft\Windows NT\SecEdit"

New-Item -Force -Type Directory -Path $ACCESS_GPO_PATH
New-Item -Force -Path $ACCESS_GPO_PATH -Name "GptTmpl.inf" -ItemType "file" -Value $ACCESS_SECURITY_TEMPLATE
New-Item -Force -Path "$env:SystemRoot\SYSVOL\sysvol\$DOMAIN_NAME\Policies\{$ACCESS_POLICY_GUID}\Machine" -Name "comment.cmtx" -ItemType "file" -Value $COMMENT_XML

# # Step 2: Generate Teleport configuration.

$WindowsDERFile = $env:TEMP + "\windows.der"
$WindowsPEMFile = $env:TEMP + "\windows.pem"
certutil "-ca.cert" $WindowsDERFile 
certutil -encode $WindowsDERFile $WindowsPEMFile

$CA_CERT_PEM = Get-Content -Path $WindowsPEMFile
$CA_CERT_YAML = $CA_CERT_PEM | ForEach-Object { "          " + $_  } | Out-String

$KDC_HOSTNAME = (Get-ADDomainController).HostName
# Get the SPN that contains the MSSQLSvc and the port number.
$SPN = (Get-ADComputer -Identity $COMPUTER_NAME -Properties servicePrincipalName).servicePrincipalName | Where-Object {$_ -like "MSSQLSvc/*:*"}

$DATABASE_ACCESS_CONFIG_YAML=@"
version: v3
teleport:
  auth_token: $TELEPORT_PROVISION_TOKEN
  proxy_server: $TELEPORT_PROXY_PUBLIC_ADDR

auth_service:
  enabled: no
ssh_service:
  enabled: no
proxy_service:
  enabled: no

db_service:
  enabled: "yes"
  databases:
    - name: $COMPUTER_NAME
      protocol: sqlserver
      uri: $DB_ADDRESS
      ad:
        domain: $DOMAIN_NAME
        spn: $SPN
        kdc_host_name: $KDC_HOSTNAME
        ldap_cert: |
$CA_CERT_YAML
"@

$OUTPUT=@'

Use the following teleport.yaml to start a Database Access Service.
For a detailed configuration reference, see

https://goteleport.com/docs/reference/agent-services/database-access-reference/configuration/


{0}

'@ -f $DATABASE_ACCESS_CONFIG_YAML

Write-Output $OUTPUT

# Cleanup files that were created during execution of this script.
Remove-Item $TeleportPEMFile -Recurse
Remove-Item $TeleportCRLFile -Recurse
Remove-Item $WindowsDERFile -Recurse
Remove-Item $WindowsPEMFile -Recurse
