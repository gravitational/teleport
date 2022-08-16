---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 80 - Hardware Private Keys

## Required approvers

Engineering: @jakule && @r0mant && @codingllama
Product: @klizhentas && @xinding33
Security: @reedloden

## What

Integrate and enforce the use of hardware keys for client-side cryptographical operations.

## Why

Hardware keys can be used to generate and store private keys which cannot be exported for external use. This feature provides desired security benefits for protecting a user's login session. See the [security](#security) section for more details.

## Details

### Hardware key overview

Cryptographical hardware keys, including HSMs, TPMs, and PIV-compatible smart cards (yubikey), can be used to generate, store, and retrieve private keys and certificates. The most widely supported interface for handling key/certificate access is [PKCS#11](http://docs.oasis-open.org/pkcs11/pkcs11-base/v2.40/errata01/os/pkcs11-base-v2.40-errata01-os-complete.html#_Toc441755758).

PKCS#11 provides the ability to:
 - generate and store private keys directly on a hardware key
 - perform cryptographic operations with a hardware key's stored private keys
 - store and retrieve certificates directly on a hardware key

However, the PKCS#11 interface is complex, hard, to use, and does not provide a standard for slot management or attestation. Since we currently only plan to support yubikeys, which are PIV-compatible, we will use PIV for its ease of use and additional capabilities.

#### PIV

Personal Identity Verification (PIV), described in [FIPS-201](https://csrc.nist.gov/publications/detail/fips/201/3/final) and defined by [NIST SP 800-73](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf), is an open standard for smart card access. 

PIV builds upon the PKCS#11 interface and provides us with additional capabilities including:
 - PIN and Touch requirements for accessing keys (if requested during key generation)
 - PIV secrets for granular [adminstrative access](https://developers.yubico.com/PIV/Introduction/Admin_access.html)
 - [Attestation](https://docs.yubico.com/yesdk/users-manual/application-piv/attestation.html) of private key slots
 
##### Attestation

Attestation makes it possible for us to take a hardware private keys and verify that it was generated on a trusted hardware key. This verification will be useful for enforcing hardware private key usage.

Note that attestation is not expressly included in the PIV standard. However, PIV was designed around the idea of a central authority creating trusted PIV smart cards, so all PIV implementations should provide some way to perform attestation. In fact, hardware keys in general should all support attestation in some capacity.

For example, Yubico created their own [PIV attestation extension](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html). Other hardware keys may implement the same extension, such as [solokeys](https://github.com/solokeys/piv-authenticator/blob/1922d6d97ba9ea4800572eea4b8a243ada2bf668/src/constants.rs#L27) which has indications that it will, or they may provide alternative methods for attestation.

##### Library

We will use the [go-piv](https://github.com/go-piv/piv-go) library, which is a Golang port of Yubikey's C library [ykpiv](https://github.com/Yubico/yubico-piv-tool/blob/master/lib/ykpiv.c). This is the same library used by [yubikey-agent](https://github.com/FiloSottile/yubikey-agent). 

Currently, Yubikey is one of the only PIV-compatible commercial hardware keys. As a result, current PIV implementations like piv-go are specifically designed around Yubikey's implementation of PIV - the `libykcs11.so` module. While the majority of PIV is standardized, the Yubikey PIV implementation has [some extensions](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html) and other idiosyncracies which may not be standard accross future PIV implemenations.

Unfortunately, there is no common PIV library, so our best option is to use `piv-go` for a streamlined implemenation and prepare to adjust in the future as more PIV-compatible hardware keys are released. Possible adjustments include:
 - using multiple PIV libraries to support custom PIV implementations
 - switching to a PIV library which expressly supports all/more PIV implemenations
 - working within a PIV library, through PRs or a Fork, to expand PIV support
 - creating our own custom PIV library which we can add custom support into as needed

Note: the adjustments above will largely be client-side and therefore should not pose any backwards compatibility concerns.

### Security

Currently, Teleport clients generate new RSA private keys to be signed by the Teleport Auth server during login. These keys are then stored on disk alongside the certificates, where they can be accessed and used to perform actions as the logged in user. These actions include any Teleport Auth server request, such as listing clusters (`tsh ls`), starting an ssh session (`tsh ssh`), or adding/changing cluster resources (`tctl create`). In order for an attacker to exflitrate a user's login session and perform such actions, they would just need to export the user's certificate and private key files, which are all stored on the user's disk (in `~/.tsh`).

With hardware private keys, even if an attacker exports a user's certificates from disk, they will not provide access unless the attacker also has access to the user's hardware key. Therefore, simple exfiltration attacks on a user's `~/.tsh` directory would not work.

However, an attacker could still potentially gain access if they hack into the user's computer while the hardware key is connected. To mitigate this risk, we have two options:
 1. Enable [per-session MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa/) alongside hardware private key enforcement, which requires you to pass an MFA check (touch) to start a new Teleport Service session (SSH/Kube/etc.). 
 2. Enforce Touch to access hardware private keys, which can be done with PIV-compatible hardware keys. In this case, Touch is required for every Teleport request, not just new Teleport Service sessions.

The first option is a bit simpler as it rides off the coattails of our existing Per-session MFA system. On the other hand, the second option provides better security principles, since touch is enforced for every Teleport request rather than just Session requests, and it requires fewer roundtrips to the Auth server.

In this RFD we'll explore both options together, since they are not mutually exclusive, and the underlying implementation will be the same regardless. 

Note: If either of these options are combined with MFA/PIV PIN enforcement, or biometric key usage (like the [Yubikey Bio Series](https://www.yubico.com/products/yubikey-bio-series/)), then even if a user's computre and hardware key are stolen, the user's login session would not provide access to an attacker. To avoid overcomplicating this RFD, we will omit this consideration and leave it as a possible future improvement.

### Server changes

Hardware private key enformcement will be configured and controlled by the Teleport Auth Server, so let's start with the server changes.

#### Configuration

Teleport admins can require hardware private key storage in the cluster's Auth Preference, which can be defined in the Auth Service's config file or with a dynamic Cluster Auth Preference object:

```yaml
auth_service:
  ...
  authentication:
    ...
    user_private_keys: disk | hardware_key | hardware_key_touch
```

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  user_private_keys: disk | hardware_key | hardware_key_touch
```

This can also be configured for individual roles:

```yaml
kind: role
version: v5
metadata:
  name: role-name
spec:
  role_options:
    user_private_keys: disk | hardware_key | hardware_key_touch
```

- `disk` (default): A user's private keys can be generated in memory and stored on disk. No enforcement necessary.
- `hardware_key`: A user's private keys must be generated on a hardware key. As a result, the user cannot use their signed certificates unless they have their hardware key connected.
- `hardware_key_touch`: A user's private keys must be generated on a hardware key, and must require touch to be accessed. As a result, the user must touch their hardware key on login, and on every subsequent request.

Alternatively, Teleport admins can "upgrade" their existing [require-session-mfa configuration](https://goteleport.com/docs/access-controls/guides/per-session-mfa/#configure-per-session-mfa) from `true` to `hardware_key`. This will essentially act as an alias for `user_private_keys: hardware_key && require_session_mfa: true`.

#### Enforcement - Attestation

In order to enforce the user private key usage, we need to take a certificate's public key and tie it back to a trusted hardware device, which can be done with attesatation, as explained [above](#attestation).

First, we need to get attestation data from the user's hardware key during login, which we will do with the new rpc `AttestHardwarePrivateKey`.

```proto
service AuthService {
  rpc AttestHardwarePrivateKey(AttestHardwarePrivateKeyRequest) returns (AttestHardwarePrivateKeyResponse);
}

message AttestHardwarePrivateKeyRequest {
  YubikeyPIVAttestationData yubikey_attestation = 1;
  // We may add non-yubikey and non-piv options in the future
}

// Data used to attest a slot - https://pkg.go.dev/github.com/go-piv/piv-go@v1.10.0/piv#Verify
message YubikeyPIVAttestationData {
  bytes slot_cert = 1;
  bytes attestation_cert = 2;
}

message AttestHardwarePrivateKeyResponse { }
```

In addition to verifying the certificate chain for a user's hardware private key to a trusted hardware key manufacturer, the resulting attestation object will provide information about the private key, including;
 - Device information, including serial number, model, version 
 - Configured Touch (And PIN) Policies if any

This attestation data will be check against server settings. If the attestation is valid, it will be stored in the backend in `/web/users/<user>/hardware_keys/<public_key>`.

```go
type AttestationData struct {
  // json blob containing the resulting attestation object.
  // For yubikey, this will look like https://pkg.go.dev/github.com/go-piv/piv-go@v1.9.0/piv#Attestation
  AttestationObject []byte `json:"attestation"`
}
```

The Auth Server will check for a valid attestation for a user's certificate before authorizing any requests, in a similar way to [Certificate Locking](https://github.com/gravitational/teleport/blob/master/rfd/0009-locking.md).

### Client changes

Teleport clients will need the ability to connect to a user's hardware key, generate/retrieve private keys, and use those keys for cryptographical operations. 

#### `user_private_key` configuration discovery

Teleport clients should be able to automatically determine if a user requires a hardware private key for login to avoid additional UX concerns. For this, we will introduce the `GetPrivateKeyRequirement` rpc.

```proto
service AuthService {
  rpc GetPrivateKeyRequirement(GetPrivateKeyRequirementRequest) returns (GetPrivateKeyRequirementResponse);
}

// The user will be pulled from the request certificate, so request message can be empty for now.
message GetPrivateKeyRequirementRequest {}

message GetPrivateKeyRequirementResponse {
    PrivateKeyRequirements private_key_requirements = 1;
}

message PrivateKeyRequirements {
    PrivateKeyMode mode = 1;
}

enum PrivateKeyMode {
    PRIVATE_KEY_MODE_DISK = 0;
    PRIVATE_KEY_MODE_HARDWARE_KEY = 1;
    PRIVATE_KEY_MODE_HARDWARE_KEY_TOUCH = 2;
}
```

Unfortunately, In order to discover a user's private key requirements, we need to first provide the user with a set of valid certificates. Unless we want to force the user to login twice, we will need to find a compromise.

First, we can have Teleport Clients follow the current login flow with a random RSA private key to get valid certificates. If hardware private key usage is enforced, then the server will only sign the certificates with a TTL of 1 minute. These certificates will only be usable by the endpoints `GetPrivateKeyRequirement` and `GenerateUserCerts` to reissue certificates. `GenerateUserCerts` will then enforce that the re-issue certificates request is using a hardware private key.

#### Hardware private key login

As mentioned above, hardware key login will start with the normal login flow, followed by a call to `GetPrivateKeyRequirement`. If the result is `PRIVATE_KEY_MODE_HARDWARE_KEY` or `PRIVATE_KEY_MODE_HARDWARE_KEY_TOUCH`, then the Teleport client will:
 1. Find a hardware key on the user's device
 2. Generate a new private key on the hardware key (with Touch policy "cached" if required)
 3. Use the hardware private key to get a new set of signed certificates from the Teleport Auth Server
 4. Upsert Attestation data with `AttestHardwarePrivateKey` so that future Teleport requests can verify that the certificates being used chain back to a trusted hardware key CA 

The resulting certificates from will only be operable if:
 - The hardware key is connected during the operation
 - The hardware private key can still be found
 - The hardware private keys' Touch challenges are passed (if applicable)

#### PIV slot overwrite/reuse logic

For PIV hardware keys, we will use slot `9a`, which is reserved for authentication. Before generating a new private key, Teleport clients will check if PIV slot `9a` is already in use by a Teleport client. If it is, then the client will reuse the existing private key instead of generating a new one. Otherwise, it will generate a new private key.

#### Private key interface

Currently, Teleport clients store a PEM encoded private key (`~/.tsh/keys/proxy/user`) for a login session. This PEM encoded private key is then unmarshalled, transformed, and parsed as needed during a client request.

With a hardware private key, we only have access to a raw `crypto.PrivateKey`, and do not have sufficient information about the key to transform it into a `*rsa.PrivateKey` and marshal it into PKCS1 format. Instead, we need to alter Teleport clients to use `cyrpto.PrivateKey` by default. This will require altering the key interface (`lib/client/interfaces.go`) and its usage across `lib/client` and other relevant locations. `lib/utils/native` will also be updated to return `*rsa.PrivateKey` instead of its PEM encoded private and public keys.

We also need a way for future Teleport Client requests to retrieve the correct `crypto.PrivateKey`. For RSA keys, we can continue to store them as PEM encoded keys in (`~/.tsh/keys/proxy/user`). For hardware private keys, we will instead store a fake PEM encoded private key which we can use to identity what device and slot to load the private key from.

```
-----BEGIN YUBIKEY PRIVATE KEY-----
# PEM encoded `yubikey_serial_number+slot`
-----END YUBIKEY PRIVATE KEY-----
```

#### Supported clients

`tsh` and Teleport Connect will both support hardware private key login, and `tctl` will be able to use resulting login sessions. 

Supporting hardware private key login through the WebUI needs to be investigated more thoroughly, but it poses a challenge since it is browser-based and does not have direct access to the user's local hardware keys.

### UX

The most notable UX change is that a user's login session will not be usable unless their hardware key is connected.

If `user_private_keys: hardware_key_touch` is used, then every Teleport Client request will require touch (cached for 15 seconds). This will be handled by a touch prompt similar to the one used for MFA.

### Additional considerations

#### Database support

`tsh db connect` uses raw RSA private key data to form connections. Since this cannot be supported with hardware private keys, users will instead need to use `tsh proxy db` to connect using a local proxy. Teleport Connect already uses `tsh proxy db` and will not be affected, but the WebUI may have an additional challeng to support datbase connections.

#### Kubernetes support

Kubernetes integration uses raw RSA private [key data to form connections](https://github.com/gravitational/teleport/blob/master/lib/kube/kubeconfig/kubeconfig.go#L164-L167). It may be possible to create a [custom auth provider plugin](https://pkg.go.dev/k8s.io/client-go@v0.24.3/tools/clientcmd/api#AuthProviderConfig) and supply it to the kubernetes Auth Info. Kubernetes support will be investigated and fixed in a follow up PR after the intial hardware private key implementation.

#### Agent key support

Initially, hardware private key login will not support `tsh --add-keys-to-agent`, `tsh -A`, or Proxy Recording mode, because [Adding agent keys from a hardware key](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). We can implement this support ourselves in the future.

For Yubikey, users can also [manually add their keys](https://github.com/jamesog/yubikey-ssh) to their ssh-agent with `ssh-add` after logging in. However, this will not add their SSH certificate to the ssh-agent, so some additional workaround will be needed.

#### PIV secret management

Some PIV operations require [adminstrative access](https://developers.yubico.com/PIV/Introduction/Admin_access.html), which require one or more of the following secrets:

| Name           | size     | default value                                      | function                                  |
|----------------|----------|----------------------------------------------------|-------------------------------------------|
| Management Key | 24 bytes | `010203040506070801020304050607080102030405060708` | private key and certificate management    |
| PIN            | 8 chars  | `123456`                                           | sign and decrypt data, reset pin          |
| PUK            | 8 chars  | `12345678`                                         | reset PIN when blocked by failed attempts |

To simplify our implementation and limit UX impact, we will expect the user's PIV device to use the default Management Key. In the future, we may want to add support for using non-default management key to better protect the generation and retrieval of private keys on the user's PIV key, as well as PIN management if we decide to add an options like `user_private_keys: hardware_key_touch_pin`.