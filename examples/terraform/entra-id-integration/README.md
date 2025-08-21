## Teleport Entra ID integration
This directory contains the Terraform module to configure Entra ID with attributes necessary for Teleport Entra ID integration. 

Configuration involves:
- Creating an enterprise application.
- Configuring enterprise application as an SAML SSO provider for Teleport.
- Configuring an authentication method for Teleport so it can import users and groups from Entra ID.
  Teleport supports two kinds of authentication methods:
  - Teleport as an OIDC IdP - Set up Teleport as an OIDC Identity Provider for the enterprise application. 
    Teleport Proxy Service must be available under public IP for this method to work. 
    Use this option if you are using Teleport cloud.
  - System credentials - Grant API permissions to the managed identity assigned to the Teleport Auth Service. 
    Use this option if you have a self-hosted Teleport cluster.

  In both the cases, you will need to configure three API permissions: `Application.ReadWrite.OwnedBy, Group.Read.All, User.Read.All`

#### Example configuration to setup SAML SSO and Teleport as an OIDC IdP.
```
cat > variables.auto.tfvars << EOF
app_name                = "teleport-entraid-integration"
proxy_service_address   = "example.teleport.sh"
certificate_expiry_date = "2026-05-01T01:02:03Z" # Warning! An expired certificate will break user authentication.
EOF
```

#### Example configuration to setup SAML SSO and managed identity.

You will need the principal ID of the managed identity that is assigned to the Teleport Auth Service.
You will also need the permission ID of the `Application.ReadWrite.OwnedBy, Group.Read.All, User.Read.All` permissions.
Permission objects aren't directly exposed in the Azure portal, you will need a powershell script to permission obtain IDs. 

```
Connect-AzureAD

# This is a service principal object representing Microsoft Graph in Azure AD with a specific app ID.
$GraphServicePrincipal = Get-AzureADServicePrincipal -Filter "AppId eq '00000003-0000-0000-c000-000000000000'"

# These are Microsoft Graph API permissions required by the managed identity.
$permissions = @(
  "Application.ReadWrite.OwnedBy"   # Permission to read application
  "Group.Read.All"     # Permission to read groups in the directory
  "User.Read.All"        # Permission to read users in the directory
)

# Filter and find the app roles in the Microsoft Graph service principal that match the defined permissions.
# Only include roles where "AllowedMemberTypes" includes "Application" (suitable for managed identities).
$appRoles = $GraphServicePrincipal.AppRoles |
    Where-Object Value -in $permissions |
    Where-Object AllowedMemberTypes -contains "Application"

# Print ID of each of the three permissions.
foreach ($appRole in $appRoles) {
  "{0} : {1}" -f $appRole.Value, $appRole.Id 
}
```

Configure `graph_permission_ids` variable with the permission ID's you retrieved using powershell.  

```
cat > variables.auto.tfvars << EOF
app_name                = "teleport-entraid-integration"
proxy_service_address   = "example.teleport.sh"
certificate_expiry_date = "2026-05-01T01:02:03Z" # Warning! An expired certificate will break user authentication.

use_system_credentials   = true
graph_permission_ids    = [<permission IDs>]
managed_id              = "<principal ID of the managed identity>"
EOF
```
