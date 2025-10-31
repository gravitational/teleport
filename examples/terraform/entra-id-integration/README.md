## Teleport Entra ID integration

This directory contains the Terraform module to configure Entra ID with attributes necessary for the Teleport Entra ID integration. 

Entra ID configuration involves:
- Creating an enterprise application.
- Configuring enterprise application as an SAML SSO provider for Teleport.
- Configuring an authentication method for Teleport, so it can import users and groups from Entra ID.
  Teleport supports two kinds of authentication methods:
  - Teleport as an OIDC IdP - Set up Teleport as an OIDC Identity Provider for the enterprise application. 
    Teleport Proxy Service must be available under public IP for this method to work. 
    Use this option if you are using Teleport cloud.
  - System credentials - Grant API permissions to the managed identity that is assigned to the Teleport Auth Service. 
    Use this option if you have a self-hosted Teleport cluster.

  In both the cases, you will need to configure three API permissions: `Application.ReadWrite.OwnedBy, Group.Read.All, User.Read.All`.

This example is best followed with the official [Teleport Entra ID integration](https://goteleport.com/docs/identity-governance/entra-id/terraform) guide. 

### Set up Azure CLI

This example expects an authenticated Azure CLI session available in your environment.
You will also need tenant-level access and an appropriate subscription to run this example.
E.g., To authenticate with Azure using a user login credential:
```
$ az login --allow-no-subscriptions
```

#### Example configuration to setup SAML SSO and Teleport as an OIDC IdP.
> [!WARNING]
> 
> The `certificate_expiry_date` field is used to configure expiry date for the certificate 
> which will be used to sign SAML assertion. To avoid access disruption, ensure to update the 
> value before it expires. When you update the value, you also must update the Entra ID Auth 
> Connector in Teleport with a new entity descriptor.
```
cat > variables.auto.tfvars << EOF
tenant_id               = "<tenant ID>"
app_name                = "teleport-entraid-integration"
proxy_service_address   = "example.teleport.sh" # Teleport Proxy Service host
certificate_expiry_date = "" # example format - "2028-05-01T01:02:03Z" 
EOF
```

#### Example configuration to setup SAML SSO and managed identity.

You will need the principal ID of the managed identity that is assigned to the Teleport Auth Service.
You will also need the permission ID of these Graph API permissions - `Application.ReadWrite.OwnedBy, Group.Read.All, User.Read.All`.
These permission objects aren't directly exposed in the Azure portal. 
You will need to run PowerShell script to obtain permission ID available in your Entra ID tenant.
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

# Filter and find the app roles in the Microsoft Graph service principal that matches with permissions.
# Only include roles where "AllowedMemberTypes" includes "Application" (suitable for managed identities).
$appRoles = $GraphServicePrincipal.AppRoles |
    Where-Object Value -in $permissions |
    Where-Object AllowedMemberTypes -contains "Application"

# Print ID of each of the three permissions.
foreach ($appRole in $appRoles) {
  "{0} : {1}" -f $appRole.Value, $appRole.Id 
}
```

Generate tfvars with `use_system_credentials=true`, `graph_permission_ids` with permission IDs you retrieved using 
PowerShell and `managed_id` with the principal ID of the managed identity.
> [!WARNING]
> 
> The `certificate_expiry_date` field is used to configure expiry date for the certificate 
> which will be used to sign SAML assertion. To avoid access disruption, ensure to update the 
> value before it expires. When you update the value, you also must update the Entra ID Auth 
> Connector in Teleport with a new entity descriptor.
```
cat > variables.auto.tfvars << EOF
tenant_id               = "<tenant ID>"
app_name                = "teleport-entraid-integration"
proxy_service_address   = "example.teleport.sh" # Teleport Proxy Service host
certificate_expiry_date = "" # example format - "2028-05-01T01:02:03Z" 

use_system_credentials  = true
graph_permission_ids    = [<permission IDs>]
managed_id              = "<principal ID of the managed identity>"
EOF
```
