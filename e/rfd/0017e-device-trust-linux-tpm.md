---
authors: Alan Parra (alan.parra@goteleport.com)
state: draft
---

# RFD 0017e - Device Trust for Linux/TPM devices

## Required approvers

* Engineering: (@zmb3 || @rosstimothy)
* Security: @reed || @jentfoo
* Product: (@xinding33 || @klizhentas)

## What

Describe Linux-specific adjustments to the [Device Trust TPM RFD][dt-tpm-rfd].

See:

* [RFD 0001e - Device Trust](https://github.com/gravitational/teleport.e/blob/master/rfd/0001e-device-trust.md)
* [RFD 0008e - Device Trust TPM Support][dt-tpm-rfd]

[dt-tpm-rfd]: https://github.com/gravitational/teleport.e/blob/master/rfd/0008e-device-trust-tpm.md

## Why

The Device Trust TPM RFD, while largely OS-agnostic, was focused on the Windows
implementation at the time of writing. Linux and Windows have distinct user and
permission models, so while implementing TPM support for Linux it became clear
that certain differences in UX need to be addressed.

There is little change in the overall guarantees of the system and its
server-side implementation, unless explicitly noted.

## Details

Three main permissions are necessary for Linux client-side device trust:

1. Permission to open and use the TPM resource manager device (`/dev/tpmrm0`)
2. Permission to read the device information such as model and serial numbers
   (files under `/sys/class/dmi/id/`, or `/dev/mem`, or `dmidecode`)
3. Permission to read the event log (
   `/sys/kernel/security/tpm0/binary_bios_measurements`)

All of these can be acquired via selective use of `sudo` by `tsh`.
Note that sudo-ing tsh itself, as in `sudo tsh ...`, is not considered as an
option: tsh works under the assumption that a non-privileged user is running it,
and thus saves and relies on many files saved against that same user.

In order to maintain a clean UX, sudo usage by tsh is viewed differently by
device trust operations:

* `tsh device enroll`: OK to sudo, as enrollment is a high-value and one-off
  operation.
* `tsh device collect` and `tsh device asset-tag`: OK to sudo, these are
   debug/self-heal commands (see the [device information section](
   #device-information)).
* `tsh login` and similar operations: sudo is to be avoided.

The sections below detail system behavior in the absence of necessary
permissions. If permissions are present, then the required operations will
simply happen without the need for any additional user input.

### TPM device

TPM device access is required by both enrollment and login operations, thus a
non-privileged route must be found. Thankfully, this is an easy permission to
acquire: in many systems assigning the user the `tss` group will do.
Alternatively, equivalent [udev rules](
https://github.com/tpm2-software/tpm2-tss/blob/master/dist/tpm-udev.rules) can
be created by system admins.

This is the only permission that needs to be acquired from outside of Teleport.
To support it, both public documentation and error messages should refer to the
solution. For example:

```shell
$ tsh device enroll
Failed to open the TPM device. Consider assigning the user to the `tss` group or creating equivalent udev rules.
See https://goteleport.com/docs/access-controls/device-trust/device-management/#troubleshooting.
```

### Device information

Device information is collected from sysfs, namely:

* /sys/class/dmi/id/product_name      (aka [model_identifier][])
* /sys/class/dmi/id/chassis_asset_tag (aka [reported_asset_tag][])
* /sys/class/dmi/id/product_serial    (aka [system_serial_number][])
* /sys/class/dmi/id/board_serial      (aka [base_board_serial_number][])

If readable, the files are simply read as the current user. If not readable,
then tsh may elevate itself via `sudo /path/to/tsh device dmi-read`, capturing
stdout and parsing the results. The following framework is used for escalation:

* `tsh device enroll`: sudo is required.
  Collected data is saved under [os.UserCacheDir](
  https://pkg.go.dev/os#UserCacheDir)/.teleport-device/dmi.json for future use.
* `tsh device collect` and `tsh device asset-tag`: sudo is required, data cached
  as above.
* `tsh login` and similar operations: cached data is preferred. If no cached
  data is present, then `sudo /path/to/tsh device dmi-read` is used as a
  fallback, as long as a device key is already present (otherwise nothing
  happens).

`tsh device dmi-read` is a newly-added hidden command that prints data to stdout
in a stable JSON format.

[model_identifier]: https://github.com/gravitational/teleport/blob/33bf6f0a8de9471925581f3af5179139097b45f1/api/proto/teleport/devicetrust/v1/device_collected_data.proto#L53
[reported_asset_tag]: https://github.com/gravitational/teleport/blob/33bf6f0a8de9471925581f3af5179139097b45f1/api/proto/teleport/devicetrust/v1/device_collected_data.proto#L77
[system_serial_number]: https://github.com/gravitational/teleport/blob/33bf6f0a8de9471925581f3af5179139097b45f1/api/proto/teleport/devicetrust/v1/device_collected_data.proto#L81
[base_board_serial_number]: https://github.com/gravitational/teleport/blob/33bf6f0a8de9471925581f3af5179139097b45f1/api/proto/teleport/devicetrust/v1/device_collected_data.proto#L85

### Event log

Currently, the [event log][tpm-plat-attestation] is collected, parsed and stored
for Windows systems, but serves no further use. As such, the simplest solution
is to not collect it for Linux devices. Little is lost here, as interpreting the
event log is [a complex project in itself](
https://github.com/google/go-attestation/blob/master/docs/event-log-disclosure.md).

<!--
Note: simply do `atttest.PlatformAttestConfig.EventLog = []byte{}` to avoid
collecting the event log.

https://pkg.go.dev/github.com/google/go-attestation@v0.5.0/attest#PlatformAttestConfig
 -->

An alternative is to collect the event log during enrollment, since escalation
is required there, but forgo it during device authentication.
At this moment this is not the intended route.

[tpm-plat-attestation]: https://github.com/gravitational/teleport.e/blob/master/rfd/0008e-device-trust-tpm.md#platform-attestation

### Collected data

The entirety of Linux [collected data][dcd] is acquired as follows:

| key | source |
|-|-|
| collect_time              | timestamppb.Now() |
| os_type                   | LINUX |
| serial_number             | firstOf(reported_asset_tag, system_serial_number, base_board_serial_number) |
| model_identifier          | /sys/class/dmi/id/product_name |
| os_id*                    | /etc/os-release ID (eg, "ubuntu") |
| os_version                | /etc/os-release VERSION_ID (eg, "22.04") |
| os_build                  | /etc/os-release VERSION (eg, "22.04.3 LTS (Jammy Jellyfish)" |
| os_username               | user.Current().Name |
| jamf_binary_version       | not collected |
| macos_enrollment_profiles | not collected |
| reported_asset_tag        | /sys/class/dmi/id/chassis_asset_tag |
| system_serial_number      | /sys/class/dmi/id/product_serial |
| base_board_serial_number  | /sys/class/dmi/id/board_serial |
| tpm_platform_attestation  | system managed |

/etc/os-release is typically globally-readable, so no additional measures are
taken to read it.

The DeviceCollectedData.os_id field is added by the RFD to allow Linux distros to be
easily distinguishable.

```diff
message DeviceCollectedData {
  // (...)

+  // OS identifier.
+  // Mainly used to differentiate Linux distros, as there would be no variation
+  // for systems like macOS or Windows.
+  // Example: "ubuntu", "centos", "fedora", "rhel".
+  string os_id = 15;
}
```
[dcd]: https://github.com/gravitational/teleport/blob/33bf6f0a8de9471925581f3af5179139097b45f1/api/proto/teleport/devicetrust/v1/device_collected_data.proto#L29

## Security

Security guarantees remain largely the same as the [TPM RFD][dt-tpm-rfd], the
only change being that Linux devices are allowed to forgo event log collection
during device enrollment and authentication.

## UX

tsh on Linux may now invoke sudo in the following situations:

* During `tsh device enroll`
* During `tsh login` and similar operations, only if the locally saved dmi.json
  file is manually deleted (see the [device information section](
  #device-information))

All sudo invocations are preceded by an explanation. For example:

```shell
$ tsh device enroll --current-device
(...)
Determining machine model and serial number, if prompted please type the sudo password.  <-- printed by tsh
Place your right index finger on the fingerprint reader  <-- printed by the system
(...)
```

## Alternatives considered

### Device information via dmidecode

Using dmidecode for device information is practical, as it is easy to spawn with
sudo. The main downside is the need for an external binary, which is why this
alternative is avoided.

The following data is collected (using `/usr/sbin/dmidecode -s <keyword>`):

* [system_product_name](https://linux.die.net/man/8/dmidecode) (aka [model_identifier][])
* system_serial_number
* baseboard_serial_number
* chassis_asset_tag (aka [reported_asset_tag][])

(Other data points don't require special privileges to collect.)

The same escalation logic is used as for the main [device information
section](#device-information).
