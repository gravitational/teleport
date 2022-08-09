---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 80 - PIV smart card integration

## Required approvers

Engineering: @jakule && @r0mant && @codingllama
Product: @klizhentas && @xinding33
Security: @reedloden

## What

We will integrate PIV smart card support so that PIV-compatible hardware keys, like the Yubikey Series 5 cards, can be used to securely generate secrets. These secrets never exist outside of the hardware key, and require PIN/Touch to be accessed for cryptographic operations. We will also make it possible for Teleport admins to enforce PIN and touch policies for their users

## Why

Currently, Teleport clients create their own public-private key pairs to be signed by the Teleport Auth server. The private key must be stored for future handshakes. In most cases, the private key is stored in disk alongside the certificates signed for it, where it can potentially exported.

For example, `tsh login` stores the private key in `~/.tsh/keys/proxy/user` and stores certs in `~/.tsh/keys/proxy`, so a user's login session is fully contained in their `~/.tsh` folder. As a result, if someone gains access to a Teleport user's computer, they would immediately have access to the user's Teleport login session. 

Using a PIV-stored private key, we can prevent such vulnerabilities. In order to gain access to the Teleport login session's private key, you would need to have access to the user's certificates on disk, their physical hardware key, and the ability to pass its PIN and touch challenges.

## Details

### PIV

