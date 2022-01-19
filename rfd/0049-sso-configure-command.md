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

Users provide choices using command line flags. The `--kind` flag determines the kind of auth connector,
while `--preset`
chooses template suited for particular vendor:

```bash
$ tctl sso configure --kind={oidc,saml,github} --preset={google,okta,adfs,generic} --interactive --template-specific-flags=...
```

Valid flag combinations (initial implementation scope):

| `--kind` | `--preset` | interactive behavior                                      | batch behaviour                    |
|----------|------------|-----------------------------------------------------------|------------------------------------|
|          |            | Ask for `--kind` to use.                                  | Exit with error: missing `--kind`. |
| `github` |            | Ask for `client_id` and `client_secret`.                  |                                    |
| `oidc`   |            | Ask for `--preset`.                                       | Default to `--preset=generic`.     |
| `oidc`   | `generic`  | Generic OIDC template.                                    |                                    |
| `oidc`   | `okta`     | Okta OIDC template. Fill in details from `.well-known`.   |                                    |
| `oidc`   | `google`   | Google OIDC template. Fill in details from `.well-known`. |                                    |
| `saml`   |            | Ask for `--preset`.                                       | Default to `--preset=generic`.     |
| `saml`   | `generic`  | Generic SAML template.                                    |                                    |
| `saml`   | `okta`     | Okta SAML template.                                       |                                    |
| `saml`   | `google`   | Google SAML template.                                     |                                    |
| `saml`   | `adfs`     | ADFS SAML template.                                       |                                    |

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

## Interactive and batch modes

We support two modes: interactive and batch (non-interactive). In interactive mode we may provide the user with
choices (e.g. preset) or ask them to provide details (secrets, identifiers etc.) In batch mode we never ask but use
defaults (when possible) or insert comments asking for the data to be filled manually afterwards. Some

Interactive mode is used when we detect TTY or `--interactive` flag is provided, batch mode is used otherwise.

## Proxy URL discovery

The command will make an effort to discover the correct proxy URL. Failing that, an appropriate default along with
comment will be put into the configuration file.

## Implementation details

Each combination of `--kind` and `--provider` will be mapped to single template based
on [text/template](https://pkg.go.dev/text/template) package. The extension points will be handled by calls to functions
or conditionals, with functions handling the input fetching and state management.

## Security

The command will never modify existing Teleport configuration, this will be done by the user by further invocation
of `tctl create` command or by other means.

The command may handle user secrets. The implementation must ensure these are silently written anywhere (e.g. we must
not save partially filled-in configuration files to temp files).

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

