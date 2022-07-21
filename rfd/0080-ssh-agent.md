---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 80 - tsh key agent integration

## What

- Enable Teleport users to login with `ssh-agent` without exporting the private key to disk (at `~/.tsh/keys/proxy/user`)
- Enable Teleport users to login with pre-existing agent keys, such as yubikey agent key served by `yubikey-agent`
- Enable Teleport users to login with a PIV smart card (yubikey) using [go-piv](https://github.com/go-piv/piv-go)

## Why

By default, `tsh` creates keys and signs certificates directly to disk in `~/.tsh`. This presents a security vulnerability, as access to a teleport cluster is as simple as exporting a user's `~/.tsh` directory. By integrating a user's key agent fully into the certfifiate signing process, we can prevent that attack vector, since attackers cannot use the certificates without access to the private key safely stored in the user's key agent. This also allows us to integrate yubikey PIV keys into the signing process, so that a Teleport cluster cannot be compromised without first compromising a user's yubikey.

## Details

### ssh-agent login support

Currently, `tsh login` adds private keys to disk in the user's `~/.tsh` profile. Then certificates are signed and used using the private key on disk in subsequent `tsh` calls.

We have a flag, `--add-keys-to-agent=only`, which can be used to add the private key to the user's `ssh-agent` instead. However, this key and all of its certificates are only good for that single `tsh` call, since `tsh` won't find the private key in `~/.tsh` in subsequent calls.

```bash
# the next call to tsh will require you to log in again
> tsh ls --add-keys-to-agent=only
Enter password for Teleport user dev:
...
# the next call to tsh will require you to log in again
> tsh ls
Enter password for Teleport user dev:
...
```

`tsh` will be updated to look for agent keys with the label `teleport:user` (or `teleport:cluster_name:user` after https://github.com/gravitational/teleport/pull/14677). If `tsh` finds an agent key, then it will be used for any certificate signing or tls/ssh handshakes during the `tsh` command execution. If no agent key is found, then `tsh` will default to using the private key stored at `~/tsh/keys/proxy.example.com/user`. 

#### Using pregenerated agent keys

With the strategy above, it is possible for a user to create a private key and add it to their `ssh-agent` themselves - `ssh-keygen -t rsa -C "teleport:user" -f ~/.ssh/dev`. As a result, the private key which exists outside of the `ssh-agent` would be used for `tsh` operations. 

To combat this, `tsh` will not trust outside keys by default. Instead, `tsh login` will continue to create the private key itself and add it to the `ssh-agent`. Then, `tsh` will generate the user's ssh and tls certificates using the private key in memory. These certificates will be used to verify that the agent key hasn't been replaced in future calls to `tsh`.

To bypass this, a user can use the new flag, `--use-agent-key="agent-key-name"`, which will enable integrations with tools like `yubikey-agent` or `pivy-agent`. This may be prefered for trusted users in a Teleport cluster, since the private key will never exist outside of the yubikey's PIV, not even in-memory during `tsh login`. 

### yubikey PIV support

[go-piv](https://github.com/go-piv/piv-go) can be used to integrate directly with a yubikey to create/store private keys, sign/store certificates, and perform tls and ssh handshakes.

#### Generating yubikey private keys
 
We can generate new private keys directly on the yubikey in `tsh login`. This would be ideal, since the private key will never exist outside of the yubikey and cannot be exported, and `tsh` would have oversight into the key generating process.

Unfortunately, [Adding agent keys from a smartcard](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). As a result, openSSH integration and agent forwarding would not work in `tsh` with this strategy.

We could potentially implement this ourselves in a fork/PR on `x/crypto/ssh/agent` or as a standalone implementation in `tsh`. It's hard to gauge how much work this would involve, but it may be worth more investigation.

#### Storing tsh-generated private keys

Given the issues outlined above, another option is to generate the private key in `tsh`, but then import it directly into yubikey for future use. At the same time, we can add it to the user's key agent. Then, `tsh` can prioritize using the yubikey to sign certs and perform handshakes, but fall back to the key agent when needed for openSSH integration and agent forwarding.

#### Storing Certificates on yubikey

With both of the above key storage methods, we can store a single certificate alongside the private key. Since we can't store every certificate signed by `tsh`, we will just store the user's `x509` certificate used for tls handshakes. In Teleport clusters with TLS routine enabled, this certificate is needed to perform any `tsh` action (double check this, what about app/etc certs).

#### UX

new/relevant `tsh` flags:
| Name | Default Value(s) | Allowed Value(s) | Description |
| `-k, --add-keys-to-agent` | auto | auto no yes only | Controls whether login keys are added to `~/.tsh`, system agent, or both. |
| (new) `--use-agent-key` | none | string | The name of an agent key to use when logging in. If empty, `tsh` will generate an agent key itself. |

In the case where the user has multiple yubikeys connected at the same time, we will default to using the first one. A user can use

Q: How will `tsh` determine which yubikey to use, in the case of multiple keys? 
A:`yubikey-agent` currently defaults to the first one listed, we can do the same for now.

### Security

When `--add-keys-to-agent=only` is used, it becomes much harder to compromise the private key, as it will only be available in the key agent running on the user's device. This still doesn't prevent hacked/stolen machines from accessing Teleport Clusters, but when combined with a yubikey as the key holder, the only way to compromise access would be to gain access to the user's yubikey and device.

Note to reviewers: below is an optional addition to enforce key agent usage across a cluster.

In cases where server admins want to be sure that no private key is ever exported by any Teleport user, they should be able to configure it in their `cluster_auth_preference`, so we will add the `add_keys_to_agent_only` field. When set to `yes`, all tsh login attempts will use `--add-keys-to-agent=only`. Question: should/can this include logins in the webui?

This can also be set on a per-role basis, with new the role option `add_keys_to_agent_only`.

Note: a user could potentially avoid this check by using an old or compromised version of `tsh`.

### UX

#### new or relevant `tsh` flags:
| Name | Default Value(s) | Allowed Value(s) | Description |
| `-k, --add-keys-to-agent` | auto | auto no yes only | Controls whether login keys are added to `~/.tsh`, system agent, or both. |
| (new) `--use-agent-key` | none | string | The name of an agent key or key device to use when logging in. |

When `--use-agent-key` is used, `--add-keys-to-agent` will be ignored, since the key is already in the agent and cannot be exported to disk.

When `add_keys_to_agent_only` is set in a cluster's auth preference or the user's role, `--use-agent-key` will be ignored/denied.

#### TBD

A user is required to tap their yubikey for every certificate sign and handshake. This will require much more UX consideration to handle properly. Leaving this as TBD while we focus on the technical specification.

### Additional considerations

- Yubikey 5 has 5 different [key slots](https://docs.yubico.com/hardware/yubikey/yk-5/tech-manual/yk5-apps.html#slot-information) to choose from. `Slot 9a: PIV Authentication` is the most fitting option, and is the same slot used by `yubikey-agent`.
- PIV integration also provides us with the ability to perform [attestation](https://docs.yubico.com/hardware/yubikey/yk-5/tech-manual/yk5-apps.html#attestation). It's not immediately clear to me how we can take advantage of this, but it could be integrated in the future.
