---
authors: Alan Parra (alan.parra@goteleport.com)
state: draft
---

# RFD 54 - Passwordless for macOS CLI

## What

Passwordless features for native macOS CLIs, aka Touch ID support for CLI/`tsh`.

This is a part of the [Passwordless RFD][passwordless rfd].

## Why

Native, non-browser macOS clients lack support for Touch ID. This RFD explores
how we can achieve that support for `tsh` in a secure way.

## Details

Touch ID support is implemented via `SecAccessControl`-protected keys, which can
be either a [Keychain entry](
https://developer.apple.com/documentation/localauthentication/accessing_keychain_items_with_face_id_or_touch_id)
or a [private key stored in the Secure Enclave](
https://developer.apple.com/documentation/security/certificate_key_and_trust_services/keys/storing_keys_in_the_secure_enclave).
Both alternatives are Secure Enclave-protected, but in the latter the keys are
generated in the Enclave and never leave it, making it our approach of choice.
(See the [alternatives considered](#alternatives-considered) section for other
APIs evaluated for the design.)

In order to make use of the Keychain Sharing services, required for Secure
Enclave protection, it is necessary to enroll in the Apple Developer Program.
Furthermore, the features may only be used by a binary signed by said account
and containing entitlements similar to the ones below:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.developer.team-identifier</key>
  <string>TEAMID</string>
  <key>com.apple.application-identifier</key>
  <string>TEAMID.com.goteleport.tsh</string>

  <key>keychain-access-groups</key> <!-- aka Keychain Sharing -->
  <array>
    <string>TEAMID.com.goteleport.tsh</string>
  </array>
</dict>
</plist>
```

(CGO is used to bridge native ObjC code into the Go binaries.)

The signed Teleport binaries for macOS are updated with 1) an account enrolled
in the Apple Developer Program and 2) an entitlements file similar to the one
above. (See [Packaging a Daemon with a Provisioning Profile](
https://developer.apple.com/forums/thread/129596) or [Signing a Daemon with a
Restricted Entitlement](
https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement)
for references in how to create a command-line binary with entitlements).

<!--
TODO(codingllama): Do we need to package tsh in a macOS .app?
 If necessary it's a matter of putting together the .app skeleton and taking
 advantage of existing macOS installers to link the binaries in the .app to a
 more convenient $PATH directory.
-->

When running in a binary that isn't correctly signed or configured, `tsh` should
disable Touch ID support.

### Registration

Registration creates and saves a new key in the Secure Enclave, using a
biometric-protected entry.

The proposed UX is similar to the current experience:

```shell
$ tsh mfa add
> Choose device type [TOTP, WEBAUTHN, TOUCHID]: touchid
> Enter device name: touchid
> Tap any *registered* security key or enter a code from a *registered* OTP device: <taps>
<system shows Touch ID prompt>
> MFA device "touchid" added.
```

Under the hood, during the Touch ID prompt stage, the following happens:

1. `tsh` creates a new Secure Enclave key, using the following parameters:

    ```objc
    // (Error handling and memory management omitted for simplicity.)

    SecAccessControlRef access = SecAccessControlCreateWithFlags(
        kCFAllocatorDefault,
        kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
        kSecAccessControlPrivateKeyUsage|kSecAccessControlBiometryAny,
        NULL /* error */);

    // Use a context with a grace period so we don't ask for multiple touches
    // in a single ceremony.
    LAContext *context = [[LAContext alloc] init];
    context.touchIDAuthenticationAllowableReuseDuration = 10; // seconds

    NSDictionary attrs = @{
      // 256-bit elliptic curve keys are required by the Enclave.
      (id)kSecAttrKeyType:                (id)kSecAttrKeyTypeECSECPrimeRandom,
      (id)kSecAttrKeySizeInBits:          @256,
      (id)kSecAttrTokenID:                (id)kSecAttrTokenIDSecureEnclave,

      (id)kSecPrivateKeyAttrs: @{
        (id)kSecAttrIsPermanent:          @YES,
        (id)kSecAttrAccessControl:        (id)access,
        (id)kSecUseAuthenticationContext: (id)context,

        (id)kSecAttrApplicationLabel:     keyHandle,            // Generated UUID
        (id)kSecAttrApplicationTag:       @"llama@example.com", // user@RPID, used to scope keys
      },
    };
    SecKeyRef key = SecKeyCreateRandomKey((CFDictionaryRef)attrs, NULL /* error */);
    ```

1. `tsh` performs the key registration process, setting all parameters for a
   passwordless / resident key

    Note that Touch ID credentials are always considered (and function as)
    resident keys, even if the RPID/Teleport where to request
    [ResidentKeyRequirement = "discouraged"](
    https://www.w3.org/TR/webauthn-2/#enum-residentKeyRequirement).

1. If registration is successful, `tsh` replaces any existing keys for the
   RPID+user pair with the newly-created key. This simplifies the authentication
   ceremony and allows re-registration as a fallback mechanism.

A few parameters specified in the code example deserve note:

* [kSecAttrAccessibleWhenUnlockedThisDeviceOnly](
  https://developer.apple.com/documentation/security/ksecattraccessiblewhenunlockedthisdeviceonly?language=objc)
  requires the device to be unlocked and the user to have a password set. It is
  the more restrictive of the possible "Accessibility Values".

* [kSecAccessControlBiometryAny](
  https://developer.apple.com/documentation/security/secaccesscontrolcreateflags/ksecaccesscontrolbiometryany)
  requires a biometric check (Touch ID on macOS). It is more restrictive than
  *kSecAccessControlUserPresence* (which allows passwords), but less restrictive
  than *kSecAccessControlBiometryCurrentSet* (doesn't work with newly enrolled
  fingerprints). *kSecAccessControlBiometryAny* seems to be the sweet spot of
  security and usability.

In case of a registration failure, `tsh` must do its best to delete the
created-but-not-registered credential. If all fails, it is possible to use the
hidden [`tsh` support commands](#tsh-support-commands) for a manual cleanup.

### Authentication

Authentication offers a plethora of options, depending on both server settings
(otp, webauthn, passwordless) and client state (FIDO2 keys present, Touch ID
keys registered). In order to decide which flow to follow, `tsh` must first
assess what is possible, preferably without asking for unnecessary user
interaction.

Unlike FIDO2 keys, it is possible for `tsh` to discover if Touch ID keys are
registered in the Enclave _without_ user interaction. Because all Touch ID keys
are functionally resident keys, as long as the server supports passwordless,
then `tsh` is free to use it.

If Touch ID passwordless authentication is possible, then it is the preferred
authentication mode and is used by default.

If passwordless is not possible, but Touch ID is present, then Touch ID MFA is
also preferred and used by default.

To allow users agency over the eager behaviors of Touch ID, `tsh` is augmented
with the following flags:

* `tsh --pwdless_mode={auto,on,off}` - activate/deactivate passwordless
  authentication

    `auto` is the default behavior described above (passwordless is preferred
    if we are certain it may be used)

    `on` enables passwordless logins (`tsh login --pwdless`, described in the
    [Passwordless FIDO2 RFD][passwordless fido2], is a shortcut to it)

    `off` disables passwordless logins

* `tsh --mfa_mode={auto,platform,cross-platform}` - choose whether to use
  platform or cross-platform MFA

    `auto` is the default behavior described above, which favors Touch ID

    `platform` prefers platform authenticators, such as Touch ID, over OTP or
    portable FIDO2 keys

    `cross-platform` prefers FIDO2 or OTP (aka `tsh` behavior prior to this RFD)

Both `--pwdless_mode` and `--mfa_mode` may be specified for fine-grained control
over `tsh` authentication logic. Note that those flags apply not only to
`tsh login`, but also for any commands that require re-authentication (such as
`tsh mfa add`).

Finally, if there are Touch ID credentials for multiple users and the login user
is not known, `tsh login` may prompt the user to specify the `--user` flag.

Example of a passwordless Touch ID login:

```shell
$ tsh login --proxy=example.com
<system shows Touch ID prompt>
> > Profile URL:        https://example.com
>   Logged in as:       codingllama
>   Cluster:            example.com
>   Roles:              access, editor
>   Logins:             codingllama
>   Kubernetes:         enabled
>   Valid until:        2021-10-04 23:32:29 -0700 PDT [valid for 12h0m0s]
>   Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty
```

### Detecting Touch ID support

Detecting Touch ID support is important so `tsh` may enable/disable related
features as appropriate.

Apart from Go build tags, which are a rather coarse "detection" mechanism, we
can take inspiration from [Chromium's implementation](
https://github.com/chromium/chromium/blob/c4d3c31083a2e1481253ff2d24298a1dfe19c754/device/fido/mac/touch_id_context.mm#L126)
and do the following checks:

* Verify macOS version (>=10.12.2)
* Verify if the `keychain-access-groups` entitlement is present
* [LAContext canEvaluatePolicy:kLAPolicyDeviceOwnerAuthenticationWithBiometrics](
  https://developer.apple.com/documentation/localauthentication/lacontext/1514149-canevaluatepolicy?language=objc) check
* Secure Enclave check (attempt to create a key using
  `kSecAttrIsPermanent = @NO`)

<!--
TODO(codingllama): In theory it's all fine, but let's run some tests on older
 machines to be safe.
-->

### `tsh` support commands

The following support commands are added to `tsh` as hidden subcommands. They
are useful to diagnose and manage certain aspects of Touch ID support.

The commands are only available on macOS builds.

`tsh touchid diag` - prints diagnostics about Touch ID support (for example, if
the binary is signed, entitlements, macOS version and Touch ID availability)

`tsh touchid list` - lists currently stored credentials

`tsh touchid delete` - deletes a stored credential

```shell
$ tsh touchid diag  # diag output subject to change
> macOS version: 12.1
> Signed: yes
> Entitlements: {
>     "com.apple.application-identifier" = "K497G57PDJ.net.teleportdemo.codingllama-touchid";
>     "com.apple.developer.team-identifier" = K497G57PDJ;
>     "keychain-access-groups" =     (
>     );
> }
> LAContext check passed: yes
> Secure Enclave check passed: yes

$ tsh touchid list
<system shows Touch ID prompt>
> RPID        User    Key Handle
> ----------- ------- ------------------------------------
> example.com llama   6ed2d2e4-7933-4988-9eeb-428e8531f122
> example.com alpaca  cbf251a3-0e44-4068-87cb-91a1eb241eaf

$ tsh touchid delete 6ed2d2e4-7933-4988-9eeb-428e8531f122
<system shows Touch ID prompt>
> Credential 6ed2d2e4-7933-4988-9eeb-428e8531f122 / llama@example.com deleted.
```

### Security

A few security tradeoffs, in particular in relation to the chosen flags, are
discussed in the [Registration](#registration) section.

The security of the system is predicated in two main components: the Secure
Enclave and WebAuthn. As long as keys are created with the correct settings, it
is not possible to employ them via `tsh` unless the user passes the biometric
check. `tsh` can't exfiltrate or access key material by itself.

The server communication protocol is based on WebAuthn, as described by the
[WebAuthn](https://github.com/gravitational/teleport/blob/master/rfd/0040-webauthn-support.md)
and [Passwordless RFDs][passwordless rfd].

### UX

UX is discussed throughout the design, but here is a summary of changes:

`tsh login --proxy=example.com` will automatically do passwordless Touch ID
login, if possible (server allows passwordless, appropriate hardware exists, one
credential registered for "example.com")

`tsh login --proxy=example.com --user=llama` behaves as above, but using a
specific user

`tsh login --pwdless_mode=on --mfa_mode=platform --proxy=example.com
--user=llama` is the zero ambiguity, (needlessly) long form of the above.

`tsh mfa add` adds support for Touch ID, both for authentication and registering
new credentials.

The following hidden maintenance commands are added:

* `tsh touchid diag`
* `tsh touchid list`
* `tsh touchid delete`

Regular users shouldn't need to touch those commands, but they are available for
troubleshooting and credential management.

## Alternatives considered

### LAContext's evaluatePolicy

The [LAContext's evaluatePolicy](
https://developer.apple.com/documentation/localauthentication/lacontext/1514176-evaluatepolicy)
method may be used to trigger a Touch ID prompt. It takes a policy to evaluate
(for example, `LAPolicyDeviceOwnerAuthenticationWithBiometrics`), plus a reason
string, and replies with a boolean (success/failure) and an error.

There are a few issues that make it unsafe: evaluatePolicy returns only a
boolean, offering no features to gate access to a resource. We must tackle key
storage and management ourselves. A boolean check in a user-controlled binary is
easy to bypass, and in the case of a bypass there is no actual security provided
by the biometric check. In general, solutions based on LAContext evaluatePolicy
are security theater.

The shortcomings of evaluatePolicy highlight a few desirable properties of an
actual secure solution:

- The biometric check must offer more than a boolean result: it must gate access
  to resources and/or supply information that can't be acquired otherwise (eg,
  perform a digital signature)
- Ideally, the biometric solution stores secret information itself and never
  lets those secrets be exfiltrated (eg, Secure Enclave keys)

### ASAuthorizationPlatformPublicKeyCredentialProvider / Authentication Services

The [public-private key authentication APIs](
https://developer.apple.com/documentation/authenticationservices/public-private_key_authentication),
released in Monterey, add native WebAuthn capabilities to macOS. They are, at
first glance, an ideal fit for our needs, except for a single requirement: the
binaries using them must have a matching [associated domain entitlement](
https://developer.apple.com/documentation/xcode/supporting-associated-domains).

Simplifying Apple's documentation, declaring an associated domain such as
`example.com` has two components:

1. A server-side XML declaring the apps with access to the `webcredentials`
   service:

    https://example.com/apple-app-site-association

    ```xml
    <!-- See https://developer.apple.com/documentation/xcode/supporting-associated-domains. -->
    {
      "applinks": {
          "details": [{...}]
      },
      "webcredentials": {
          "apps": [ "TEAMID.com.example.app" ] <-- this is what we care about
      },
      "appclips": {...}
    }
    ```

2. A client-side entitlement for `webcredentials`, signed into the binary

    Example:

    ```xml
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
    <dict>
      <!-- See https://developer.apple.com/documentation/bundleresources/entitlements/com_apple_developer_associated-domains. -->
      <key>com.apple.developer.associated-domains</key>
      <array>
        <string>webcredentials:example.com</string>
      </array>
    </dict>
    </plist>
    ```

Client apps query the server-side entitlements directly from Apple servers, the
server themselves hit the corresponding domains periodically (or on first load)
and cache the entitlements.

The issue with entitlements is simple: we can't know beforehand the domains for
all `tsh` installations. Usage of the API could be possible, but would likely
require different entitlements per customer (an arrangement that might not be
allowed by Apple). It is likely possible to make use of those APIs for Teleport
Cloud, but we would need a solution for other installations regardless.

A final consequence of the above is that Passkey support (aka iCloud-stored
credentials) for CLIs is out of the roadmap for the forseeable future (but
Passkeys _can_ be used for Safari-based access).

References:

* [Supporting Passkeys](
  https://developer.apple.com/documentation/authenticationservices/public-private_key_authentication/supporting_passkeys)
  (aka Touch ID with iCloud-stored credentials)
* [Supporting Security Key Authentication Using Physical Keys](
  https://developer.apple.com/documentation/authenticationservices/public-private_key_authentication/supporting_security_key_authentication_using_physical_keys)
  (aka FIDO2 authenticators)

<!-- Links -->

[passwordless rfd]: https://github.com/gravitational/teleport/blob/master/rfd/0052-passwordless.md
[passwordless fido2]: https://github.com/gravitational/teleport/blob/master/rfd/0053-passwordless-fido2.md
