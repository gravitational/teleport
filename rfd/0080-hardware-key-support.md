---
authors: Brian Joerger (bjoerger@goteleport.com)
state: implemented
---

# RFD 80 - Hardware Key Support

## Required approvers

* Engineering: @jakule && @r0mant && @codingllama
* Product: @klizhentas && @xinding33
* Security: @reedloden

## What

Integrate and enforce the use of hardware keys for client-side cryptographical operations.

## Why

Hardware keys can be used to generate and store private keys which cannot be exported for external use. This feature provides desired security benefits for protecting a user's login session. See the [security](#security) section for more details.

## Details

### Hardware key overview

Cryptographical hardware keys, including HSMs, TPMs, and PIV-compatible smart cards (yubikey), can be used to generate, store, and retrieve private keys and certificates. The most widely supported interface for handling key/certificate access is [PKCS#11](http://docs.oasis-open.org/pkcs11/pkcs11-base/v2.40/errata01/os/pkcs11-base-v2.40-errata01-os-complete.html#_Toc441755758).

PKCS#11 provides the ability to:

* generate and store private keys directly on a hardware key
* perform cryptographic operations with a hardware key's stored private keys
* store and retrieve certificates directly on a hardware key

However, the PKCS#11 interface is complex, hard to use, and does not provide a standard for slot management or attestation. Since we currently only plan to support yubikeys, which are PIV-compatible, we will use PIV for its ease of use and additional capabilities.

#### PIV

Personal Identity Verification (PIV), described in [FIPS-201](https://csrc.nist.gov/publications/detail/fips/201/3/final) and defined by [NIST SP 800-73](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf), is an open standard for smart card access.

PIV builds upon the PKCS#11 interface and provides us with additional capabilities including:

* Optional pin and touch requirements for accessing keys
* PIV secrets for granular [administrative access](https://developers.yubico.com/PIV/Introduction/Admin_access.html)
* [Attestation](https://docs.yubico.com/yesdk/users-manual/application-piv/attestation.html) of private key slots

##### Attestation

Attestation makes it possible for us to take a hardware private key and verify that it was generated on a trusted hardware key. This verification will be useful for enforcing hardware private key usage.

Attestation is not expressly included in the PIV standard. However, PIV was designed around the idea of a central authority creating trusted PIV smart cards, so all PIV implementations should provide some way to perform attestation. In fact, even non-PIV hardware keys can be expected to support attestation in some form.

For example, Yubico created their own [PIV attestation extension](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html). Other hardware keys may implement the same extension, such as [solokeys](https://github.com/solokeys/piv-authenticator/blob/1922d6d97ba9ea4800572eea4b8a243ada2bf668/src/constants.rs#L27) which has indications that it will, or they may provide alternative methods for attestation.

##### Library

We will use the [go-piv](https://github.com/go-piv/piv-go) library, which is a Golang port of Yubikey's C library [ykpiv](https://github.com/Yubico/yubico-piv-tool/blob/master/lib/ykpiv.c). This is the same library used by [yubikey-agent](https://github.com/FiloSottile/yubikey-agent).

Currently, Yubikey is one of the only PIV-compatible commercial hardware keys. As a result, current PIV implementations like piv-go are specifically designed around Yubikey's implementation of PIV - the `libykcs11.so` module. While the majority of PIV is standardized, the Yubikey PIV implementation has [some extensions](https://developers.yubico.com/PIV/Introduction/Yubico_extensions.html) and other idiosyncracies which may not be standard across future PIV implementations.

There is no common PIV library, so our best option is to use `piv-go` for a streamlined implementation and prepare to adjust in the future as more PIV-compatible hardware keys are released. Possible adjustments include:

* using multiple PIV libraries to support custom PIV implementations
* switching to a PIV library which expressly supports all/more PIV implementations
* working within a PIV library, through PRs or a Fork, to expand PIV support
* creating our own custom PIV library which we can add custom support into as needed

Note: the adjustments above will largely be client-side and therefore should not pose any backwards compatibility concerns.

### Security

#### Touch enforcement

Currently, Teleport clients generate new RSA private keys to be signed by the Teleport Auth server during login. These keys are then stored on disk alongside the certificates (in `~/.tsh`), where they can be accessed and used to perform actions as the logged in user. These actions include any Teleport Auth server request, such as listing clusters (`tsh ls`), starting an ssh session (`tsh ssh`), or adding/changing cluster resources (`tctl create`). If an attacker manages to exfiltrate a user's `~/.tsh` folder, they could use the contained certificates and key to perform actions as the user.

With the introduction of a hardware private key, the user's key would not be stored on disk in `~/.tsh`. Instead, it would be generated and stored directly on the hardware key, where it can not be exported. Therefore, if an attacker exfiltrates a user's `~/.tsh` folder, the contained certificates would be useless without also having access to the user's hardware key.

So far, just introducing hardware private keys into the login process prevents simple exfiltration attacks. However, an attacker could still potentially steal a user's login session if they hack into the user's computer while the hardware key is connected. To mitigate this risk, we also need to enforce a presence check.

For this, we have two options:

 1. Enable [per-session MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa/), which requires you to pass an MFA check (touch) to start a new Teleport Service session (SSH/Kube/etc.)
 2. Require touch to access hardware private keys, which can be done with PIV-compatible hardware keys. In this case, touch is required for every Teleport request, not just new Teleport Service sessions

The first option is a bit simpler as it rides off the coattails of our existing per-session MFA system. On the other hand, the second option provides better security principles, since touch is enforced for every Teleport request rather than just Session requests, and it requires fewer roundtrips to the Auth server.

In this RFD we'll explore both options together, since they are not mutually exclusive, and may provide unique value.

#### PIN enforcement

Hardware key private keys can also be configured to require pin to be used in cryptographical operations. When combined with touch, requiring pin provides a level of authentication security similar to passwordless, as both user presence and a user secret are verified.

#### Web Sessions

Unlike WebAuthn, PIV does not have any native browser support. This means that the WebUI is incompatible with Hardware Key support. We could create custom browser extensions for some of the most commonly used browsers, but this induces too large of a development and maintenance cost to justify currently.

Instead, web sessions will be treated as an exception from Hardware Key support. This exception will only apply to web sessions created through the auth http endpoint `POST /:version/users/:user/web/authenticate`. This is the endpoint used by the WebUI login flow. Web Session created through this endpoint can only be accessed by the Auth and Proxy services. This will result in similar security properties to hardware private keys since the user, or an attacker, has no way to extract web session secrets without direct access to the Proxy/Auth services or Auth storage.

Web sessions created by user-authorized endpoints like the auth http endpoint `POST /:version/users/:user/web/sessions` will still be subject to hardware key restrictions to prevent abuse.

##### Web Session Access

Currently, the auth grpc endpoint `GetWebSession` can be used by a user to retrieve a specific web session, including secrets. This endpoint will be restricted to require `read` permissions for `KindWebSession`, similar to `GetWebSessions`. Users will still be able to retrieve non-secret web session info with the auth http endpoint `GET /:version/users/:user/web/sessions/:sid`.

##### Web Session cookies

Although we can guarantee that web session private key material is safely stored, web session cookies are easy to obtain from a user's browser. Web session cookies can only be used with the HTTP web API (`/webapi`), which provides a subset of functionality provided by the main Auth API to web sessions. Essentially, any functionality available in the WebUI is available through the `/webapi`.

Since MFA will still be required for sessions and admin actions, this is an acceptable tradeoff.

### Server changes

#### Private Key Policy

First, let's introduce the idea of private key policies. A private key policy refers to a characteristic of a private key which the Auth Service will enforce before signing its public key.

We will start with the following private key policies:

* `none` (default): No enforcement on private key usage
* `hardware_key`: A user's private keys must be generated on a hardware key. As a result, the user cannot use their signed certificates unless they have their hardware key connected
* `hardware_key_touch`: A user's private keys must be generated on a hardware key, and must require touch to be accessed. As a result, the user must touch their hardware key on login, and on subsequent requests (touch is cached on the hardware key for 15 seconds)
* `hardware_key_pin`: A user's private keys must be generated on a hardware key, and must require pin to be accessed. As a result, the user must enter their PIV pin on login, and on subsequent requests.
  * Unlike touch, pin is not cached explicitly. However, the pin is cached for the duration of a single PIV transaction. PIV transactions take a few seconds to close and can be reclaimed by subsequent PIV connections during the closing period. In this case, when multiple `tsh` commands are run in quick succession, it is as if the pin is cached.
  * This policy is intended for rare circumstances where a touch policy can not be configured due to the use of external PIV tools. However, since pin alone does not verify user presence, this option opens the door for remote attacks. When possible, `hardware_key_touch_and_pin` should be used instead of this option.
* `hardware_key_touch_and_pin`: combination of `hardware_key_touch` and `hardware_key_pin`.
* `web_session`: private key stored as a web session in the Auth service storage. This key is only accessible by the Auth and Proxy services. Keys with this policy meet all other key policy requirements.

In the future, we could choose to enforce more things, such as requiring a specific key algorithm.

#### Private Key Policy Enforcement

In order to enforce private key policies, we need to take a certificate's public key and tie it back to a trusted hardware device, which can be done with attestation, as explained [above](#attestation).

Attestation will be handled during the normal login/certificate signing process by adding a new `AttestationStatement` field to login requests. For all login paths, we need to include the `AttestationStatement` field in the http request objects:

```go
// lib/client/weblogin.go
type SSOLoginConsoleReq struct {
  ...
  AttestationStatement AttestationStatement `json:"attestation_statement,omitempty"`
}

type CreateSSHCertReq struct {
  ...
  AttestationStatement AttestationStatement `json:"attestation_statement,omitempty"`
}

type AuthenticateSSHUserRequest struct {
  ...
  AttestationStatement AttestationStatement `json:"attestation_statement,omitempty"`
}
```

For SSO login, the `AttestationStatement` field also needs to be added to each SSO auth request type (`OIDCAuthRequest`, `SAMLAuthRequest`, `GithubAuthRequest`), so we will make `AttestationStatement` a proto type.

```proto
// AttestationStatement is an attestation statement for a hardware private key.
message AttestationStatement {
  oneof attestation_statement {
    // yubikey_attestation_statement is an attestation statement for a specific YubiKey PIV slot.
    YubiKeyAttestationStatement yubikey_attestation_statement = 1;
  }
}

// YubiKeyAttestationStatement is an attestation statement for a specific YubiKey PIV slot.
message YubiKeyAttestationStatement {
  // slot_cert is an attestation certificate generated from a YubiKey PIV
  // slot's public key and signed by the YubiKey's attestation certificate.
  bytes slot_cert = 1;

  // attestation_cert is the YubiKey's unique attestation certificate, signed by a Yubico CA.
  bytes attestation_cert = 2;
}
```

When the Auth Server receives a login request, it will check the attached attestation statement:

* The `slot_cert`'s public key matches the public key to be signed
* The `slot_cert` chains to the `attestation_cert`
* The `attestation_cert` chains to a trusted hardware key CA (Yubico)

After the attestation statement has been verified, we can pull additional properties from the `slot_cert`'s extensions, which includes data like:

* Device information including serial number, model, and version
* Configured touch and pin Policies

This data will then be checked against the user's private key policy requirement. If the policy requirement is met, the Auth server will sign the user's certificates with a private key policy extension matching the attestation.

```go
// tls extension
PrivateKeyPolicyASN1ExtensionOID = asn1.ObjectIdentifier{1, 3, 9999, 1, 15}

// ssh extension
CertExtensionPrivateKeyPolicy = "private-key-policy"
```

The `AttestationData` will also be stored in the backend under `/key_attestations/<sha256>` so that reissue requests can pass the attestation check without re-providing the attestation statement. Key attestations will expire at the same time as the initial login certificates. Currently, we are only interested in verifying the certificate chain for the public key, and checking its private key policy, so the stored attestation data will look like:

```go
// AttestationData is verified attestation data for a public key.
type AttestationData struct {
  // PublicKeyDER is the public key in PKIX, ASN.1 DER form.
  PublicKeyDER []byte `json:"public_key"`
  // PrivateKeyPolicy specifies the private key policy supported by the associated private key.
  PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
}
```

#### Certificate key policy extension enforcement

On every Teleport request that enforces valid certificates, we will check that the required private key policy extension is included. This check will be handled by Teleport's shared authorizer, in a similar way to [user locking](https://github.com/gravitational/teleport/blob/master/rfd/0009-locking.md) enforcement.

#### Per-session MFA configuration

Hardware key enforcement configuration has been rolled in with per-session MFA, since both settings fulfill the same purpose.

This change will also require changing the `require_session_mfa` fields above from a `bool` to a `string`. This will be handled by introducing a new proto field and custom marshalling logic to maintain interoperability between new and old servers and clients. See [OIDC multiple redirect URLs](https://github.com/gravitational/teleport/pull/12054) for an example of this.

```yaml
auth_service:
  ...
  authentication:
    ...
    require_session_mfa: off | on | hardware_key | hardware_key_touch | hardware_key_pin | hardware_key_touch_and_pin
```

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  require_session_mfa: off | on | hardware_key | hardware_key_touch | hardware_key_pin | hardware_key_touch_and_pin
```

```yaml
kind: role
version: v5
metadata:
  name: role-name
spec:
  role_options:
    require_session_mfa: off | on | hardware_key | hardware_key_touch | hardware_key_pin | hardware_key_touch_and_pin
```

* `on`: Enforce per-session MFA. Users are required to pass an MFA challenge with a registered MFA device in order to start new SSH|Kubernetes|DB|Desktop sessions. Non-session requests, and app-session requests are not impacted.
* `hardware_key`: Enforce per-session MFA and private key policy `hardware_key`.
* `hardware_key_touch`: Enforce private key policy `hardware_key_touch`. This replaces per-session MFA with per-request PIV touch.
* `hardware_key_pin`: Enforce private key policy `hardware_key_pin`. This replaces per-session MFA with per-request PIV pin.
* `hardware_key_touch_and_pin`: Enforce private key policy `hardware_key_touch_and_pin`. This replaces per-session MFA with per-request PIV touch and pin.

##### PIV PIN counts as MFA

As [mentioned before](#private-key-policy), `hardware_key_pin` does not verify presence. In order to support this use cases, we have 2 options:

1. Require normal MFA in addition to PIV pin when MFA verification is required (per-session MFA, admin actions MFA). This would be functionally similar to the `hardware_key` option, where MFA touch is only required for sessions.
2. Treat `hardware_key_pin` as MFA verified, skipping the presence. This would be functionally the same as `hardware_key_touch`, but only PIN would be prompted instead of touch.

For a simpler UX, we will go with option 2. If in the future we decide to switch approaches, we can deprecate `hardware_key_pin` and replace it with `hardware_key_pin_and_mfa` to implement option 1.

##### Webauthn

Per-session MFA requires that WebAuthn is configured for the cluster, so a valid configuration would look like:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: on
    webauthn:
      rp_id: example.com
    require_session_mfa: on | hardware_key
```

However, touch/pin policies use PIV instead of MFA, so it can be configured without WebAuthn:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: off
    require_session_mfa: hardware_key_touch | hardware_key_pin | hardware_key_touch_and_pin
```

##### Per-resource enforcement

When `require_session_mfa` is configured on specific roles rather than the cluster auth preference, the per-session MFA check is only applied to resources (services) accessed via that role. For example, a user with the following roles would be prompted for MFA when connecting to nodes with `env: prod`, but not nodes with `env: staging`.

```yaml
kind: role
version: v5
metadata:
  name: staging
spec:
  options:
    require_session_mfa: false
  allow:
    node_labels:
      'env': 'staging'
  deny:
    ...
---
kind: role
version: v5
metadata:
  name: production
spec:
  options:
    require_session_mfa: true
  allow:
    node_labels:
      'env': 'prod'
  deny:
    ...
```

However, the same resource-based approach does not apply to hardware key policy requirement. Since the initial login credentials are used for all requests, regardless of resource, the user's login session must start with the strictest private key policy requirement.

### Client changes

Teleport clients will need the ability to connect to a user's hardware key, generate/retrieve private keys, and use those keys for cryptographical operations.

#### Private key policy discovery

Teleport clients should be able to automatically determine if a user requires a hardware private key for login to avoid additional UX concerns. Since it is not possible to retrieve a user's actual private key policy requirement before login, Teleport clients will make a best effort attempt to guess the key policy requirement.

First, the client will ping the Teleport Auth server to get the cluster-wide private key policy if set. Second, the client will check for an existing key in the user's key store (`~/.tsh`), and check its associated private key policy. Between the two private key policies retrieved, the stricter one will be used for initial login. This guessing logic will capture all cases except for the case where a user's role private key policy is stricter than the cluster-wide policy, and do not have an active/expired login session stored in `~/.tsh`.

If the private key policy was incorrect and a stricter requirement is needed, then the server will respond with a `private key policy not met: <private-key-policy>` error. The client will parse this error and resort to re-authenticating with the correct private key policy, meaning that the user will be re-prompted for their login credentials.

If a user's private key policy requirement is increased during an active login, the server will respond to any requests from the user with a `private key policy not met: <private-key-policy>` error. The Teleport client can capture this error and initiate re-login with the correct key policy.

#### Hardware private key login

On login, a Teleport client will find a private key that meets the private key policy provided (via the key policy guesser or server error). If the key policy is `none`, then a new RSA private key will be generated as usual.

If a hardware key policy is required, then a private key will be generated directly on the hardware key. The resulting login certificates will only be operable if:

* The hardware key is connected during the operation
* The hardware private key can still be found
* The hardware private key's touch challenge is passed (if applicable)
* The hardware private key's pin challenge is passed (if applicable)

#### PIV slot logic

PIV provides us with up to 24 different slots. Each slot has a different intended purpose, but functionally they are the same. We will use the first four slots (`9a`, `9c`, `9d`, `9e`) to support each of the 4 hardware key policy requirements (`hardware_key`, `hardware_key_touch`, `hardware_key_touch_and_pin`, `hardware_key_pin` respectively).

Each of these keys will be generated for the first time when a Teleport client is required to meet its respective private key policy. Once a key is generated, it will be reused by any other Teleport client required to meet the same private key policy.

Teleport clients will also store a self-signed metadata-containing certificate. When this certificate is present, Teleport clients will reuse or regenerate keys in the slot as needed. If the certificate in the slot is unknown or missing, Teleport clients will prompt the user for confirmation before overwriting an existing key or cert in the slot:

```bash
> tsh login
certificate in YubiKey PIV slot "9a" is not a Teleport client cert:
Slot 9a:
  Algorithm:  ECCP256
  Subject DN: CN=SSH key
  Issuer DN:  OU=(devel),O=yubikey-agent
  Serial:   20876611871300106558747702921785395021
  Fingerprint:  1ce4faf8bdbfc9668a9f532c20b03ccf1dbadcd06b51f235aeb3fe388bb1703b
  Not before: 2022-08-19 01:10:14
  Not after:  2064-08-19 01:10:14
Would you like to overwrite this slot's private key and certificate? (y/N):
```

##### Custom slot configuration

To support non-standard use cases, users can also provide a specific PIV slot to use via client or server settings:

* `tsh` flag/envvar: `--piv-slot`, `TELEPORT_PIV_SLOT`
* server settings: `auth_service.authentication.piv_slot`
* cluster auth preference settings: `cluster_auth_preference.spec.piv_slot`

This value can be set to the hexadecimal string representing the slot, such as `9d`. Any existing key in the slot will be used. If no key exists, the Teleport Client will attempt to generate a key in the slot. If the key does not meet the private key policy requirement for the user, the client will display an error to the user and prompt them to overwrite the slot.

If the key does not meet the private key policy requirement for the user, the user will be prompted to overwrite the slot:

```bash
> tsh --piv-slot=9a login
private key in YubiKey PIV slot "9a" does not meet private key policy "hardware_key_touch".
Would you like to overwrite this slot's private key and certificate? (y/N):
```

#### Private key interface

Currently, Teleport clients store a PEM encoded private key (`~/.tsh/keys/proxy/user`) for a login session. This PEM encoded private key is then unmarshalled, transformed, and parsed as needed during a client request.

With a hardware private key, we only have access to a raw `crypto.PrivateKey`, and do not have sufficient information about the key to transform it into an `*rsa.PrivateKey` and marshal it into PKCS1 format. Instead, we need to alter Teleport clients to use `crypto.PrivateKey` by default. This will require altering the key interface (`lib/client/interfaces.go`) and its usage across `lib/client` and other relevant locations. `lib/utils/native` will also be updated to return `*rsa.PrivateKey` instead of its PEM encoded private and public keys.

We also need a way for future Teleport Client requests to retrieve the correct `crypto.PrivateKey`. For RSA keys, we can continue to store them as PEM encoded keys in (`~/.tsh/keys/proxy/user`). For hardware private keys, we will instead store a fake PEM encoded private key which we can use to identity what device and slot to load the private key from.

```bash
-----BEGIN YUBIKEY PIV PRIVATE KEY-----
# base64 encoded
serial_number=<serial_number>
slot=<slot>
-----END YUBIKEY PIV PRIVATE KEY-----
```

#### Supported clients

`tsh` and Teleport Connect will both support hardware private key login, and `tctl` will be able to use resulting login sessions.

### UX

#### Hardware key login

When possible, hardware key login will not be any different from the normal login flow. However, in some cases, additional user intervention will be required. Below are some examples along with the resulting UX.

Note: Teleport Connect will need custom solutions for these edge cases, such as tsh-initiated callbacks.

##### Initial login fails due to an unmet private key policy

```bash
> tsh login --user=dev
Enter password for Teleport user dev:
Tap any security key
Initial login failed due to an unmet private key policy, "hardware_key".
Re-initiating login with YubiKey generated private key...
Enter password for Teleport user dev:
Tap any security key
```

Note: this should only occur when a user's role determines it's private key policy requirement, and the user does not have an existing login session which meets the required policy (expired or active).

##### User's YubiKey not connected during login

```bash
> tsh login --user=dev
Cluster "root" requires a YubiKey generated private key to login, but there
is no YubiKey connected. Please insert a YubiKey to re-initiate login...
// tsh polls the PIV library until the user connects a YubiKey (30 second timeout) or the user cancels
Re-initiating login with YubiKey generated private key.
Enter password for Teleport user dev:
Tap any security key
```

##### User's Yubikey not connected during a request

```bash
> tsh ls
Please insert the YubiKey used during login (serial number XXXXXX) to continue...
// tsh polls the PIV library until the user connects a YubiKey (30 second timeout) or the user cancels
```

##### Touch requirement

If a user has private key policy `hardware_key_touch` or `hardware_key_touch_and_pin`, then Teleport client requests will require touch (cached for 15 seconds). This will be handled by a touch prompt similar to the one used for MFA. This prompt will occur before prompting for login credentials.

```bash
> tsh login --user=dev
Enter password for Teleport user dev:
Tap any security key
Tap your YubiKey
```

##### PIN requirement

If a user has private key policy `hardware_key_pin` or `hardware_key_touch_and_pin`, then Teleport client requests will require pin. This will be handled by a password style prompt.

```bash
> tsh login --user=dev
Enter password for Teleport user dev:
Tap any security key
Enter your YubiKey PIV PIN:
```

Note: Since this prompt requires stdin, it may not work in environments that do not support stdin (ex: `ssh` with `tsh proxy ssh` ProxyCommand).

### Additional considerations

#### Database support

`tsh db connect` uses raw RSA private key data to form connections. Since this cannot be supported with hardware private keys, users will instead need to use `tsh proxy db` to connect using a local proxy. Teleport Connect already uses `tsh proxy db` and will not be affected, but the WebUI may have an additional challenge to support database connections.

Update: this has been implemented as part of [RFD 90](https://github.com/gravitational/teleport/blob/master/rfd/0090-db-mfa-sessions.md#integrating-with-piv-hardware-private-keys-for-security-improvements).

#### Kubernetes support

Kubernetes integration uses raw RSA private [key data to form connections](https://github.com/gravitational/teleport/blob/master/lib/kube/kubeconfig/kubeconfig.go#L164-L167). It may be possible to create a [custom auth provider plugin](https://pkg.go.dev/k8s.io/client-go@v0.24.3/tools/clientcmd/api#AuthProviderConfig) and supply it to the kubernetes Auth Info. Kubernetes support will be investigated and fixed in a follow up PR after the initial hardware private key implementation.

Update: this has been implemented as part of [RFD 121](https://github.com/gravitational/teleport/blob/master/rfd/0121-kube-mfa-sessions.md).

#### Application Access

Application Access can be supported by expanding the changes outlined in the
section above for [Web Sessions](#web-sessions) for App Sessions, which are
essentially just a specific type of Web Session.

When a user creates an App Session with `rpc CreateAppSession`, the Auth
service will check the user's current certificate to determine whether the App
Session should be attested as a `web_session`.

Note: This implementation will be reliant on Per-session MFA support, since
a `web_session` attestation cannot be counted as MFA verification and
therefore cannot satisfy the private key policies alone. When creating an App
Session, users will be required to perform an MFA check to mark the App Session
as MFA verified in addition to the `web_session` attestation. The details for
Per-session MFA for App Access will be covered in a separate RFD and
implemented alongside these changes.

##### Protecting attested App Sessions

Similarly to Web Sessions, App Sessions can be managed entirely by the Proxy and
Auth services. In order to ensure this is the case, `rpc GetAppSession` and
`rpc ListAppSessions` will be restricted to require `read` or `list` permissions
for `KindWebSession`. These permissions will only be granted to the Proxy
Service role.

Additionally `rpc CreateAppSession`, which is called using the user's credentials
rather than the Proxy Service's credentials, will return the App Session without
secrets.

```proto
// CreateAppSessionResponse contains the requested application web session.
message CreateAppSessionResponse {
  // Session is the application web session with secrets excluded.
  types.WebSessionV2 Session = 1 [(gogoproto.jsontag) = "session"];
}
```

Note: Since no current callers of `CreateAppSession` in the teleport code base
use the secrets returned in `CreateAppSessionResponse`, there should be no
backwards compatibility issues.

#### Agent key support

Initially, hardware private key login will not support `tsh --add-keys-to-agent`, `tsh -A`, or Proxy Recording mode, because [Adding agent keys from a hardware key](https://tools.ietf.org/id/draft-miller-ssh-agent-01.html#rfc.section.4.2.5) to a user's `ssh-agent` is [not supported in x/crypto/ssh/agent](https://github.com/golang/go/issues/16304). We can implement this support ourselves in the future.

For Yubikey, users can also [manually add their keys](https://github.com/jamesog/yubikey-ssh) to their ssh-agent with `ssh-add` after logging in. However, this will not add their SSH certificate to the ssh-agent, so some additional workaround will be needed.

#### PIV secret management

Some PIV operations require [administrative access](https://developers.yubico.com/PIV/Introduction/Admin_access.html), which require one or more of the following secrets:

| Name           | size     | default value                                      | function                                  |
|----------------|----------|----------------------------------------------------|-------------------------------------------|
| Management Key | 24 bytes | `010203040506070801020304050607080102030405060708` | private key and certificate management    |
| PIN            | 8 chars  | `123456`                                           | sign and decrypt data, reset pin          |
| PUK            | 8 chars  | `12345678`                                         | reset PIN when blocked by failed attempts |

To simplify our implementation and limit UX impact, we will assume the user's PIV device to use the default Management Key. User's can use the private `--piv-management-key` flag during login in case they need to use a non-default management key.

However, if pin is required, we must require the user to set a non-default PIN and PUK to prevent these keys from easily being accessed by attackers. To that end, if a user provides `123456` during a PIN prompt, they will be prompted to provide a new PIN and PUK before continuing. Again, the default values will not be accepted.

```bash
> tsh login --user=dev
Enter your YubiKey PIV PIN [blank to use default PIN]:
# \n
The default PIN 123456 is not supported.
Please set a new 6 digit PIN:
Enter your new YubiKey PIV PIN:
Confirm your new YubiKey PIV PIN:
Enter your YubiKey PIV PUK to reset PIN [blank to use default PUK]
# \n
The default PUK 12345678 is not supported
Please set a new 8 digit PUK:
Enter your new YubiKey PIV PUK:
Confirm your new YubiKey PIV PUK:
```
