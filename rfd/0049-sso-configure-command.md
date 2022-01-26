---
authors: Krzysztof Skrzętnicki <krzysztof.skrzetnicki@goteleport.com>
state: draft
---

# RFD 49 - `tctl sso configure` command

## What

This RFD proposes a new subcommand for the `tctl` tool: `sso configure`. This is a convenience command to help with the
configuration of auth connectors for common use cases. The command will prepopulate as much of the config file as
possible given the available information (queried interactively or provided by command line flags). The config file will
be written to standard output or file chosen by the user.

For select providers (e.g. Okta) we will support more feature-rich templating. The ultimate goal is to support all
providers for which we have detailed [SSO how-to pages](../docs/pages/enterprise/sso), but the initial scope will be
smaller.

## Why

We want to simplify the task of configuring a new auth connector by providing an easy-to-follow set of prompts for doing
so. Additionally, the command provides a foundation for creating similar functionality in web-based UI.

## UX

The simplest way to use the command is to invoke it without any arguments. Assuming the command is invoked in an
interactive shell, the desired configuration will be queried interactively.

```bash
$ tctl sso configure
```

The `--preset` flag chooses the template to use. Additionally, each template may define its own flags.

```bash
$ tctl sso configure --preset={google,okta,adfs,saml,oidc,github} --template-specific-flags=...
```

Valid flag combinations (initial implementation scope):

| `--preset`    | description                                                                         |
|---------------|-------------------------------------------------------------------------------------|
|               | Error: missing `--preset`. If possible (interactive TTY) ask user which one to use. |
| `github`      | Ask for `client_id` and `client_secret`.                                            |
| `oidc`        | Generic OIDC template.                                                              |
| `saml`        | Generic SAML template.                                                              |
|               |                                                                                     |
| `okta`        | Okta template (SAML).                                                               |
| `google`      | Google template (SAML).                                                             |
| `adfs`        | ADFS SAML template.                                                                 |
|               |                                                                                     |
| `okta-oidc`   | Okta OIDC template. Fill in details from `.well-known`.                             |
| `google-oidc` | Google OIDC template. Fill in details from `.well-known`.                           |

Individual templates can accept additional non-standard flags. For example `--entity-descriptor=FILE` flag used by SAML
templates will read Entity Descriptor XML from the provided file and properly embed it in the resulting YAML file. The
parameters *may* be subject to validation (e.g. XML structure being correct), but by default incorrect parameters should
only cause warnings to be emitted. In general, we cannot guarantee correctness of generated config; it is expected that
the user will review the config file and test it for correctness. This is especially true for things
like `attributes_to_roles` mappings or the provider side of the configuration. This tool is meant to make the
configuration easier, not replace the appropriate documentation.

Detailed help message can be access with:

```bash
$ tctl sso configure --help
...
```

## Interactive mode

If the terminal attached to stdin is interactive we default to interactive mode. In interactive mode we may provide the
user with choices (e.g. preset) or ask them to provide details (secrets, identifiers...). In non-interactive mode we
always use defaults or return error if no sensible defaults are possible.

The interactive mode may be forced on and off with `--interactive` flag.

## Proxy URL discovery

The command will make an effort to discover the correct proxy URL from client profile. Failing that, an appropriate
default along with comment will be put into the configuration file.

## Initial scope for POC

Only `--preset=okta` and non-interactive mode are in scope of POC. 

## Security

The command will never modify existing Teleport configuration, this will be done by the user by further invocation
of `tctl create` command or by other means.

The command may handle user secrets. The implementation must ensure these are not silently written anywhere (e.g. we
must never save partially filled-in configuration files to temp files).

When asking for secrets we should disable local echo (same as when asking for passwords).

## Further work

### Full provider coverage

We should extend the set of supported providers to match [SSO how-to pages](../docs/pages/enterprise/sso).

### Extending third-party tools to generate Teleport configuration

It may be useful to extend the Okta CLI tool to make it generate the Teleport auth connector configs. More information:

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