Personal Identity Verification (PIV), described in [FIPS-201](https://csrc.nist.gov/publications/detail/fips/201/3/final) and defined by [NIST SP 800-73](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf), is an open standard for smart card access. The general idea of PIV is that a central authority serves out private key and certificate data through individual smart cards, which in turn can be used with PIN and touch activation to perform access. 

The primary capabilities of PIV which make it desirable to us are the following:
 - generate and store private keys directly on a smart card
 - attest that generated private keys are generated on a trusted hardware key
 - perform cryptographic operations with a hardware key
 - require PIN and touch policies for cryptographic operations
 - native support for all relevant OS's

We will use the [go-piv](https://github.com/go-piv/piv-go) library, which is a Golang port of Yubikey's C library [ykpiv](https://github.com/Yubico/yubico-piv-tool/blob/master/lib/ykpiv.c). This is the same library used by [yubikey-agent](https://github.com/FiloSottile/yubikey-agent).

#### PKCS#11

PIV builds upon the more common cryptographical token interface ([PKCS#11](http://docs.oasis-open.org/pkcs11/pkcs11-base/v2.40/errata01/os/pkcs11-base-v2.40-errata01-os-complete.html#_Toc441755758)) which is used in cryptographical hardware devices, including HSMs, TPMs, and PIV-compatible smartcards. We could use PKCS#11 instead of PIV to support hardware-backed private keys across a broader range of hardware devices, but we would lose some key PIV features which we plan to make use of:
 - PIV provides additional useful functionality, including generating new certificates, setting a key's PIN and other management data
 - PIV provides a useful standard for PIN and touch requirements, which will give us a universal UX across PIV devices
 - new hardware keys are likely to implement a PIV module (Such as the [solo2](https://solokeys.com/blogs/news/update-on-our-new-and-upcoming-security-keys))

If we want to expand support for non-PIV cryptographical devices in the future, we could add this functionality separately. TPM support in particular could be used to broaden hardware-backed key support to a wide array of modern computers, including all Windows 11 systems.

#### Non-yubikey device compatibility

Currently, Yubikey is one of the only PIV-compatible commercial hardware keys. As a result, current PIV implementations are specifically designed around Yubikey's implementation of PIV - the `libykcs11.so` module. While the majority of PIV is standardized, `libykcs11.so` has [some extensions](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html) and other idiosyncracies which may not be standard accross future PIV implemenations.

Unfortunately, there is no common PIV library, so our best option is to use `piv-go` for a streamlined implemenation and prepare to adjust in the future as more PIV-compatible hardware keys are released. Possible adjustments include:
 - using multiple PIV libraries to support custom PIV implementations
 - switching to a PIV library which expressly supports all/more PIV implemenations
 - working within a PIV library, through PRs or a Fork, to expand PIV support
 - creating our own custom PIV library which we can add custom support into as needed

Note: the adjustments above will largely be client-side and therefore should not pose any backwards compatibility concerns.

### tsh PIV integration

We'll add the `--piv` flag so that users can use `tsh login --piv` to login with their PIV card. The majority of the login flow is the same, except the key generation and certificate signing process.

When `tsh login` would usually generate a new private key in memory, it will instead use [`piv.GenerateKey`](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.GenerateKey) to generate a new key pair directly on the PIV Authentication slot (`9a`). As usual, the certificate signing process will use the public key to generate and store certs in `~/.tsh/keys/proxy`.

Neither the public or private key will be stored, as the private key can't be exported and we can later access the public key by checking the hardware key's [slot cert](https://pkg.go.dev/github.com/go-piv/piv-go/piv#YubiKey.Attest).

#### PIN and touch policies

When we generate a new private key, we can provide it with a custom touch and PIN policy. These policies will be enforced whenever the private key is used for signing or decrypting, which may occur multiple times for each `tsh` command.

The available PIN policies are:
 - `never`: never prompt the user for PIN
 - `always`: always prompt the user for PIN
 - `once`: prompt the user for PIN just once per session, where a session is an open connection to the PIV module
   - Due to the current keystore implementation reloading keys multiple times for every command, this may still lead to the user being prompted for PIN multiple times in each tsh call. This can potentially be reworked, but it will most likely not be fixed in the initial implementation.

The available touch policies are:
 - `never`: never prompt the user for touch
 - `always`: always prompt the user for touch
 - `cached`: cache user's touch for 15 seconds before prompting them for it again
   - This should result in each `tsh` command requiring just one touch
   - This could occur in the middle of a `tsh` command since we don't have control over when the touch is cached/expires.

The user can provide `--piv-pin-policy=<never|always|once>` and `--piv-touch-policy=<never|always|cached>` to specify their desired policies.

#### PIV slots

PIV cards are required to have 4 primary slots, as well as the 20 retired key management slots. 

Each slot has a different intended function and different default PIN and touch policies. Since we are manually setting the PIN and touch policies ourselves, we can technically use any of the 24 slots available in the same way.

We will use slot `9a: PIV Authentication` by default, as it fits the teleport use case most closely. However, users will be able to provide their own desired slot with the flag `--piv-slot`, in case they are already using slot `9a` for a different login session or for a different PIV application, such as `yubikey-agent`.

Note: Yubikeys also have an attestation slot, `f9`, which is not standardized by the PIV specifications.

#### tsh PIV login session

Usually, `tsh` checks the user's `~/.tsh/keys/proxy.example.com` for an existing public-private key pair. If found, that key pair will be used alongside any stored certificates to handle `tsh` calls.

Since we can't store the PIV private key on disk, we will instead store a fake private key which we can use to identify the PIV slot to load the private key from. This key will be PEM encoded data where, `pem.Type` specifies the PIV key type, and the encoded data contains a unique ID and PIV slot.

Note: for yubikey, the unique ID will be the card's serial number. Different implementations of PIV may provide alternative methods for identifying unique cards

```
-----BEGIN PIV YUBIKEY PRIVATE KEY-----
# PEM encoded `yubikey_serial_number+piv_slot`
-----END PIV YUBIKEY PRIVATE KEY-----
```

##### Check the public key

It's possible that after login, the user overwrites the PIV slot used to login. To catch this, future calls to `tsh` will retrieve the public key from the PIV slot and check if it matches the public key on the user's TLS certificate (`~/.tsh/keys/proxy/user-x509.pem`). If it doesn't match, then the user will see an error and get prompted to relogin.

```
> tsh ls
ERROR: PIV private key does not match login certificates.
// Trigger relogin
```

##### Concurrent login sessions

`tsh` should support multiple concurrent login sessions for different proxies. This can be done by specifying a different slot for each concurrent login session, allowing up to 24 concurrent PIV login sessions. To make it easier for `tsh` to track used slots, and avoid overwriting slots that are already in use, `tsh login --piv` will also create a self-signed certificate with login metadata attached and add it to the slot.

```
Slot 9a:
    Certificate:
        Subject DN:	CN = yubikey, O = tsh, OU = v10.0.0, L = proxy.example.com:user
```

Now, future calls to `tsh login --piv --piv-slot=9a` can retrieve the certificate from the slot and decide what to do:
 - if there is no certificate, overwrite the slot
 - if there is a tsh-generated certificate that matches the current user and proxy, overwrite the slot
 - if there is a tsh-generated certificate that doesn't match the current user and proxy, prompt the user to overwrite. If they say no, then prompt them to use the next open slot.
  ```
  > tsh login --piv
  PIV slot 9a is currently in use by another tsh login session:
    proxy: proxy.example.com
    user: username
  Would you like to use the next open slot? (9c) (y/N):
  Would you like to overwrite this slot? (y/N):
  ```
 - if there is a non-tsh-generated certificate, prompt the user to overwrite. If they say no, then prompt them to use the next open slot.
  ```
  > tsh login --piv
  PIV slot 9a may be in use by another program.
  Would you like to overwrite this slot? (y/N):
  Would you like to use the next open slot? (9c) (y/N):
  ```

We will also provide the user with the `--piv-slot-overwrite` flag to skip the prompts above.

Note that we do not give user's the option to reuse a specific slot for multiple `tsh` login sessions or other applications. Doing so would increase the overall complexity of the system, and could lead to issues like:
 - race conditions between reads/writes to a slot's login session certificate
 - using the same key for clusters/users with differing PIN/touch policy requirements. This would require more granular error messages, prompts, etc. and would lead to a more complicated UX than desired
 - a key's life cycle is not determinant, so its unclear how long to store a key's attestation

If necessary we could overcome these issues, but it's unclear whether this would even improve UX.

##### logout

When a user does `tsh logout` during a PIV login session, we cannot wipe the private key data on that slot. Instead, we will just remove the certificate stored on the slot so that future logins will see that the slot is not in use.

Note that if a user does not `tsh logout` and instead does something like `rm -r ~/.tsh`, then the slot will inaccurately report that it's still in use. The user will have to overwrite the slot in a future call to `tsh login --piv` to re-sync.

### Server Enforcement (Yubikey PIV)

In order to enforce PIV login, we need to take a public key and tie it back to a trusted hardware device. This can be down with [Attestation](https://docs.yubico.com/yesdk/users-manual/application-piv/attestation.html), which yubikey makes possible by reserving the `f9` slot for an attestation certificate. This certificate can then be used to [verify](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#Verify) the certificate chain from a yubikey's public key to a trusted yubikey CA.

Note: attestation is not included in the PIV standard, so the following strategy will only be applicable to PIV keys that include the [Yubico Assert Extension](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html). New PIV hardware keys may also implement this extension. For example, solokeys' unfinished implementation of PIV at least has [mentions of YubicoPIVExtension in the code](https://github.com/solokeys/piv-authenticator/blob/1922d6d97ba9ea4800572eea4b8a243ada2bf668/src/constants.rs#L274) which indicates that these extensions may be supported at some point.

After generating a Yubikey keypair, `tsh` will grab the Yubikey's attestation certificate and generate a new attestation statement [slot cert](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#YubiKey.Attest) for the slot in question. These two certificates will be passed to the Auth server during the certificate signing process in order to verify certificates signed with the slot's public key.

In addition to verifying the certificate chain, the attestation will return the private key's PIN and touch policies, which we can check to enforce specific PIN and touch policies across Teleport Clusters and Roles.

#### Attestation enforcement

Attestation will be enforced in a similar way to [Certificate Locking](https://github.com/gravitational/teleport/blob/master/rfd/0009-locking.md). Essentially, a user will not be able to successfully make requests unless the public key on their certificates has been attested, and the attestation properties (pin and touch policies) meet the criteria set by the server (auth preference and role options).

For this flow, we will need to:
 1. perform attestation after the user receives their certificates from a successful login
 2. store the attestation object in the backend for future checks
 3. check if a certificate has a valid attestation in the backend before authorizing any requests

In `tsh login --piv`, attestation will be performed immediately after the signing process with a new gRPC endpoint:

```
service AuthService {
  rpc AssertPIVSlot(AssertPIVSlotRequest) returns (AssertPIVSlotResponse);
}

message AssertPIVSlotRequest {
  YubikeyPIVAttestationData yubikey_attestation = 1;
}

message YubikeyPIVAttestationData {
  bytes SlotCert = 1;
  bytes AttestationCert = 2;
}
```

`AssertPIVSlot` will then check if the attestation passes the current enforcement criteria for the user. If it doesn't, an access denied error will be returned describing what criteria were not met. Otherwise, the attestation will be inserted into the backend in the table `/piv_assertions/<public_key>` to verify future requests for corresponding certificates.

#### Auth Service Config or Auth Preference

You can provide a `piv` field to to a cluster's Auth Preference, in the Auth Service's config file or with a dynamic Cluster Auth Preference object:

```yaml
auth_service:
  ...
  authentication:
    ...
    piv: 
      mode: required
      pin_policy: always
      touch_policy: always
```

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  piv: 
    mode: required
    pin_policy: always
    touch_policy: always
```

Allowed values for `piv.mode`:
 - `required`: certificates must have a valid attestation to be used
 - `optional` (default): certificates won't require a valid attestation to be used, but users can still attest their keys pre-emptively in case of future config changes
 - `disabled`: attestation will not be performed - essentially `AssertYubikeyPIVSlot` will be disabled by the auth server and always return no error


If `pin_policy` or `touch_policy` is set, then the Auth server will check the [`Attestation`](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#Attestation) object's `PINPolicy` or `TouchPolicy` fields. If the checked fields are at least as strict as the policies required, then the request will proceed.

Examples:
 - `pin_policy=once` + `PINPolicy=always` = success
 - `touch_policy=always` + `TouchPolicy=always` = success
 - `touch_policy=cached` + `TouchPolicy=never` = failure

#### Role Options

To make PIV login required or optional on a per-role basis, you can set the same options in a Role.

```yaml
kind: role
version: v5
metadata:
  name: role-name
spec:
  role_options:
    piv: 
      mode: required
      pin_policy: once
      touch_policy: cached
```

A user's role options will be combined with the cluster's Auth Preference to form the strictest combination.

Example:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  piv: 
    mode: optional
    pin_policy: once
    touch_policy: cached
---
kind: role
version: v5
metadata:
  name: piv-pin
spec:
  role_options:
    piv: 
      mode: required
      pin_policy: always
---
kind: role
version: v5
metadata:
  name: piv-touch
spec:
  role_options:
    piv:
      mode: required
      touch_policy: always
```
- Users with `piv-pin` role must use PIV login with `pin=always` and `touch=cached|always`
- Users with `piv-touch` role must use PIV login with `pin=once|always` and `touch=always`
- Users with both the `piv-pin` and `piv-touch` roles must use PIV login  with `pin=always` and `touch=always`
- Users with neither the `piv-pin` nor `piv-touch` roles are not required to use PIV login

#### Retrieve user's PIV attestation requirements during login

Since PIN and touch policies are configured locally on a PIV private key, we cannot delegate the task of setting policies for a private key to the server. Additionally, since these policies depend on Cluster Auth Preference and Role settings, there is no way for us to give the required settings to the user before they login.

To work around this limitiation, we will add another gRPC endpoint `GetPIVRequirements` for a user to fetch their PIV requirements:

```
rpc GetPIVRequirements(GetPIVRequirementsRequest) returns (GetPIVRequirementsResponse);

// The user will be pulled from the request certificate, so request message can be empty for now.
message GetPIVRequirementsRequest {
}

message GetPIVRequirementsResponse {
    PIVRequirements PIVRequirements = 1;
}

message PIVRequirements {
    string Mode = 1;
    string PINPolicy = 2;
    string TouchPolicy = 3;
}
```

This can be performed after the initial login/signing flow, and before attestation. If the user did not use `--piv` or did not provide strict enough `--piv-pin-policy` or `-piv-touch-policy`, then the user will see a warning message, and `tsh login` will automatically try to generate a valid PIV key and re-perform the login/singing flow with the new key.

### PIV secret management - Management Key, PUX, and PIN

Some PIV operations require [adminstrative access](https://developers.yubico.com/PIV/Introduction/Admin_access.html), which require one or more of the following secrets:

| Name           | size     | default value                                      | function                                  |
|----------------|----------|----------------------------------------------------|-------------------------------------------|
| Management Key | 24 bytes | `010203040506070801020304050607080102030405060708` | private key and certificate management    |
| PIN            | 8 chars  | `123456`                                           | sign and decrypt data, reset pin          |
| PUK            | 8 chars  | `12345678`                                         | reset PIN when blocked by failed attempts |

The Management Key must be provided during login, and the PIN must be provided during login and during handshakes. Using the default secrets is not recommended, so we will prompt the user for these values when they are needed.

```
> tsh login --piv
Enter the PIV card's Management Key [blank to retrieve it from PIN protected metadata]:
Enter the PIV card's PIN:
```

#### tsh piv configure

We will provide a new command - `tsh piv configure` - so that users can configure their PIV management secrets before they login. This command will prompt the user for the current PIN, PUK, and Management Key, with defaults mentioned. The PIN and PUK will be set from user input, while the Management Key will be randomly generated and stored in [PIN protected metadata](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#YubiKey.Metadata). This way, the user will only need to provide the PIN during login. If the user does not have a management key set in metadata, because they didn't run `tsh piv configure` previously, then they will still be prompted as shown above.

Storing the Management Key in metadata is the [intended purpose](https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#YubiKey.Metadata) set by `piv-go`, because it can greatly improve UX. However, it essentially conflates the PIN and Management Key into a single key, which lowers the general security profile of the card.

In our case, we are ok with making this tradeoff because the Management Key does not actually provide any significant security benefit. It is only needed to grant access to [generating/importing key pairs and importing certificates](https://developers.yubico.com/PIV/Introduction/Admin_access.html), which will be strictly managed by `tsh`. If a user attempts to generate/import a new key, it would only break the user's existing `tsh` login session, because the user's signed Teleport certificates would not be signed with the new key. Importing a new certificate would just make it so that `tsh` cannot find the certificate holding login metadata.

Note: Using metadata to store the Management Key is possible because of the [Yubico PIV Extension](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html), so this strategy may not work for all PIV cards.

```
> tsh piv configure

// Prompt user to set PUK
Enter the current PUK (default 12345678):
Enter the new PUK:
Repeat for confirmation:
New PUK set.

// Prompt user to set PIN
Enter the current PIN (default 123456):
Enter the new PIN:
Repeat for confirmation:
New PIN set.

// Prompt user to set management key
Please enter the current Management key [blank to use default key]:
New Management Key generated: <random-24-byte-key>

Would you like to store this key in PIN protected PIV metadata, so that
tsh can find it automatically? This is not recommended if you plan to use
the Management Key with other PIV applications, like `yubikey-agent`. (y/N):

The Management key has been added to PIN protected metadata. You can 
retrieve this key with `tsh piv configure --get-mgm-key` for use with
other PIV clients, like `ykman` or `yubico-piv-tool`.

> tsh login --piv
Enter the PIV card's PIN:
// Management key retrieved from metadata, so we don't prompt for it
```

This command will also be sharded into each of the three individual actions with `--set-pin`, `--set-puk`, and `--set-mgm-key`.

If the user's PIN is blocked by PUK, then `tsh piv configure --set-pin` will notify the user and prompt them for PUK to reset their PIN instead.

```
> tsh piv configure --set-pin
PIN is blocked, using PUK to reset PIN.
Warning: three failures to enter the correct PUK will lock the PUK
and you will be required to `tsh piv configure --reset` to reset your
PIV card to defaults and erase all private key data.

// Prompt user for current PUK and new PIN
Enter the current PUK:
Enter a new PIN:
Repeat for confirmation:
New PIN set.
```

We will also provide the user with the flags `--get-mgm-key` and `--reset`, which are noted in the command outputs above.

```
> tsh piv configure --get-mgm-key

// Retrieve management key from metadata
Management Key: <24 byte random code>
WARNING: This is not recommended if you are using your PIV card with any other applications. Continue? (y/N):

// Or if management key key not found in metadata...
Management Key not found, try configuring it with `tsh piv configure --set-mgm-key`
```

```
> tsh piv configure --reset
// Reset PIV to default administrative keys and wipe private key and certificate data, equivalent to `ykman reset`
```

### UX

#### new `tsh` flags

| Name | Default Value(s) | Allowed Value(s) | Description |
|-|-|-|-|
| `--piv` | - | - | During login, use PIV to generate and store private key data for your login session. |
| `--piv-slot` | 9a | 9a, 9c-9e, 82-95 | Configure which PIV slot is used is used for this login session. |
| `--piv-yubikey-serial` | - | - | Specify which yubikey to use for PIV login by serial number. This is only needed when more than one smart card is connected. |
| `--piv-pin-policy` | once | always once never | Can be provided with `tsh login -piv` to control how often you need to enter your PIV PIN to access the PIV slot's private key data. |
| `--piv-touch-policy` | cached | always cached never | Can be provided with `tsh login -piv` to control how often you need to touch your PIV card to access the PIV slot's private key data. |
| `--piv-overwrite` | - | - | When provided, `tsh login -piv` will overwrite a PIV slot's existing data if it exists. |

### Additional considerations

#### Kubernetes and MongoDB support

Both Kubernetes and MongoDB login in `tsh` depend on raw RSA Private key data:
 - https://github.com/gravitational/teleport/blob/master/lib/kube/kubeconfig/kubeconfig.go#L164-L167
 - https://github.com/gravitational/teleport/blob/fc05eaf305f32b943a0e18b3a84c9f61a544b1ae/lib/client/client.go#L351-L353

Since we can't get the raw RSA private key data from a PIV card, we don't have an easy way to make these integrations work.

For Kubernetes, it may be possible to create a custom auth provider plugin and supply it to the kubernetes Auth Info - https://pkg.go.dev/k8s.io/client-go@v0.24.3/tools/clientcmd/api#AuthProviderConfig.

#### PIV agent key support

Initially, PIV login will not support `tsh --add-keys-to-agent`, `tsh -A`, or Proxy Recording mode, because [Adding agent keys from a smartcard](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). We can implement this support ourselves in the future.

For Yubikey, users can [manually add their keys](https://github.com/jamesog/yubikey-ssh) to their ssh-agent with `ssh-add` after performing `tsh login --piv`. However, this will not add their SSH certificate to the ssh-agent, so some additional workaround will be needed.

Alternatively, we could workaround this by instead adding ssh-agent integration that would work with something like `yubikey-agent`. Then, instead of `tsh login --piv`, users could use something like `tsh login --use-agent-key=<agent-key-name>`. 

Note: `yubikey-agent` [holds an open connection](https://github.com/FiloSottile/yubikey-agent#conflicts-with-gpg-agent-and-yubikey-manager) to yubikey, preventing `tsh` from connecting with PIV to do things like attestation with the existing private key. Instead, we'd likely need to create a custom `tsh yubikey agent` which does not hold an open connection to the yubikey, or potentially use a different agent like `pivy-agent`.

#### Universal Teleport Client support

At first, PIV login support will only be added to `tsh`, but in the future other clients like `tctl`, the WebUI, and Teleport Connect can support it with the same general flow. 

When PIV login is enforced on the Auth Server, unsupported Teleport clients will fail to login unless we add PIV login support for them as well. This would be especially disruptive for the WebUI. Until all Teleport clients are supported, PIV login will remain in preview mode, and we will communicate with preview users that required PIV attestation should be enabled with caution.

#### Storing Teleport certificates in the PIV slot

Storing Teleport-signed certificates on a PIV slot is possible, but we can only store one certificate per slot. Since in most cases we need both an ssh and tls certificate, this functionality would not provide us with much benefit, and it's much simpler to store them in the usual locations on disk. This would also prevent us from using the certificate slot for login metadata, which we use to track PIV slots used by `tsh`.