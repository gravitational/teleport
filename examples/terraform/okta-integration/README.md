## Teleport Okta integration

This directory contains the Terraform module to configure Okta with the apps and
permissions necessary for the Teleport Okta integration.

Okta configuration involves:
- Creating a SAML 2.0 app for Teleport SSO and assigning Okta groups to it.
- Creating an API Services app that Teleport uses to call the Okta API for user
  sync and for app and group sync:
  - Teleport authenticates with a JWT signed by its Okta key. Okta fetches the
    public key from the `/.well-known/jwks-okta` endpoint of the Teleport Proxy
    Service, so the Proxy Service must be reachable from Okta.
  - The app gets the `okta.apps.*`, `okta.groups.*`, and `okta.users.*` API
    scopes, and a custom admin role scoped to a resource set that limits which
    users, groups, and apps Teleport can access.

This example is best followed with the official [Teleport Okta integration](https://goteleport.com/docs/identity-governance/integrations/okta/)
guides.

### Set up Okta credentials

This example expects Okta credentials available in your environment. For example,
to use an API token from an Okta super administrator:
```
$ export OKTA_API_TOKEN=<API token>
```

Creating a custom admin role, a resource set, and an admin role assignment for
an app requires super administrator permissions.

### Example configuration

```
cat > variables.auto.tfvars << EOF
okta_org_name         = "dev-123456"          # dev-123456.okta.com
okta_base_url         = "okta.com"
proxy_service_address = "example.teleport.sh" # Teleport Proxy Service host
group_names           = ["admins", "readonly"]
EOF
```

Then apply the configuration:
```
$ terraform init
$ terraform plan
$ terraform apply
```

### Finish the integration in Teleport

In the Teleport Web UI, go to **Add New** > **Integration**, select the **Okta**
tile, and use the module outputs:

- For SSO, paste `saml_metadata_url` as the SAML app metadata URL.
- For user sync, enter `okta_org_url` as the Okta organization URL and
  `api_app_client_id` as the client ID. The module already configures the app
  to trust the Teleport JWKS URL, so skip that part of the guided flow.
- The module doesn't configure SCIM provisioning, you need to enable it in the
  Okta Admin Console. See the [Okta SCIM integration](https://goteleport.com/docs/identity-governance/integrations/okta/scim-integration/) guide.

### Private Teleport clusters

If Okta can't reach the Teleport Proxy Service, replace `jwks_uri` in `okta_app_oauth.teleport_okta_integration_api`
with a `jwks` block that holds the public key shown under **If your cluster is
private** in the guided flow.
