---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 80 - tsh key agent integration

## What

- Enable Teleport users to login and export their session's private key to their ssh-agent rather than disk (at `~/.tsh/keys/proxy/user`)
- Allow Teleport users to login with pre-existing agent keys, including keys in [yubikey-agent](https://github.com/FiloSottile/yubikey-agent)
- Enable Teleport users to login with a PIV smart card (yubikey) using [go-piv](https://github.com/go-piv/piv-go)

## Why

By default, `tsh` creates keys and signs certificates directly to disk in `~/.tsh`. This presents a security vulnerability, as access to a teleport cluster is as simple as exporting a user's `~/.tsh` directory. By integrating a user's key agent fully into the certfifiate signing process, we can prevent that attack vector, since attackers cannot use the certificates without access to the private key safely stored in the user's key agent. This also allows us to integrate yubikey PIV keys into the signing process, so that a Teleport cluster cannot be compromised without first compromising a user's yubikey.

## Details

### ssh-agent login support

Currently, `tsh login` adds private keys to disk in the user's `~/.tsh` profile. `tsh` uses this private key to sign certificates and perform handshakes in subsequent `tsh` calls.

We have a flag, `--add-keys-to-agent=only`, which can be used to add the private key to the user's `ssh-agent` instead, so that the private key cannot exfiltrated. However, this key and all of its certificates are only good for that single `tsh` call, since `tsh` won't find the private key in `~/.tsh` in subsequent calls.

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

`tsh` will be updated to look for agent keys with the label `teleport:proxy_host:teleport_user`. If `tsh` finds an agent key, then it will be used for any certificate signing or tls/ssh handshakes during the `tsh` command execution. If no agent key is found, then `tsh` will default to using the private key stored at `~/tsh/keys/proxy.example.com/user`, or initiate a new login. 

#### Using pregenerated agent keys

With the strategy above, it is possible for a user to create a private key and add it to their `ssh-agent` themselves - `ssh-keygen -t rsa -C "teleport:user" -f ~/.ssh/dev`. As a result, the private key which exists outside of the `ssh-agent` would be used for `tsh` operations. 

To combat this, `tsh` will not trust outside keys by default. Instead, `tsh login` will continue to create the private key itself and add it to the `ssh-agent`. Then, `tsh` will generate the user's ssh and tls certificates using the private key in memory. These certificates will be used to verify that the agent key hasn't been replaced in future calls to `tsh`.

To bypass this, a user can use the new flag, `--use-agent-key="agent-key-name"`, which will enable integrations with tools like `yubikey-agent` or `pivy-agent`. This may be prefered for trusted users in a Teleport cluster, since the private key will never exist outside of the yubikey's PIV, not even in-memory during `tsh login`. 

Note: `yubikey-agent` holds an open connection to yubikey, preventing us from using `piv-go` to access the yubikey at the same time. Most likey, we'll need to investigate `pivy-agent` further, or consider adding our own standalone yubikey agent implementation - `tsh agent`.

### yubikey PIV support

We can use [go-piv](https://github.com/go-piv/piv-go) to access and interact with PIV-compatible yubikeys (Any 5 series yubikey). The libary provides the ability to open connections to a yubikey's smart card application, which can in turn be used to generate/store private keys, sign/store certificates, and perform tls and ssh handshakes.

The libary provides two options for storing yubikey private keys:
 1. [GenerateKey](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.GenerateKey) - Generate a new key directly on the yubikey, which cannot be exported
 2. [SetPrivateKeyInsecure](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.SetPrivateKeyInsecure) - Store a pre-generated key on the yubikey

Both options have their own pros/cons and different implementation challenges, but they are both worth considering as they both provide the desired benefits:
 - `tsh` can use private keys which only exist on the yubikey and never on disk
 - signing certificates and performing handshakes requires physical contact with the yubikey device
 - `tsh` has oversight into the key generating process, and can provide the following key policies
    ```go
    type Key struct {
        // Algorithm to use when generating the key.
        Algorithm Algorithm
        // PINPolicy for the key.
        PINPolicy PINPolicy
        // TouchPolicy for the key.
        TouchPolicy TouchPolicy
    }
    ```

The differences between the two lie in their drawbacks and technical challenges, but could potentially provide the same level of security and UX. Although we will probably choose the `GenerateKey` option, I've decided to go into more detail below so that we can come to an educated decision.

#### `GenerateKey`

`GenerateKey` is ideal from a security perspective, as it will always guarentee that the private key stored in the yubikey only exists inside the yubikey.

The only drawback to this approach is that `ssh-agent` integration is not supported out of the box, because [Adding agent keys from a smartcard](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). As a result, openSSH integration and agent forwarding may not work in `tsh` with this strategy.

There are a few ways that we can work around this issue:
 1. Implement [Adding agent keys from a smartcard](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) ourselves, either in a fork/PR on `x/crypto/ssh/agent` or as a standalone implementation in `tsh`. As detailed in the [rfc](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5), this can be done without access to the raw private key, and instead works by delegating future private key operations in the `ssh-agent` to the yubikey instead. This can be done manually [using `ykman` and `ssh-add`](https://github.com/jamesog/yubikey-ssh), and probably wouldn't be hard to add real support for in `tsh`.
 2. Add logic into `tsh proxy ssh` to integrate yubikey support. This is a simple solution that could potentially work around openSSH integration issues, but would not fix other agent-related issues like agent forwarding.

#### `SetPrivateKeyInsecure`

Although named `SetPrivateKeyInsecure`, if we use it carefullly, we can get a similar level of security. The main benefit of this strategy is that when we store the key on the yubikey, we can also store the key in the user's ssh-agent. The ssh-agent key would only be used for ssh-agent and openSSH integrations.

The primary concerns with `SetPrivateKeyInsecure` mentioned in the libary are the following:

**There's no way to prove the key wasn't copied, exfiltrated, or replaced with malicious material before being imported**

Since `tsh` will have full control over the key generation and certificate signing process, we should be able to work around this. Specifically, during `tsh login`, we can hold a connection to the yubikey, store a new key, and immediately sign the user's tls and ssh certificates with the key. This process guarentees that the certificates were generated with the new key and circumvents the above concern.

In future calls to `tsh`, we can depend on these certificates to catch any yubikey shananigans which could take place after the initial login. This would require more investigation to ensure that we can properly guarentee that any malicious activity would lead to `tsh` rejecting any future actions.

**It is not explicitly supported and may not provide all the same functionality (particularly Attestation)**

We should still have access to the primary methods we need, being those for generating certificates and using them in handshakes. However, this would need a bit more thorough investigation.

#### Storing Certificates on yubikey

With both of the above key storage methods, we can store a single certificate alongside the private key. Since we can't store every certificate signed by `tsh`, we will just store the user's `x509` certificate used for tls handshakes. In Teleport clusters with TLS routine enabled, this certificate is needed to perform any `tsh` action.

### Security

When `--add-keys-to-agent=only` is used, it becomes much harder to compromise the private key, as it will only be available in the key agent running on the user's device. This still doesn't prevent hacked/stolen machines from accessing Teleport Clusters, but when combined with a yubikey as the key holder, the only way to compromise access would be to gain access to the user's physical yubikey and device.

Note to reviewers: below is an optional addition to enforce key agent usage across a cluster, and needs further investigation if we want to include it.

In cases where server admins want to be sure that no private key is never on disk for any Teleport user, they should be able to configure it in their `cluster_auth_preference`, so we will add the `add_keys_to_agent_only` field. When set to `yes`, all tsh login attempts will use `--add-keys-to-agent=only`. Question: should/can this include logins in the webui?

This can also be set on a per-role basis, with new the role option `add_keys_to_agent_only`.

Note: a user could potentially avoid this check by using an old or compromised version of `tsh`.

### UX

#### new or relevant `tsh` flags:
| Name | Default Value(s) | Allowed Value(s) | Description |
| `-k, --add-keys-to-agent` | auto | auto no yes only | Controls whether login keys are added to `~/.tsh`, system agent, or both. |
| (new) `--use-agent-key` | none | string | The name of an agent key or key device to use when logging in. |

When `--use-agent-key` is used, `--add-keys-to-agent` will be ignored, since the key is already in the agent and cannot be exported to disk.

When `add_keys_to_agent_only` is set in a cluster's auth preference or the user's role, `--use-agent-key` will be ignored/denied.

#### Multiple yubikeys

In the case where the user has multiple yubikeys connected at the same time, we will default to using the first one for now. In the future, we can add support for multiple yubikeys, and potentially multiple slots on each yubikey, so that user's can have more than one concurrent yubikey login.

#### TBD

A user is required to tap their yubikey for every certificate sign and handshake. This will require much more UX consideration to handle properly. I'm leaving this as TBD while we focus on the technical specification.

### Additional considerations

- Yubikey 5 has 5 different [key slots](https://docs.yubico.com/hardware/yubikey/yk-5/tech-manual/yk5-apps.html#slot-information) to choose from. `Slot 9a: PIV Authentication` is the most fitting option, and is the same slot used by `yubikey-agent`. We may want to consider using more slots as mentioned above.
- PIV integration also provides us with the ability to perform [attestation](https://docs.yubico.com/hardware/yubikey/yk-5/tech-manual/yk5-apps.html#attestation). It's not immediately clear to me how we can take advantage of this, but it could be integrated in the future.
