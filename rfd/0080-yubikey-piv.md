---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 80 - yubikey PIV integration

## What

We will integrate yubikey smart card capabilities into the Teleport login process so that users can login with a yubikey generated private key. Teleport admins will be able to require yubikey login with optional PIN and touch policies for each cluster and role.

## Why

Currently, Teleport clients create their own public-private key pairs to be signed by the Teleport Auth server. The private key must be stored for future handshakes. In most cases, the private key is stored in disk alongside the certificates signed for it, where it can potentially exported.

For example, `tsh login` stores the private key in `~/.tsh/keys/proxy/user` and stores certs in `~/.tsh/keys/proxy`, so a user's login session is fully contained in their `~/.tsh` folder. As a result, if someone gains access to a Teleport user's computer, they would immediately have access to the user's Teleport login session. 

Using a hardware backed private key like yubikey, we can prevent such vulnerabilities. In order to gain access to the Teleport login session's private key, you would need to have access to the user's certificates on disk, their physical yubikey, and the ability to pass the yubikey's touch and pin policies.

Additionally, Teleport currently has no way to determine the origins of a private key it signs, and simply trusts that it will be stored securely on the client side. With yubikey private keys, Teleport Auth servers will able to perform [Attestation](https://docs.yubico.com/yesdk/users-manual/application-piv/attestation.html) to confirm that a public key was securely generated in a yubikey before signing certificates for the key.

## Details

Yubikeys (Series 5 and on) have a built in smart card which provides PIV compatibility, which we can interact with using the [go-piv library](https://github.com/go-piv/piv-go).

### yubikey login

We'll add the `tsh` flag `--yubikey/-y` so that users can login with the new yubikey login flow. The majority of the login flow is the same, except the key generation and certificate signing process.

#### Generate yubikey key pair and certificates

When `tsh login` would usually generate a new private key in memory, it will instead use [`piv.GenerateKey`](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.GenerateKey) to generate a new key pair directly on the yubikey's PIV Authentication slot (`9a`). The certificate signing process will continue as usual, using the yubikey's public key, and the certs will be stored in `~/.tsh/keys/proxy` as usual.

Neither the yubikey's public or private key will be stored, as the private key can't be exported and we can later access the public key by checking the yubikey's [slot cert](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.Attest).

##### Yubikey PIN/Touch policies

When we generate a new private key, we can provide it with a custom Touch and PIN policy. These policies will be enforced whenever the private key is used for signing or decrypting, which may occur multiple times for each `tsh` command.

The available PIN policies are:
 - `never`: never prompt the user for PIN
 - `always`: always prompt the user for PIN
 - `once`: prompt the user for PIN just once per session, where a session is an open connection to the yubikey (needs verification)
   - Due to the current keystore implementation reloading keys multiple times for every command, this may still lead to the user being prompted for PIN multiple times in each tsh call. This can potentially be reworked, but it will most likely not be fixed in the initial implementation.

The available Touch policies are:
 - `never`: never prompt the user for touch
 - `always`: always prompt the user for touch
 - `cached`: cache user's touch for 15 seconds before prompting them for it again
   - This should result in each `tsh` command requiring just one touch
   - We can try to control when the user is prompted for touch by doing a no-op signature at the start of a `tsh` command. However, if the cached touch runs out during the command, the user may be prompted later in the command execution than desired.

The user can provide `--yubikey-pin-policy=<never|always|once>` and `--yubikey-touch-policy=<never|always|cached>` to specify their desired policies. They will both default to `never`.

##### PIN Prompt

Once the key is generated, we will need to prompt the user to enter a new PIN for it. If the PIN policy is `never`, then this prompt will be skipped.

TBD: need to flush out how this will interact with webauthn/etc.

##### Management Key

TBD

### yubikey login session

Since `tsh` cannot detect a private key on disk like usual, we'll need to provide a way for `tsh` to reaccess its login session in another way.

On yubikey login, `tsh` will add `yubikey: true` to the user's `~/.tsh/proxy.yaml` file so that `tsh` knows to use the user's yubikey for subsequent `tsh` commands. If the yubikey or its private key is not found or does not match the certificates on file, the user will be receive an error or be prompted to re-login like usual.

### Server Enforcement

In order to make this functionality from the server's perspective, we need to add ways to enforce yubikey login and guarentee its proper usage. This can be done with [Attestation](https://docs.yubico.com/yesdk/users-manual/application-piv/attestation.html), which we can use to verify the certificate chain from the yubikey's public key to the yubikey's CA.

For this [verification](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#Verify) to take place on the server, we will need to pass the user's yubikey [attestation certificate](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#YubiKey.AttestationCertificate), as well as an attestation statement [slot cert](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#YubiKey.Attest) for the slot in question. Once this verification succeeds, we can be positive that the public key in the slot cert belongs to a yubikey-backed private key. If this public key matches the one the Auth Server is preparing to sign, then it can safely continue to do so.

To pass the attestation certificates to the Auth Server, they will be added to the login endpoints, including `/webali/ssh/certs`, `/webapi/mfa/login/finish`, `/webapi/<sso>/callback`.

Additionally, the attestation will return useful information about the user's yubikey and the private key in question, including:
 - The private key's PIN policy
 - The private key's Touch policy
 - The yubikey serial number

#### Auth Service Config or Auth Preference

To require yubikey login for all users, you can set the `require_yubikey_login` field in the Cluster's Auth Preference, either through the Auth Service's config file:

```yaml
auth_service:
  ...
  authentication:
    ...
    yubikey_login: 
      required: true
      pin_policy: always
      touch_policy: always
```

Or with a dynamic Cluster Auth Preference object:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  yubikey_login: 
    required: true
    pin_policy: always
    touch_policy: always
```

If `yubikey_login_required` is `false`, the policy fields will be ignored and attestation will be skipped.

When `yubikey_login_required` is `true`, then the server will perform Attestation and verify the passed public key.

If `pin_policy` or `touch_policy` is set, then the Auth server will check the [`Attestation`](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#Attestation) object's `PINPolicy` or `TouchPolicy` fields. If the checked fields are at least as strict as the policies required, then the request will proceed.

Examples:
 - `pin_policy=once` + `PINPolicy=always` = success
 - `touch_policy=always` + `TouchPolicy=always` = success
 - `touch_policy=cached` + `TouchPolicy=never` = failure

#### Role Options

To make yubikey login required or optional on a per-role basis, you can set the same options in a Role.

```yaml
kind: role
version: v5
metadata:
  name: role-name
spec:
  yubikey_login: 
    required: true
    pin_policy: once
    touch_policy: cached
```

#### Combined yubikey requirements

Each user's yubikey requirements will be determined by:
 1. Combining their role options to form the strictest setting
 2. Combining the strictest role options settings with the cluster auth preference, with role options taking precedence (if set).

Note: A user's yubikey requirement settings cannot be easily determined before generating the yubikey private key and locking in its pin/touch policy settings, which occurs before logging in. Thus, it is the responsibility of the user to figure out their own pin/touch policy settings before logging in, or else they will get `access denied` errors.

#### Examples:

Example 1:
 - All users must use yubikey login
 - Users with the `admin` role must also use `pin=always` and `touch=always` (to secure adminstrative infrastructure)
 - Users with the `superadmin` role are not required to use yubikey login (superadmin roles are inherently unsafe, might as well own it)
```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  yubikey_login: 
    required: true
---
kind: role
version: v5
metadata:
  name: admin
spec:
  yubikey_login: 
    pin_policy: always
    touch_policy: always
---
kind: role
version: v5
metadata:
  name: superadmin
spec:
  yubikey_login: 
    required: false
```

Example 2:
 - Users with `yubikey-pin` role must use yubikey login with `pin=always` and `touch=cached|always`
 - Users with `yubikey-touch` role must use yubikey login with `pin=once|always` and `touch=always`
 - Users with both the `yubikey-pin` and `yubikey-touch` roles must use yubikey login  with `pin=always` and `touch=always`
 - Users with neither the `yubikey-pin` nor `yubikey-touch` roles are not required to use yubikey login
```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  yubikey_login: 
    required: false
    pin_policy: once
    touch_policy: cached
---
kind: role
version: v5
metadata:
  name: yubikey-pin
spec:
  yubikey_login: 
    required: true
    pin_policy: always
---
kind: role
version: v5
metadata:
  name: yubikey-touch
spec:
  yubikey_login: 
    required: true
    pin_policy: never
```

### UX

#### yubikey slots

Yubikey 5 has 5 different [key slots](https://docs.yubico.com/hardware/yubikey/yk-5/tech-manual/yk5-apps.html#slot-information) to choose from, one of which is reserved for attestation. `Slot 9a: PIV Authentication` is the most fitting option, and is the same slot used by `yubikey-agent`.

If the slot is already in use, we will provide the user with a prompt to reuse this key, overwrite it, or abort. Users can also provide the `--yubikey-overwrite` or `--yubikey-reuse` flags to bypass this prompt.

```
> tsh login

yubikey slot 9a has a pre-generated private key with public key:
-----BEGIN PUBLIC KEY-----
...
-----END PUBLIC KEY-----

Would you like to:
 (1) Resuse this key
 (2) Overwrite this key
 (3) Abort login
```

In the case of a Teleport user connecting to multiple clusters, they'll need to use `--yubikey-reuse` in order to maintain access to them simultaneously.

Note that reusing the key is not any less secure than generating a new one, as the Teleport Auth server can still attest that the key was generated on the yubikey, and therefore cannot be exported. 

#### new `tsh` flags

| Name | Default Value(s) | Allowed Value(s) | Description |
| `-y, --yubikey` | - | - | During login, use yubikey to generate and store private key data for your login session. |
| `--yubikey-pin-policy` | never | always once never | Can be provided with `tsh login -y` to control how often you need to enter your yubikey PIN to access the yubikey private key data. |
| `--yubikey-touch-policy` | never | always cached never | Can be provided with `tsh login -y` to control how often you need to touch your yubikey to access the yubikey private key data. |
| `--yubikey-overwrite` | - | - | When provided, `tsh login -y` will overwrite existing yubikey data if it exists. |
| `--yubikey-reuse` | - | - | When provided, `tsh login -y` will reuse existing yubikey data if it exists. Useful when using yubikey to connect to multiple clusters simultaneously. |

#### tsh config file

Users can also provide their yubikey login settings in their tsh config file (`/etc/tsh.yaml` or `~/.tsh/config/config.yaml`). This is important so that users don't need to provide the flag options on every login.

```yaml
yubikey: true
yubikey_pin: once
yubikey_touch: cached
yubikey_reuse: true
```

#### Multiple yubikeys

If the user has multiple yubikeys connected, they will receive an error asking them to connect just one yubikey. Otherwise, we can't guarentee that the right yubikey will be chosen. We may be able to add support for this in the future.

### Additional considerations

- At first, yubikey login support will only be added to `tsh`, but in the future other clients like `tctl` and the WebUI can support it with the same general flow. 
  - When `yubikey_login_required` is enforced on the Auth Server, we can expect other Teleport clients to fail to login unless we add yubikey login support for them as well. Initial implementation will most likely suffer from this deficiency.
- Initially, yubikey login will not support `tsh --add-keys-to-agent` or `tsh -A`, because [Adding agent keys from a smartcard](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). We can implement this support ourselves in the future.
- Storing certificates on the yubikey is possible, but we can only store one certificate per slot. Since in most cases we need both an ssh and tls certificate, this functionality would not provide us with much benefit, and it's much simpler to store them in the usual locations on disk. When integrating ssh agent functionality, we may need to revisit this.
- It's possible that this can be integrated more closely with the existing webauthn flow to get a better UX flow, but I am not familiar enough with that code to know how this could be done.