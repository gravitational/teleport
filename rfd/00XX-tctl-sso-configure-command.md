---
authors: Krzysztof Skrzętnicki <krzysztof.skrzetnicki@goteleport.com>
state: draft
---

# RFD XX - `tctl sso configure` command

## What

This RFD proposes a new subcommand for the `tctl` tool: `sso configure`. This is a convenience command to help with the
configuration of auth connectors. The command is an alternative to directly editing the auth connector resource files
and provides automated validation and sanity checks.

Not everything can be checked by the tool, as the overall validity of the SSO setup depends in part on the IdP
configuration which is invisible for us. This is why the output of the command should be tested with `tctl sso test`
command, e.g. by piping the output from one to another: `tctl sso configure ... | tctl sso test`.

The input is provided exclusively via command flags. In particular, no "guided" or "interactive" experience is provided.
This is a deliberate choice, as the web UI admin interface will be a better hosting environment for such a feature.

Ultimately we would like to support all providers for which we have
detailed [SSO how-to pages](../docs/pages/enterprise/sso). Initial scope covers only SAML.

## Why

We want to simplify the task of configuring a new auth connector. In contrast with free-form text editing of resource,
this command line tool will only output well-formed, validated auth connector. Whenever possible, the tool will
automatically fill in the fields, e.g. by fetching the Proxy public address and calculating the respective webapi
endpoint.

Additionally, the command provides a foundation for creating similar functionality in web-based UI.

## UX

Each connector kind is served by a separate subcommand. Most subcommand flags are not shared.

```bash
$ tctl sso configure --help

Create auth connector configuration.

...

Commands:
  sso configure saml   Configure SAML connector, optionally using a preset. Available presets: [okta onelogin ad adfs]
  sso configure oidc   Configure OIDC auth connector, optionally using a preset.
  sso configure github Configure GitHub auth connector.
```

### Flags: SAML

For `SAML` the commonly used flags will be:

```
-p, --preset                   Preset. One of: [okta onelogin ad adfs]
-n, --name                     Connector name. Required, unless implied from preset.
-e, --entity-descriptor        Set the Entity Descriptor. Valid values: file, URL, XML content. Supplies configuration parameters as single XML instead of individual elements.
-a, --attributes-to-roles      Sets attribute-to-role mapping in the form 'attr_name,attr_value,role1,role2,...'. Repeatable.
    --display                  Display controls how this connector is displayed.
```

The `--attributes-to-roles/-a` flag is particularly important as it is used to provide a mapping between the
IdP-provided attributes and Teleport roles. It can be specified multiple times.

Alternatives to `--entity-descriptor/-e` flag, allowing to specify these values explicitly:

```
--issuer                   Issuer is the identity provider issuer.
--sso                      SSO is the URL of the identity provider's SSO service.
--cert                     Cert is the identity provider certificate PEM. IDP signs <Response> responses using this certificate.
--cert-file                Like --cert, but read the cert from file.
```

Rarely used flags; the tool fills that information automatically:

```
--acs                      AssertionConsumerService is a URL for assertion consumer service on the service provider (Teleport's side).
--audience                 Audience uniquely identifies our service provider.
--service-provider-issuer  ServiceProviderIssuer is the issuer of the service provider (Teleport).
```

Advanced features:

- assertion encryption (support varies by IdP)
- externally-provided signing key (by default Teleport will self-issue this)
- overrides for specific IdP providers

```
--signing-key-file         A file with request signing key. Must be used together with --signing-cert-file.
--signing-cert-file        A file with request certificate. Must be used together with --signing-key-file.
--assertion-key-file       A file with key used for securing SAML assertions. Must be used together with --assertion-cert-file.
--assertion-cert-file      A file with cert used for securing SAML assertions. Must be used together with --assertion-key-file.
--provider                 Sets the external identity provider type. Examples: ping, adfs.
```

Flags for ignoring warnings:

```
--ignore-missing-roles     Ignore non-existing roles referenced in --attributes-to-roles.
--ignore-missing-certs     Ignore the lack of valid certificates from -e and --cert.
```

Available presets (`--preset/-p`):

| Name       | Description                          | Display   |
|------------|--------------------------------------|-----------|
| `okta`     | Okta                                 | Okta      |
| `onelogin` | OneLogin                             | OneLogin  |
| `ad`       | Azure Active Directory               | Microsoft |
| `adfs`     | Active Directory Federation Services | ADFS      |

Examples:

1) Generate SAML auth connector configuration named `myauth`.

- members of `admin` group will receive `access`, `editor` and `audit` role.
- members of `developer` group will receive `access` role.
- the IdP metadata will be read from `entity-desc.xml` file.

```
$ tctl sso configure saml -n myauth -a groups,admin,access,editor,audit -a group,developer,access -e entity-desc.xml
```

2) Generate SAML auth connector configuration using `okta` preset.

- The choice of preset affects default name, display attribute and may apply IdP-specific tweaks.
- Instead of XML file, a URL was provided to `-e` flag, which will be fetched by Teleport during runtime.

```
$ tctl sso configure saml -p okta -a group,dev,access -e https://dev-123456.oktapreview.com/app/ex30h8/sso/saml/metadata
```

3) Generate the configuration and immediately test it using `tctl sso test` command.

```
$ tctl sso configure saml -p okta -a group,developer,access -e entity-desc.xml | tctl sso test
```

Full flag reference: `tctl sso configure saml --help`.

### Flags: OIDC

_This section will be filled with expanded implementation scope._

### Flags: GitHub

_This section will be filled with expanded implementation scope._

## Security

The command will never modify existing Teleport configuration. This will be done by the user by further invocation
of `tctl create` command or by other means.

The command may handle user secrets. The implementation will ensure these are not silently written anywhere (e.g. we
must never save partially filled-in configuration files to temp files).

## Further work

### Full provider coverage

We should extend the set of supported providers to match [SSO how-to pages](../docs/pages/enterprise/sso).

### Integration with individual IdPs

We create IdP-specific integration code, which would reduce the expected configuration time for given IdP. The
feasibility of integration is likely to vary greatly depending on specific IdP.

For example, we may use the Okta CLI tool:

- [blog](https://developer.okta.com/blog/2020/12/10/introducing-okta-cli)
- [installation](https://cli.okta.com/)
- [source code](https://github.com/okta/okta-cli).

```bash
$ okta start spring-boot
Registering for a new Okta account, if you would like to use an existing account, use 'okta login' instead.

First name: Jamie
Last name: Example
Email address: jamie@example.com
Company: Okta Test Company
Creating new Okta Organization, this may take a minute:
OrgUrl: https://dev-123456.okta.com
An email has been sent to you with a verification code.

Check your email
Verification code: 086420
New Okta Account created!
Your Okta Domain: https://dev-908973.okta.com
To set your password open this link:
https://dev-908973.okta.com/reset_password/drpFaK66lHuY4d1WbrFP?fromURI=/

Configuring a new OIDC Application, almost done:
Created OIDC application, client-id: 0oazahf9k5LDCx32C4x6

Change the directory:
    cd spring-boot

Okta configuration written to: src/main/resources/application.properties
Don't EVER commit src/main/resources/application.properties into source control

Run this application with:
    ./mvnw spring-boot:run
```

