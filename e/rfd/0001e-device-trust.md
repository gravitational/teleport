---
authors: Alan Parra (alan.parra@goteleport.com)
state: implemented
---

# RFD 01e - Device Trust

## Required approvers

* Engineering: (@zmb3 || @rosstimothy)
* Security: @reed
* Product: (@xinding33 || @klizhentas)

## What

Make sure only devices registered and enrolled by Teleport may access sensitive
resources. The certificate private key is created in a secure hardware store and
can't be exfiltrated from it (for example, Apple's Secure Enclave or a TPM -
Trusted Platform Module). (Paraphrased from
[gravitational/teleport#7084](https://github.com/gravitational/teleport/issues/7084#issuecomment-1203362575).)

Device trust is a Teleport Enterprise feature. It is available as a preview in
Teleport 12 (see
[#514](https://github.com/gravitational/teleport.e/issues/514#issuecomment-1224609251)
for current status).

See also:
* [RFD 0007e - Device Trust MDM integration](
  https://github.com/gravitational/teleport.e/blob/master/rfd/0007e-device-trust-mdm-integration.md)
* [RFD 0008e - Device Trust TPM Support](
  https://github.com/gravitational/teleport.e/blob/master/rfd/0008e-device-trust-tpm.md)
* [RFD 0017e - Device Trust for Linux/TPM devices](
  https://github.com/gravitational/teleport.e/blob/master/rfd/0017e-device-trust-linux-tpm.md)

## Why

Device trust bridges one of the gaps in order to provide a full BeyondCorp
solution. It gives Teleport admins guarantees about the provenance of the
machines used and establishes a foundation for more sophisticated, device
posture-based access controls.

## Details

Device trust varies in its implementation depending on available hardware and
operating system behavior. To begin with, let's state the base guarantees we can
infer from the resulting system:

__macOS__ devices with a hardware Secure Enclave can be guaranteed to hold a
certain private key. In systems without hardware or software compromise
(firmware, OS, applications), said key is held by the Secure Enclave and can't
be exfiltrated.

macOS systems, unlike their TPM (Trusted Platform Module) counterparts, don't
provide
[quoting](https://pkg.go.dev/github.com/google/go-attestation/attest#TPM.AttestPlatform)
or
[certification](https://pkg.go.dev/github.com/google/go-attestation/attest#AK.Certify)
capabilities. This makes it impossible to acquire cryptographic proof that the
Secure Enclave holds the keys or to verify the boot chain. A good enrollment
process, therefore, goes a long way. If the enrollment process is performed by a
trusted actor that ensures neither firmware, OS, or tsh are compromised, then we
can act in confidence that private keys are indeed stored in the Secure Enclave,
greatly increasing the guarantees of the system.

macOS devices without a Secure Enclave are not supported.

__Linux__ and __Windows__ devices with a TPM do provide quoting and
certification. Quoting allows us to verify the boot chain up to the
bootloader/OS (it varies depending on specifics). Certification lets us attest
that the TPM indeed holds the private key. The underlying assumption, then, is
that the TPM corresponds to a specific device (a platform certificate
strengthens this assumption). As with Macs, a good enrollment process by trusted
actors enhances the properties of the system.

Device attestation is no panacea. A sophisticated adversary with unlimited
physical access to a device can find ways to beat the game. Strong processes
greatly increase the effectiveness of the system and layered defenses are
necessary to detect compromise.

The implementation details for TPM support is now specified in [RFD 0008e - Device Trust TPM Support](./0008e-device-trust-tpm.md)

### Enrollment ceremony

```
                                         Enrollment Overview

       ┌─┐
       ║"│
       └┬┘
       ┌┼┐
        │                               ┌────┐          ┌───┐             ┌─────┐          ┌────┐
       ┌┴┐                              │tctl│          │tsh│             │Proxy│          │Auth│
      admin                             └─┬──┘          └─┬─┘             └──┬──┘          └─┬──┘
        │1. tctl devices add --asset_tag=X│               │                  │               │
        │     --os=macos                  │               │                  │               │
        │     --enroll                    │               │                  │               │
        │─────────────────────────────────>               │                  │               │
        │                                 │               │                  │               │
        │                                 │               │ 1.1. CreateDevice│               │
        │                                 │ ─────────────────────────────────────────────────>
        │                                 │               │                  │               │
        │                                 │               │ DeviceEnrollToken│               │
        │                                 │ <─────────────────────────────────────────────────
        │                                 │               │                  │               │
        │       prints instructions       │               │                  │               │
        │<─────────────────────────────────               │                  │               │
        │                                 │               │                  │               │
        │         2. tsh device enroll --token=T          │                  │               │
        │────────────────────────────────────────────────>│                  │               │
        │                                 │               │                  │               │
        │                                 │               │ 2.1. authenticate│               │
        │                                 │               │ ─────────────────>               │
        │                                 │               │                  │               │
        │                                 │               │        ok        │               │
        │                                 │               │ <─────────────────               │
        │                                 │               │                  │               │
        │                                 │               │  2.2. EnrollDevice(token, data)  │
        │                                 │               │  (streaming, multiple steps)     │
        │                                 │               │ ─────────────────────────────────>
        │                                 │               │                  │               │
        │                                 │               │                ok│               │
        │                                 │               │ <─────────────────────────────────
        │                                 │               │                  │               │
        │                       ok        │               │                  │               │
        │<────────────────────────────────────────────────│                  │               │
      admin                             ┌─┴──┐          ┌─┴─┐             ┌──┴──┐          ┌─┴──┐
       ┌─┐                              │tctl│          │tsh│             │Proxy│          │Auth│
       ║"│                              └────┘          └───┘             └─────┘          └────┘
       └┬┘
       ┌┼┐
        │
       ┌┴┐
```

Enrollment is divided into two main phases: device registration and device
enrollment.

Phase 1, device registration, is when an admin records device information into
Teleport. Devices must be recorded before enrollment proper (phase 2), so
Teleport is in a position to check expected device information against its
database.

The `CreateDevice` RPC records device information on Teleport and returns the
enrollment token necessary to start phase 2. The enrollment token is both an
authorization component for enrollment and the means to tie the _actual_ device
to the _expected_ device.

Phase 2 is the device enrollment phase. It must be executed via `tsh` in the
device to be registered. A few allowances are made for `tsh device enroll` to
support distinct use-cases:

* If the `tsh` user is already authenticated then the present identity is used;
  otherwise
* If the `tsh` user is not authenticated, `tsh device enroll` authenticates the
  user but doesn't write any data to disk. This allows IT to enroll new devices
  without the risk of leaving powerful user certificates behind.

Enrollment varies whether the enrolled device is a macOS device (backed by
Secure Enclave) or a Linux/Windows device (backed by TPM). The `EnrollDevice`
RPC is implemented as a multi-step stream in order to simplify state
bookkeeping, a common practice throughout ceremonies in this RFD.

Device data captured during device enrollment is kept indefinitely, or until a
new enrollment ceremony is completed for that device. This includes both
DeviceCollectedData and any system-specific attestation data that could be used
to re-verify the enrollment ceremony.

<details open><summary>Enrollment RPC and messages</summary>

```proto
service DeviceTrustService {
  rpc EnrollDevice(stream EnrollDeviceRequest) (stream EnrollDeviceResponse);
}

message EnrollDeviceRequest {
  oneof payload {
    EnrollDeviceInit init = 1;
    MacOSEnrollChallengeResponse macos_challenge_response = 2;
    TPMEnrollChallengeResponse tpm_challenge_response = 3;
  }
}

message EnrollDeviceResponse {
  oneof payload {
    MacOSEnrollChallenge macos_challenge = 1;
    TPMEnrollChallenge tpm_challenge = 2;
    EnrollDeviceSuccess success = 3;
  }
}

message EnrollDeviceInit {
  // token is the device enrollment token.
  string token = 1;

  // Unique identifier for the device key.
  string credential_id = 2;

  DeviceCollectedData device_data = 3;

  // macos payload is used for macOS.
  MacOSEnrollPayload macos = 4;
  // tpm payload is used for Linux/Windows.
  TPMEnrollPayload tpm = 5;
}

message EnrollDeviceSuccess {}

// Messages below explained in following sections.

message DeviceCollectedData {}

message MacOSEnrollPayload {}
message MacOSEnrollChallenge {}
message MacOSEnrollChallengeResponse {}

message TPMEnrollPayload {}
message TPMEnrollChallenge {}
message TPMEnrollChallengeResponse {}
```

</details>

#### macOS enrollment

```
                         MacOS Enrollment

     ┌───┐                                             ┌────┐
     │tsh│                                             │Auth│
     └─┬─┘                                             └─┬──┘
       ────┐                                             │
           │ get or create device key                    │
       <───┘ (Secure Enclave)                            │
                                                         │
       │                                                 │
       ────┐                                             │
           │ collect device data                         │
       <───┘                                             │
       │                                                 │
       │EnrollDeviceInit(token, collectedData, publicKey)│
       │*stream starts*                                  │
       │─────────────────────────────────────────────────>
       │                                                 │
       │             MacOSEnrollChallenge(c)             │
       │<─────────────────────────────────────────────────
       │                                                 │
       ────┐                                             │
           │ sign c                                      │
       <───┘                                             │
       │                                                 │
       │     MacOSEnrollChallengeResponse(signed c)      │
       │─────────────────────────────────────────────────>
       │                                                 │
       │               EnrollDeviceSuccess               │
       │               *stream ends*                     │
       │<─────────────────────────────────────────────────
     ┌─┴─┐                                             ┌─┴──┐
     │tsh│                                             │Auth│
     └───┘                                             └────┘
```

macOS enrollment works by creating a Secure Enclave key in the device and
proving ownership of that key to the server. Unlike TPM-based devices, macOS
doesn't provide ways to certify the key, thus we are limited to a proof of
possession.

Key creation has the same signing and notarization requirements as
[passwordless Touch ID][tsh.app]. Access control and key creation parameters are
shown in the snippet below:

[tsh.app]: https://github.com/gravitational/teleport/blob/master/rfd/0054-passwordless-macos.md#registration

```objc
  SecAccessControlRef access = SecAccessControlCreateWithFlags(
      kCFAllocatorDefault, kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
      kSecAccessControlPrivateKeyUsage, NULL /* error */);

  NSString *label = @"com.gravitational.teleport.devicekey"
  NSDictionary *attributes = @{
    // Secure Enclave requires EC/256bit keys.
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecAttrKeySizeInBits : @256,
    (id)kSecAttrTokenID : (id)kSecAttrTokenIDSecureEnclave,

    (id)kSecPrivateKeyAttrs : @{
      (id)kSecAttrIsPermanent : @YES,
      (id)kSecAttrAccessControl : (__bridge id)access,

      // kSecAttrLabel is a human-readable label.
      (id)kSecAttrLabel : label,
      // kSecAttrApplicationLabel is used to lookup keys programatically.
      (id)kSecAttrApplicationLabel : label,
      // kSecAttrApplicationTag is a private application tag.
      // Set to the credential ID on creation.
      (id)kSecAttrApplicationTag : @"24fa09a9-17ae-4caf-82f5-75ef2bf30525",
    },
  };
  SecKeyRef privateKey = SecKeyCreateRandomKey(
      (__bridge CFDictionaryRef)attributes, NULL /* error */);
```

The combination of flags above means that `tsh` has access to the device key
without the need for user interaction. It also disables the possibility of key
migration to other devices (_ThisDeviceOnly_). Finally, it avoids the deprecated
(and somewhat too permissive)
[kSecAttrAccessibleAlwaysThisDeviceOnly](https://developer.apple.com/documentation/security/ksecattraccessiblealwaysthisdeviceonly?language=objc).

The enrollment process continues with a challenge. The challenge is a 32-bit
crypto-random byte string, not unlike a WebAuthn challenge, to be signed with
the device key. After signature verification the ceremony is completed.

A caveat of using Secure Enclave storage is that device enrollment is scoped
per-user. At the moment, the design doesn't support multiple, concurrent
enrollments for different users (although changes can be made to allow it).

<details open><summary>macOS enrollment messages</summary>

```proto
message MacOSEnrollPayload {
  // public_key is the PKIX, ASN.1 DER form public key.
  bytes public_key = 1;
}

message MacOSEnrollChallenge {
  bytes challenge = 1;
}

message MacOSEnrollChallengeResponse {
  bytes signature = 2;
}
```

</details>

#### TPM-based enrollment

TPM-based enrollment is specified in [RFD 0008e - Device Trust TPM Support](./0008e-device-trust-tpm.md#device-enrollment)

#### Device enrollment token

A device enrollment token is necessary to start the device enrollment ceremony.
Possession of a token gives the bearer powers to enroll a specific device (along
with the necessary device/`enroll` permission).

A number of precautions are taken to limit the powers of enrollment tokens:

* Device bound: tokens are tied to a particular device
* Time bound: tokens expire in a reasonably short amount of time (eg, 1h)
* Single-use: tokens are single-use and immediately spent once EnrollDevice
  starts
* Hashed in storage: tokens are hashed as a password before being recorded in
  storage (bcrypt)

#### Device collected data

Device collected data contains a small set of useful information to identify the
device. For example:

* Model Name: MacBook Pro
* Model Identifier: MacBookPro16.1
* Serial Number: XXXXXXXXXXXX
* OS version: macOS 12.5 (21G72)
* Kernel version: Darwin 21.6.0

For macOS devices, the serial number is used as the asset tag and is the main
data point verified by Teleport.

(Captured data may vary for Linux and Windows.)

<!--
macOS-centric examples:

```shell
system_profiler SPHardwareDataType  # model name/identifier, serial number, etc
system_profiler SPSoftwareDataType  # OS, kernel, etc.
system_profiler SPiBridgeDataType   # T2 firmware version, boot UUID
system_profiler SPEthernetDataType  # MAC address
system_profiler SPStorageDataType   # Volume UUIDs
```

Programmatic access to serial number:
* https://developer.apple.com/library/archive/technotes/tn1103/_index.html#//apple_ref/doc/uid/DTS10002943-CH1-TNTAG3

(similar to `ioreg -c IOPlatformExpertDevice -d 2`)
 -->

Data collected during the enrollment ceremony is kept indefinitely, or until a
new enrollment happens for the same device. The latest N entries are kept for
data collected during device authentication.

```proto
enum OSType {
  OS_TYPE_UNSPECIFIED = 0;
  LINUX = 1;
  MACOS = 2;
  WINDOWS = 3;
}

message DeviceCollectedData {
  // Time of data collection, set by the client.
  // Required.
  google.protobuf.Timestamp collect_time = 1;

  // Time of data collection, as received by the server.
  // System managed.
  google.protobuf.Timestamp record_time = 2;

  // OS used by the device.
  // Required.
  OSType os_type = 3;

  // serial_number is required for macOS devices.
  string serial_number = 4;

  string model_name = 5;
  string model_identifier = 6;
  string os_version = 7;
  string kernel_version = 8;

  // Additional fields as needed.
}
```

### Device Authentication

```
                             Device Authentication

     ┌───┐                                                          ┌────┐
     │tsh│                                                          │Auth│
     └─┬─┘                                                          └─┬──┘
       ────┐                                                          │
           │ read device serial number                                │
       <───┘                                                          │
       │                                                              │
       ────┐                                                          │
           │ get device key                                           │
       <───┘                                                          │
       │                                                              │
       ────┐                                                          │
           │ collect device data                                      │
       <───┘                                                          │
       │                                                              │
       │AuthenticateDeviceInit(credentialID, collectedData, userCerts)│
       │──────────────────────────────────────────────────────────────>
       │                                                              │
       │                AuthenticateDeviceChallenge(c)                │
       │<──────────────────────────────────────────────────────────────
       │                                                              │
       ────┐                                                          │
           │ sign c                                                   │
       <───┘                                                          │
       │                                                              │
       │        AuthenticateDeviceChallengeResponse(signed c)         │
       │──────────────────────────────────────────────────────────────>
       │                                                              │
       │                 UserCertificates                             │
       │                   TLS with device extensions                 │
       │                   SSH with device extensions                 │
       │<──────────────────────────────────────────────────────────────
     ┌─┴─┐                                                          ┌─┴──┐
     │tsh│                                                          │Auth│
     └───┘                                                          └────┘
```

Device authentication augments existing user certificates (TLS and SSH) with
device extensions. The ceremony is a challenge/response streaming RPC that
exchanges the current certificates and a solved challenge for new certificates.

Device authentication as a separate RPC has a few interesting properties, such
as the ease of integration with existing authn endpoints, no need for persistent
challenge storage and no need to emit or manage unauthenticated device
challenges (otherwise necessary for SSO and passwordless).

TLS certificates with device extensions contain the custom OIDs below, embedded
in the Subject's CN (Common Name) field. The presence of (valid) device
extensions authorizes the user to perform device aware actions.

* `1.3.9999.3.1`: device ID
* `1.3.9999.3.2`: asset tag
* `1.3.9999.3.3`: credential ID

<!--
https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/lib/tlsca/ca.go#L301
 -->

SSH certificates are augmented with the following device extensions, encoded as
[SSH extensions](https://pkg.go.dev/golang.org/x/crypto/ssh#Certificate),
equivalent to the ones above:

* `teleport-device-id`
* `teleport-device-asset-tag`
* `teleport-device-credential-id`

<!--
https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/constants.go#L457
 -->

<details open><summary>Device authentication RPC and messages</summary>

```proto
service DeviceTrustService {
  // (...)
  rpc AuthenticateDevice(stream AuthenticateDeviceRequest) (stream AuthenticateDeviceResponse);
}

message AuthenticateDeviceRequest {
  oneof payload {
    AuthenticateDeviceInit init = 1;
    AuthenticateDeviceChallengeResponse challenge_response = 2;
  }
}

message AuthenticateDeviceResponse {
  oneof payload {
    AuthenticateDeviceChallenge challenge = 1;
    UserCertificates user_certificates = 2;
  }
}

message AuthenticateDeviceInit {
  // In-band user certificates to augment with device extensions.
  // - The x509 certificate is acquired from the mTLS connection, thus the
  //   in-band certificate is ignored.
  // - All certificates must be valid and issued by the Teleport CA.
  // - All certificates must match (same public key, same Teleport user, plus
  //   whatever additional checks the backend sees fit).
  // - Augmented certificates have the same expiration as the original
  //   certificates.
  UserCertificates user_certificates = 1;

  string credential_id = 2;
  DeviceCollectedData device_data = 3;
}

message AuthenticateDeviceChallenge {
  bytes challenge = 1;
}

message AuthenticateDeviceChallengeResponse {
  bytes signature = 1;
}

message UserCertificates {
  // DER-encoded X.509 user certificate, augmented with device extensions.
  bytes x509_der = 1;

  // SSH certificate marshaled in the authorized key format.
  bytes ssh_authorized_key = 2;
}
```

</details>

### Authorization

Possession of a user certificate with valid device extensions (device ID, asset
tag and credential ID) authorizes the user to perform device aware actions.

Not all endpoints are practical to guard using device trust. For example,
authentication and Device Trust endpoints are required for bootstrapping,
endpoints required by the Web UI must be able to function without device trust,
etc.

The following accesses may be configured to require a trusted device (`tsh`):

* Connecting to a node via SSH
* Connecting to a database
* Connecting to a Windows Desktop
* Connecting to an app
* Connecting to Kubernetes

Additionally, the following administrative actions may also require a trusted
device:

* Creating, updating and deleting resources (including roles and users)

    See
    [OSS](https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/tool/tctl/common/resource_command.go#L93)
    and
    [Enterprise](https://github.com/gravitational/teleport.e/blob/7f65ada14e1fd422e642a95f544ec063a8061fe3/tool/tctl/resource_command.go#L35)
    `tctl` sources for reference.

(Additional protections likely to be added over time.)

Authz guards around device registration can cause a situation where all admins
are locked out due to non-enrolled devices. For this reason, endpoints necessary
to register and enroll devices are not guarded by device aware checks.

In order to pass device challenges on macOS, `tctl` needs to be signed,
notarized and packaged in a similar manner to tsh.app, as explained by the
[passwordless RFDs][tsh.app]. This has cascading changes into Teleport packages
and installers (to be decided outside of the RFD).

Online blocking of devices is handled by the [locking subsystem](#locks).

### Audit log

Device Trust adds the `DeviceEvent` type, with the following event codes:

* `TV001I` / DeviceCreate: fired by registration
* `TV002I` / DeviceDelete: fired by device removal
* `TV003I` / DeviceEnrollTokenCreate: fired by registration or standalone token
  creation
* `TV004I` / DeviceEnrollTokenSpent: fired by [enrollment](#enrollment-ceremony)
* `TV005I` / DeviceEnroll: fired by [enrollment](#enrollment-ceremony)
* `TV006I` / DeviceAuthenticate: fired by
  [device authentication](#device-authentication)

The events above are meant to describe the lifecycle of a device, encompassing
multiple owners and potentially outliving the device itself.

Note that [audit log retention is configurable](
https://goteleport.com/docs/reference/backends/?scope=enterprise#dynamodb) and
may vary between Teleport installations (defaults to 365d).

<!--
If the retention period above is insufficient we could keep a permanent history
of key events (Create, Delete and Enroll events) in a dedicated history table.
 -->

Additionally to the events above, if the user certificate contains device
extensions, these are logged beside the
[UserMetadata](https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/api/types/events/events.proto#L60)
in the audit log.

<details open><summary>Audit log event definitions</summary>

```proto
message DeviceMetadata {
  string device_id = 1;
  devicetrust.OSType os_type = 2;
  string asset_tag = 3;
  string credential_id = 4;
}

message DeviceEvent {
  Metadata metadata = 1; // embedded
  Status status = 2;
  ConnectionMetadata connection = 3;
  DeviceMetadata device = 4;
  UserMetadata user = 5;
}
```

<details open><summary>UserMetadata changes</summary>

```diff
// UserMetadata is a common user event metadata
message UserMetadata {
  // Existing fields omitted.

+  // TrustedDevice contains information about the users' trusted device.
+  // Requires a registered and enrolled device to be used during authentication.
+  DeviceMetadata TrustedDevice = 8 [(gogoproto.jsontag) = "trusted_device,omitempty"];
}
```
</details>

### Configuration and roles

There are two large buckets of configurations:

1. Device trust mode, specifying how Teleport handles device authz
2. Device inventory management, including powers to manage and enroll devices

The device trust mode is controlled via Teleport configuration and roles.

teleport.yaml:

```yaml
auth_service:
  authentication:
    device_trust:
      # Device trust mode.
      # * `off` disables device trust.
      #   Only mode available for OSS.
      # * `optional` means Teleport attempts use Device Trust extensions for
      #   enrolled devices, but they aren't enforced.
      #   Default for Enteprise.
      # * `required` means that Teleport enforces Device Trust extensions for
      #   the entire cluster, requiring an enrolled device to use/access SSH
      #   sessions, Databases, Windows Desktops, Apps and Kubernetes.
      mode: optional
```

<details open><summary>cluster_auth_preference / AuthPreferenceSpecV2</summary>

<!--
https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/api/types/types.proto#L1106
 -->

```diff
message AuthPreferenceSpecV2 {
  // (...)
+
+   DeviceTrust device_trust = 12;
}
```

```proto
message DeviceTrust {
  string mode = 1; // "off", "optional" or "required"
}
```

</details>

Instead of enforcing cluster-wide device trust, admins may enable it on a
per-role basis, similarly to `require_session_mfa`:

<!--
https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/api/types/types.proto#L1635
 -->

```diff
message RoleOptions {
  // (...)
+
+  string device_trust_mode = 22;
}
```

Note that, in case of multiple modes affecting the same resources (multiple
roles and/or cluster-wide configuration), the strictest mode takes effect.

Access to device inventory management (2) is controlled via roles, using the
"device" resource kind. All standard verbs (`create`, `delete`, `list`, `read`,
`update`) are supported, plus the following:

* `create_enroll_token` - allows creation of enrollment tokens
* `enroll` - allows enrollment of devices

The builtin
[editor](https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/lib/services/presets.go#L31)
preset is modified to include all "device" permissions listed above (new
installations only).

### Device management

#### Device resource

A "device" resource is added to Teleport, with `tctl` support for create, read
and delete operations.

Example commands:

* `tctl create device.yaml` - creates the specified device(s), useful for bulk
  operations
* `tctl get devices` - lists all devices (detailed view)
* `tctl get devices/<ID or tag>` - read by device ID or asset tag
* `tctl rm devices/<ID or tag>` - hard-delete by device ID or asset tag (must
  match a single device).

Device resource definition:

```proto
package types; // api/types

// DeviceV1 represents a device resource, as defined by Device Trust.
// Device Trust is a Teleport Enterprise feature.
message DeviceV1 {
  string Kind = 1;        // "device"
  string SubKind = 2;     // unused
  string Version = 3;     // "v1", copied from Spec.api_version.
  Metadata Metadata = 4;  // Metadata.Name is set to Spec.asset_tag.
                          // Other Metadata fields unused.

  // Spec is lifted straight from the Device Trust API, instead of going for the
  // more usual, locally defined, "DeviceSpecV1" proto.
  devicetrust.Device Spec = 5;
}
```

#### `tctl devices` subcommands

A family of `tctl devices` subcommands is added in order to cover use-cases or
express functionality that the "device" resource cannot.

Example commands:

* `tctl devices add` - allows simultaneous creation of device and enrollment
  token
* `tctl devices ls` - single-line, abridged list
* `tctl devices rm` - allows unambiguous deletion of devices by either device ID
  or asset tag
* `tctl devices enroll` - creates a new device enrollment token
* `tctl devices lock` - locks device by either device ID or asset tag

See the [UX](#ux) section for a comparison of "device" resource and `tctl
devices` commands.

#### Locks

Devices may be locked using the `tctl lock --device <device ID>` or `tctl
create` commands.

[LockTarget](https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/api/types/types.proto#L3283)
is updated to support devices, as shown below:

```diff
message LockTarget {
  // (...)

+   // Device specifies the device ID of an enrolled device.
+   // Requires Teleport Enterprise.
+   string Device = 8 [ (gogoproto.jsontag) = "device,omitempty" ];
}
```

### Device Trust API

A device inventory management API is provided, with RPCs to create, read, list
and delete devices, among other functionality.

<details open><summary>Device Trust RPCs and messages</summary>

```proto
service DeviceTrustService {
  // (...)

  // Device inventory RPCs.
  // (GetDevice and UpdateDevice omitted in the current iteration.)
  rpc CreateDevice(CreateDeviceRequest) returns (Device);
  rpc FindDevices(FindDevicesRequest) returns (FindDevicesResponse);
  rpc ListDevices(ListDevicesRequest) returns (ListDevicesResponse);
  rpc DeleteDevice(DeleteDeviceRequest) returns (google.protobuf.Empty);

  // CreateDevice alternative focused on bulk insertion.
  // Does not support creation of enrollment tokens.
  rpc BulkCreateDevice(BulkCreateDeviceRequest) returns (BulkCreateDeviceResponse);

  // Enrollment support.
  rpc CreateDeviceEnrollToken(CreateDeviceEnrollTokenRequest) returns (DeviceEnrollToken);
}

message CreateDeviceRequest {
  Device device = 1;

  // If true, the returned device has an enrollment token set.
  bool create_enroll_token = 2;
}

message BulkCreateDeviceRequest {
  repeated Device devices = 1;
}

message BulkCreateDeviceResponse {
  repeated DeviceOrStatus devices = 1;
}

message DeviceOrStatus {
  // Status of the device creation or update.
  google.rpc.Status status = 1;

  // ID is the created device ID.
  // Only present if the status is successful.
  string id = 2;
}

// Finds a device by device ID or asset_tag.
// Similar to a Get, but the response isn't guaranteed to be a single device.
message FindDevicesRequest {
  string id_or_tag = 1;
}

message FindDevicesResponse {
  // Devices matching the FindDevicesRequest.
  // Numbers of devices expected to be low, may be artifically capped by the
  // server otherwise.
  repeated Device devices = 2;
}

// Paginated list without filters.
// See https://cloud.google.com/apis/design/standard_methods#list.
message ListDevicesRequest {
  // Maximum page_size enforced by the server, responses may be silently
  // reduced to the max.
  int32 page_size = 1;
  string page_token = 2;

  // See https://cloud.google.com/apis/design/design_patterns#resource_view.
  // Defaults to DEVICE_LIST_VIEW.
  DeviceView view = 3;

  // For complex searches that don't adhere to list semantics, a separate Search
  // RPC is recommended.
  // https://cloud.google.com/apis/design/custom_methods#common_custom_methods
}

enum DeviceView {
  DEVICE_VIEW_UNSPECIFIED = 0;

  // View used for general device listings, like `tctl devices ls`.
  // Contains only basic information, such as IDs and enrollment status.
  DEVICE_LIST_VIEW = 1;

  // View used for detailed device queries, like `tctl get devices`.
  // Presents a complete view of the device.
  DEVICE_RESOURCE_VIEW = 2;
}

message ListDevicesResponse {
  repeated Device devices = 1;
  string next_page_token = 2;
}

// Device hard-delete.
// Not recommended for enrolled devices, as history will be lost.
// Prefer locking instead.
message DeleteDeviceRequest {
  string device_id = 1;
}

message CreateDeviceEnrollTokenRequest {
  string device_id = 1;
}

message Device {
  // API version of the Device definition, present for compatibility with
  // types.DeviceV1.
  // Set to "v1".
  string api_version = 1;

  // ID is the device identifier.
  // System managed.
  string id = 2;

  // Device operating system.
  // Required.
  OSType os_type = 3;

  // Asset tag is the inventory device identifier.
  // May take different meanings depending on the device and operating system.
  // For macOS, it's the device serial number.
  // Required.
  string asset_tag = 4;

  // Create time.
  // System managed.
  google.protobuf.Timestamp create_time = 5;

  // Last update time.
  // System managed.
  google.protobuf.Timestamp update_time = 6;

  // Enrollment token for the device.
  // Only present in situations where device creation and enrollment are rolled
  // into a single operation.
  // Transient.
  DeviceEnrollToken enroll_token = 7;

  // Enrollment status of the device.
  // System managed.
  DeviceEnrollStatus enroll_status = 8;

  // Currently enrolled device credential.
  // System managed.
  DeviceCredential credential = 9;

  // Device data collected during enrollment and device authentication.
  // Enrollment data is always present, while authentication data is capped at N
  // most recent events.
  // Only present in certain read modes.
  // Read-only.
  repeated DeviceCollectedData collected_data = 10;
}

message DeviceCredential {
  string id = 1;

  // public_key is the PKIX, ASN.1 DER form public key.
  bytes public_key = 2;
}

enum DeviceEnrollStatus {
  DEVICE_NOT_ENROLLED = 0;
  DEVICE_ENROLLED = 1;
}

message DeviceEnrollToken {
  string token = 1;
}
```

</details>

### Storage

The keyspace below is added to storage:

* `devices/id/$dev_id`:
  base device information, serves authz and list requests by itself
* `devices/enroll_token/$dev_id`:
  device enrollment token
* `devices/collected_data/$dev_id/$data_id`:
  records last N instances of collected data
* `devices/byTag/$asset_tag`:
  maps an asset tag device IDs

`devices/id/$dev_id` stores a stripped-down Device proto (update_time, collected
data and enroll_token are always removed).

`devices/enroll_token/$dev_id` stores a StoredEnrollToken. It contains the
bcrypt-hashed enrollment token. Expires in a short time-frame (eg, 1h).

`devices/collected_data/$dev_id/$data_id` stores a StoredCollectedData
proto. Up to N (eg, 10) entries are kept; the system automatically deletes
older entries. The special ID `1` is used for the enrollment data - this ID is
never deleted and doesn't count for the N most recent entries.

`devices/byTag/$asset_tag` stores a manual index of asset tag to device ID, in
the form of an DevicesRef proto.

(Note: objects are marshaled as JSON for storage.)

```proto
message StoredEnrollToken {
  // Device enrollment token, hashed using bcrypt.
  bytes hashed_token = 1;
}

enum CollectedDataOrigin {
  COLLECTED_DATA_ORIGIN_UNSPECIFIED = 0;

  // Data collected for the enrollment ceremony.
  ORIGIN_ENROLLMENT = 1;

  // Data collected for device authentication.
  ORIGIN_DEVICE_AUTHENTICATION = 2;
}

message StoredCollectedData {
  CollectedDataOrigin origin = 1;

  DeviceCollectedData device_data = 2;

  // Additional data captured during the enrollment ceremony.
  MacOSEnrollData macos_enroll_data = 3;
  TPMEnrollData tpm_enroll_data = 4;
}

message MacOSEnrollData {
  // challenge is the macOS enrollment challenge.
  bytes challenge = 1;
  // signature is a signature over challenge, by the device key.
  bytes signature = 2;
}

message TPMEnrollData {
  bytes ek_public = 1;

  // Reserved for the Attestation Key.
  // DeviceCredential holds the Attestation Key.
  reserved 2;

  TPMAttestationData attestation_data = 3;
}

message DevicesRef {
  repeated DeviceRef devices = 1;
}

message DeviceRef {
  string device_id = 1;
}
```

### OSS and Enterprise

Device Trust is an Enterprise feature, but certain aspects of it must live in
the open source Teleport repository.

OSS contains:

Feature                      | Reason
---                          | ---
`tsh device enroll`          | May be verified by the community<br/>Doesn't require Enterprise packages for clients<br/>Easier Connect integration (doesn't require "Connect Enterprise")
Audit logs                   | Easier to integrate into existing codebase
Configuration and Role knobs | Easier to integrate into existing codebase
Locking                      | Easier to integrate into existing codebase
Device aware authorization   | Endpoints are largely OSS
Device Trust RPC definitions | Necessary for integration with Auth

Enterprise contains all the remaining implementation, including:

* DeviceTrustService implementation
* Storage implementation (`devices/` keyspace)
* `tctl` device resource commands
* `tctl devices` subcommands

### Web UI

`tsh` and Connect are first-class citizens for Device Trust. In particular, for
macOS, they both fulfill the necessary signature/notarization requirements for
Secure Enclave usage.

Web UI access for Device Trust endpoints is currently out-of-scope. Browsers are
not allowed access to `tsh`-registered keys, so without additional components
the Web UI is not in a position to solve device challenges.

### Security

Device Trust adds a few RPCs, behind DeviceTrustService, all requiring
authorized access. Existing endpoints are altered in minor ways, without
significant impact.

The design adds additional authz criteria to Teleport, but these shouldn't
weaken the system - the question is whether Device Trust itself adds a
meaningful authz component and if the ceremonies are sound.

__Device enrollment__ relies on:

* User authorization, namely the device/`enroll` permission,
* The device enrollment token, which both grants authorization to enroll a
  particular device and ties the device back to the inventory,
* Proof of possession of the device private key; and
* Validation of the collected device data (OS and serial number)

The enrollment ceremony proves the ownership of a private key. The process is
strengthened when performed by a trusted operator - in this scenario, the
assumption that the private key lies under secure storage in a specific device
is stronger. Device enrollment tokens have short expirations (in human terms)
and are spent immediately on first use.

The outcome of enrollment is a registered device credential.

__Device authentication__ relies on:

* Valid user certificates (TLS and SSH)
* Validation of the collected device data (matched against enrollment and
  previous data)
* Proof of possession of the device private key

The inputs above are exchanged for certificates with device extensions. The
device extensions are used, then, to authorize access to device aware endpoints.

Both __device enrollment__ and __device authentication__ endpoints are rate
limited.

__Authorization__ for device aware endpoints looks at the device extensions in
the user certificate in order to make access decisions. Device locks inspect the
same extensions, providing the means to block untrustworthy (enrolled) devices.

Access to device aware endpoints is controlled either by cluster-wide
configuration (`device_trust.mode`) or Role options (`device_trust_mode`).

Device inventory management and enrollment are controlled by verbs attached to
the "device" resource (CRUD verbs, plus `create_enroll_token` and `enroll`).

### UX

Device enrollment, in its simplest form, is a sequence of the following
commands:

```shell
# run by an admin, in any machine:
# --enroll means "create an enrollment token"
$ tctl devices add --os=macos --asset_tag=X --enroll
> Device X/macos added to the inventory.
> Run the command below on device "X" to enroll the device:
> tsh device enroll --token=Y

# run by an admin/user in device "X":
$ tsh device enroll --token=Y
> Device "X" enrolled.
```

It's possible to bulk register devices (`tctl create devices.yaml`), which won't
create enrollment tokens. It's also possible to manually create new enrollment
tokens, either for initial enrollment or to re-enroll a device (`tctl devices
enroll --device-id=N`).

Inventory management is performed either via `tctl devices` subcommands or using
the `tctl` device resource. `tctl` resources are preferred for detailed views
(JSON or YAML) and for bulk management; `tctl devices` subcommands bridge the
gap where resources commands fall short.

The table below compares the alternatives, highlighting differences in
functionality and format:

Command                 | Details
---                     | ---
tctl devices add        | single device<br/>may create enrollment token
tctl create device.yaml | single or multiple devices<br/>doesn't create enrollment token
tctl devices ls         | simplified view, one device per row
tctl get devices        | full view
tctl get devices/X      | reads a single device by ID or tag<br/>doesn't have a `tctl devices` counterpart
tctl devices rm         | explicit --device-id or --asset-tag
tctl rm devices/X       | uses either device ID or tag<br/>may fail in case of ambiguity
tctl devices lock       | locks device by ID or asset tag
tctl lock --device      | locks device by ID<br/>tctl lock interface

User visible configuration is explored in the
[configuration and roles](#configuration-and-roles) section.

See below for examples of the complete UX of new `tctl` and `tsh` commands.

```shell
# Options between [square brackets] are optional.

$ tctl devices add --os=linux|macos|windows --asset-tag=<serial_number> [--enroll]

$ tctl devices ls

# Either --device-id or --asset-tag must be present.
$ tctl devices rm [--device-id=X] [--asset-tag=Y]

# Creates a device enrollment token.
# Either --device-id or --asset-tag must be present.
$ tctl devices enroll [--device-id=X] [--asset-tag=Y]

# Reads devices by device ID or asset tag.
# Prints multiple devices in case of ambiguity.
$ tctl get devices/<device ID or asset tag>

# Deletes a device by ID or asset tag.
# Fails in case of ambiguity; in those cases, `tctl devices rm` is preferred.
$ tctl rm devices/<device ID or asset tag>

# Locks device by ID or asset tag.
$ tctl devices lock [--device-id=X] [--asset-tag=Y]

# Locks device by device ID.
$ tctl lock --devices=X

# Enrolls a device using a device enrollment token.
$ tsh device enroll --token=T
```

### Alternatives considered

#### Auto-enroll

Auto-enroll allows users to enroll a device without an enrollment token. The
capability to auto enroll is granted by the device/`auto_enroll` permission.

Because of the lack of an enrollment token, auto-enroll removes an important
verification from the system, which is checking that the enrolled machine
matches what the admin expect it to be. For that reason, the alternative is
considered out of scope for the design, but it may be brought back if the
convenience provided appears greater than the risk.

There are broadly two ways for auto-enrollment to function:

1. The enrolled device has to be in the inventory, but as long as it matches _a_
   device in the inventory, then enrollment is allowed to proceed.

2. The device doesn't have to be in the inventory. In this scenario,
   auto-enrollment would automatically register the device as well.

#### Device session verification

Using a mechanism similar to the proposed device challenges, it's possible to
add a device check akin to `require_session_mfa`. The check adds a extra layer
of confidence, as it includes a proof of possession of the device key before
sensitive operations, thus reducing the power of an exfiltrated user
certificate.

Device session checks aren't fundamental for the functioning of device trust, so
they are considered initially out of scope, but it's a worth feature for a
follow up.

#### Device certificates

Initial designs included a device certificate. Enrollment and device
authentication would change, roughly, as follow:

1. Enrollment emits a device certificate in the final step

    (macOS) The device certificate is stored in the Keychain

2. Device authentication exchanges collected data, device challenge and a valid
   device certificate for a user certificate with device extensions

3. A device certificate renewal endpoint is added to the system, allowing the
   exchange of collected data and a device challenge for a renewed certificate.

    `tsh` automatically renews the device certificate when necessary.

The device certificate signifies that the machine successfully cleared collected
data validation and proof of possession in a recent enough time.

Using the device certificate in place of a challenge/proof of possession means
that an exfiltrated device certificate grants "device powers" to any who hold
it; thus the design often required both certificate and challenge, making the
former redundant.

The certificate adds value if the device were to communicate directly with
Teleport, without a user certificate present, but in the proposed design that
doesn't happen. In the end, cutting the device certificate makes for a simpler
design (less endpoints and moving parts) while keeping the same guarantees.

#### DCDevice and AppAttestService (macOS)

[DCDevice](https://developer.apple.com/documentation/devicecheck/accessing_and_modifying_per-device_data)
is an API that allows storing 2 bits of data about a device on an Apple server.
In short, the device creates an ephemeral token that identifies it and relays
that token to the server, which in turn uses the token to access the device's
bits stored by Apple.

While DCDevice does, somehow, identify the device, it doesn't share that
information - the device token is ephemeral and cannot be used as an identifier.
The bits of storage could be useful to identify duplicate enrollments, for
example, but that isn't a design necessity.

[AppAttestService](https://developer.apple.com/documentation/devicecheck/establishing_your_app_s_integrity)
provides the means to verify the authenticity of an app - a useful feature that
could establish if `tsh.app` or Connect were tampered with. Unfortunately, at
the time of writing, [it doesn't support Mac devices](
https://developer.apple.com/documentation/devicecheck/dcappattestservice/3573915-supported).

#### Simultaneous user/device authentication

```
                                          MFA Authentication

     ┌───┐                                   ┌─────┐                          ┌────┐
     │tsh│                                   │Proxy│                          │Auth│
     └─┬─┘                                   └──┬──┘                          └─┬──┘
       ────┐                                    │                               │
           │ read device serial number          │                               │
       <───┘                                    │                               │
       │                                        │                               │
       ────┐                                    │                               │
           │ get device key                     │                               │
       <───┘                                    │                               │
       │                                        │                               │
       │POST /webapi/mfa/login/begin            │                               │
       │  device_credential_id: <credential ID> │                               │
       │───────────────────────────────────────>│                               │
       │                                        │                               │
       │                                        │  CreateAuthenticateChallenge  │
       │                                        │───────────────────────────────>
       │                                        │                               │
       │                                        │                               │────┐
       │                                        │                               │    │ create and store
       │                                        │                               │<───┘ device challenge
       │                                        │                               │
       │                                        │                               │
       │                                        │MFAAuthenticateChallenge       │
       │                                        │  device_challenge: []byte{...}│
       │                                        │<───────────────────────────────
       │                                        │                               │
       │     MFAAuthenticateChallenge{...}      │                               │
       │<───────────────────────────────────────│                               │
       │                                        │                               │
       ────┐                                    │                               │
           │ sign userPubKey||device_challenge  │                               │
       <───┘ using the device key               │                               │
                                                │                               │
       │                                        │                               │
       │POST /webapi/mfa/login/finish           │                               │
       │  device_credential_id: <credential ID> │                               │
       │  device_challenge: <signed challenge>  │                               │
       │───────────────────────────────────────>│                               │
       │                                        │                               │
       │                                        │      AuthenticateSSHUser      │
       │                                        │───────────────────────────────>
       │                                        │                               │
       │                                        │     user certificate with     │
       │                                        │     device extensions         │
       │                                        │<───────────────────────────────
       │                                        │                               │
       │         user certificate with          │                               │
       │         device extensions              │                               │
       │<───────────────────────────────────────│                               │
     ┌─┴─┐                                   ┌──┴──┐                          ┌─┴──┐
     │tsh│                                   │Proxy│                          │Auth│
     └───┘                                   └─────┘                          └────┘
```

Simultaneous user/device authentication plugs the Device Authentication ceremony
into existing authentication endpoints for Teleport.

While, in general, more performant than the proposed
[Device Authentication](#device-authentication) ceremony (saving one or two hops
depending on the scenario), it's more complex to implement and has a higher
impact on existing endpoints. In particular:

* All authn endpoints are affected,
* It's harder to cleanly split between OSS and Enterprise,
* It requires persistent device challenge storage,
* Device Trust has to issue non-authenticated/anonymous challenges
  (SSO and passwordless ceremonies),
* Single-hop authentication endpoints (like
  [directLogin](https://github.com/gravitational/teleport/blob/be1438aecddd3b3b2104b25ff986b772fb52e665/lib/client/api.go#L3344))
  have to be deprecated and removed; and
* SSO endpoints need an additional hop to acquire the challenge and
  modifications to store device-related data.

Given the impact of the changes, the more independent streaming solution is
considered a better approach. As a future improvement, simultaneous
authentication could be integrated into select, frequently used endpoints, while
falling back to the streaming device authentication for others.

<details><summary>Simultaneous MFA/Device authn (draft)</summary>

```proto
package devicetrust;

message DeviceAuthenticateChallenge {
  bytes challenge = 1;
}

message DeviceAuthenticateChallengeResponse {
  string credential_id = 1;

  // signature over pubKey||challenge, using the device key.
  bytes signature = 2;
}


package proto; // aka api/client/proto

message CreateAuthenticateChallengeRequest {
  oneof Request {
    // (...)
  }
  string device_credential_id = 2;
}

message MFAAuthenticateChallenge {
  // (...)
  DeviceAuthenticateChallenge device_challenge = 4;
}

message MFAAuthenticateResponse {
  oneof Response {
    // (...)
  }
  DeviceAuthenticateChallengeResponse device_challenge = 4;
}
```

</details>

#### Unified user authentication

Unified user authentication is the next step from simultaneous authentication:
it takes advantage of the work necessary to modify the authn endpoints, but
instead chooses to refactor __all__ authn into a single, multi-step ceremony.

<!--
Related: https://github.com/gravitational/teleport/issues/7493
 -->

A unified ceremony could greatly reduce complexity in the codebase, while at the
same time allowing features like Device Trust to be easily plugged into it.

Like simultaneous authentication, it is considered out of scope for the proposed
design, but it's an interesting alternative to pursue in a smaller design of its
own.


<!-- Plant UML diagrams -->
<!--
```
@startuml
title Enrollment Overview

actor admin as adm
participant tctl
participant tsh
participant Proxy as proxy
participant Auth as auth

adm -> tctl: 1. tctl devices add --asset_tag=X\n     --os=macos\n     --enroll
tctl -> auth: 1.1. CreateDevice
auth -> tctl: DeviceEnrollToken
tctl -> adm: prints instructions

adm -> tsh: 2. tsh device enroll --token=T
tsh -> proxy: 2.1. authenticate
proxy -> tsh: ok
tsh -> auth: 2.2. EnrollDevice(token, data)\n(streaming, multiple steps)
auth -> tsh: ok
tsh -> adm: ok
@enduml
```

```
@startuml
title MacOS Enrollment

participant tsh as tsh
participant Auth as auth

tsh -> tsh: get or create device key\n(Secure Enclave)
tsh -> tsh: collect device data

tsh -> auth: EnrollDeviceInit(token, collectedData, publicKey)\n*stream starts*
auth -> tsh: MacOSEnrollChallenge(c)

tsh -> tsh: sign c
tsh -> auth: MacOSEnrollChallengeResponse(signed c)
auth -> tsh: EnrollDeviceSuccess\n*stream ends*
@enduml
```

```
@startuml
title TPM Enrollment

participant tsh as tsh
participant Auth as auth

tsh -> tsh: fetch EKs
tsh -> tsh: create new AK
tsh -> tsh: assemble attestation data
tsh -> tsh: create new application key
tsh -> tsh: assemble certification parameters

tsh -> tsh: collect device data

tsh -> auth: EnrollDeviceInit(token, collectedData, attestations)\n*stream starts*
auth -> tsh: TPMEnrollChallenge(cred, encryptedSecret)

tsh -> tsh: activate credential (cred, encryptedSecret)
tsh -> auth: TPMEnrollChallengeResponse(secret)
auth -> tsh: EnrollDeviceSuccess\n*stream ends*
@enduml
```

```
@startuml
title Device Authentication

participant tsh
participant Auth as auth

tsh -> tsh: read device serial number
tsh -> tsh: get device key
tsh -> tsh: collect device data

tsh -> auth: AuthenticateDeviceInit(credentialID, collectedData, userCerts)
auth -> tsh: AuthenticateDeviceChallenge(c)

tsh -> tsh: sign c
tsh -> auth: AuthenticateDeviceChallengeResponse(signed c)
auth -> tsh: UserCertificates\n  TLS with device extensions\n  SSH with device extensions
@enduml
```

```
@startuml
title MFA Authentication

participant tsh
participant Proxy as proxy
participant Auth as auth

tsh -> tsh: read device serial number
tsh -> tsh: get device key

tsh -> proxy: POST /webapi/mfa/login/begin\n  device_credential_id: <credential ID>
proxy -> auth: CreateAuthenticateChallenge
auth -> auth: create and store\ndevice challenge
auth -> proxy: MFAAuthenticateChallenge\n  device_challenge: []byte{...}
proxy -> tsh: MFAAuthenticateChallenge{...}

tsh -> tsh: sign userPubKey||device_challenge\nusing the device key

tsh -> proxy: POST /webapi/mfa/login/finish\n  device_credential_id: <credential ID>\n  device_challenge: <signed challenge>
proxy -> auth: AuthenticateSSHUser
auth -> proxy: user certificate with\ndevice extensions
proxy -> tsh: user certificate with\ndevice extensions
@enduml
```
 -->
