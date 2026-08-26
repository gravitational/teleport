---
authors: Russell Jones (rjones@goteleport.com)
state: implemented
---

# RFD 18 - Agent loading.

## Why

[#3169](https://github.com/gravitational/teleport/issues/3169) raised UX concerns about how `tsh` loads keys into an SSH agent.

* `tsh` will by default attempt to load the users certificate upon login into any running agent. This breaks some SSH agents like `gpg-agent` that do not support certificates.
* `tsh` has a flag that can be used in multiple ways for certificate loading behavior that users have found confusing.
* `tsh` always writes certificates to disk, users have asked to only load certificates into the running agent.

Furthermore, users have legitimate reasons for wanting to control this behavior (hence the flags). They may want to force load a certificate (our automatic detection algorithm may be incorrect) or not (the user does not want to load Teleport certificates into their agent).

## What

By default `tsh login` should attempt to load the fetched certificate into the agent. If `tsh` thinks the agent does not support SSH certificates, like `gpg-agent`, Teleport should not load the certificate and instead print a warning informing the user the certificate was not loaded and how to force load.

To make complex things possible a `--add-keys-to-agent` (short flag `-k`) flag should be added to `tsh` that can be used to control if `tsh` attempts to load a certificate into the running agent or not.

This matches the [AddKeysToAgent](https://man.openbsd.org/ssh_config.5#AddKeysToAgent) option that OpenSSH supports:

```
AddKeysToAgent

Specifies whether keys should be automatically added to a running ssh-agent(1). If this option is set to yes
and a key is loaded from a file, the key and its passphrase are added to the agent with the default lifetime,
as if by ssh-add(1). If this option is set to ask, ssh(1) will require confirmation using the SSH_ASKPASS
program before adding a key (see ssh-add(1) for details). If this option is set to confirm, each use of the
key must be confirmed, as if the -c option was specified to ssh-add(1). If this option is set to no, no keys
are added to the agent. Alternately, this option may be specified as a time interval using the format
described in the TIME FORMATS section of sshd_config(5) to specify the key's lifetime in ssh-agent(1),
after which it will automatically be removed. The argument must be no (the default), yes, confirm (optionally
followed by a time interval), ask or a time interval.
```

### Details

The `--add-keys-to-agent` (short flag `-k`) should support following values `auto`, `only`, `yes`, `no`. The default value should be `auto`.

* `auto` indicates that a best effort attempt will be made to detect the agent, and if it supports SSH certificates, load the certificate into the running agent.
* `only` indicates that keys will only be loaded into the running agent and not written to disk. It's behavior is `yes` in that no detection logic will be used.
* `yes` indicates that an attempt will be made to load the certificate into the running agent.
* `no` indicates that no attempt will be made to load the certificate into the running agent.

#### Detection Algorithm

The initial detection algorithm should be simple. `tsh` should read in the `$SSH_AUTH_SOCK` environment variable. If the path contains a string that indicates the agent does not support SSH certificates, like `gpg-agent`, it will not attempt to load the certificate into the agent.

#### Backward Compatibility

The existing `use-local-ssh-agent` flag will be hidden and depreciated but will continued to be supported forever to not break existing users workflow.

The existing flag will be mapped to the following values:

| use-local-ssh-agent  | add-keys-to-agent |
| -------------------- | ----------------- |
| true                 | auto              |
| false                | false             |

###  Examples

For most users, not specifying the `add-keys-to-agent` (where the default `auto` is used) is the behavior they want and expect. For example, for `ssh-agent`.

```
$ tsh --proxy=proxy.example.com login
[...]

$ ssh-add -l
2048 SHA256:HiqSRCpljSaGt7eHCxlGGlCHcqRxSJ1vu5K7Sgl9pp4 teleport:rjones (RSA-CERT)
2048 SHA256:HiqSRCpljSaGt7eHCxlGGlCHcqRxSJ1vu5K7Sgl9pp4 teleport:rjones (RSA)
```

If `gpg-agent` is being used no keys will be loaded.

```
$ SSH_AUTH_SOCK=$(gpgconf --list-dirs agent-ssh-socket)

$ tsh --proxy=example.com login
[...]
Warning: Certificate was not loaded into agent because the agent at SSH_AUTH_SOCK does not appear
to support SSH certificates. To force load the certificate into the running agent, use
the --add-keys-to-agent=yes flag.

$ gpg-connect-agent 'keyinfo --list' /bye
OK
$
```
