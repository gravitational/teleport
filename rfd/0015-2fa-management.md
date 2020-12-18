---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: draft
---

# RFD 15 - 2FA device management

## What

Improvements to 2FA device management (OTP and U2F):
- support for multiple 2FA devices per account
- ability to mix OTP and U2F devices (and possibly more in the future)
- ability to list/add/delete devices by the user from Web UI and CLI

## Why

Currently, Teleport only supports a single OTP or U2F device per local user.
This setting is global to the cluster - either everyone uses OTP or U2F or
nothing. Users don't have a choice, and losing a 2FA device requires an admin
to reset the account to recover.

[RFD 14](0014-session-2FA.md) adds support for 2FA challenges per-connection.
2FA devices will become more widely used by Enterprise users, who would
previously delegate this to their SSO provider.

Teleport needs to make 2FA device support more flexible, as both a
long-standing OSS users' request, and the new Enterprise use-case.

## Details

### protocols

Teleport supports two 2FA protocols: OTP and U2F.

There's no change to the list of supported protocols, but the implementation
should assume that more protocols will be added (e.g. WebAuthn).

For U2F, we will migrate from https://github.com/tstranex/u2f to
https://github.com/flynn/u2f, to allow client-side CLI authentication without
the `u2f-host` dependency.

### UX

For the below examples, we assume that the user already has at least one 2FA
device enrolled. The [bootstrap](#bootstrap) section covers registration of the
first device.

#### CLI

Login:

```sh
$ tsh login --login=non-sso-user
Enter password for Teleport user non-sso-user: ...
Tap your security key... <tap>

$ tsh login --login=non-sso-user --mfa=otp
Enter password for Teleport user non-sso-user: ...
Enter OTP code: ...

$ tsh login --login=sso-user --auth=oidc
# SSO page opens
# no 2FA prompt from Teleport
```

2FA management:

```sh
$ tsh mfa ls
MFA deivice name   Type   ID                                     Last used
----------------   ----   ------------------------------------   -------------------------------
android OTP        OTP    fa004bf4-acc7-435d-8965-5f5a0a4552e8   Tue 15 Dec 2020 01:29:42 PM PST
yubikey            U2F    c8fbb126-3a29-4a9c-bfe9-ebaed31d8585   Wed 16 Dec 2020 02:00:13 PM PST

$ tsh mfa add
Adding a new MFA device.
Choose device type (1 - OTP, 2 - U2F): 2
Enter device name: solokey
Tap any *registered* security key... <tap>
Tap your *new* security key... <tap>
MFA device "solokey" added.

$ tsh mfa ls
MFA deivice name   Type   ID                                     Last used
----------------   ----   ------------------------------------   -------------------------------
android OTP        OTP    fa004bf4-acc7-435d-8965-5f5a0a4552e8   Tue 15 Dec 2020 01:29:42 PM PST
yubikey            U2F    c8fbb126-3a29-4a9c-bfe9-ebaed31d8585   Wed 16 Dec 2020 02:00:13 PM PST
solokey            U2F    87d4fb03-012c-451c-ab0d-5f2a681c119a   Wed 16 Dec 2020 02:05:46 PM PST

# remove by name
$ tsh mfa rm yubikey
Tap any *registered* security key... <tap>
MFA device "yubikey" removed.

# remove by ID
$ tsh mfa rm fa004bf4-acc7-435d-8965-5f5a0a4552e8
Tap any *registered* security key... <tap>
MFA device "android OTP" removed.

$ tsh mfa ls
MFA deivice name   Type   ID                                     Last used
----------------   ----   ------------------------------------   -------------------------------
solokey            U2F    87d4fb03-012c-451c-ab0d-5f2a681c119a   Wed 16 Dec 2020 02:05:46 PM PST

# If 2FA is optional:
$ tsh mfa rm solokey
You are about to remove the only remaining MFA device.
This will disable MFA during login.
Are you sure? (y/N): N

# If 2FA is required:
$ tsh mfa rm solokey
Can't remove the only remaining MFA device.
Please add a replacement MFA device first using "tsh mfa add".
```

#### web UI

Web UI management of 2FA devices should be logically similar to the CLI:
- a page to see all enrolled devices
- a button to enroll a new device
- buttons to remove any enrolled device (except for the last one)

Web UI details, wireframes and implementation will be added later, when we have
the capacity to do it. Initially, 2FA management is CLI-only.

#### bootstrap

Initially, a user account doesn't have a 2FA device. Depending on [cluster
configuration](#configuration), use of 2FA devices might be required.

If 2FA is required, a user is required to enroll a device during account
creation (for local users) or first login (for SSO users).

If 2FA is optional, a user can create an account and login without 2FA. They
can then add 2FA devices as described above. If an existing user has at least
one 2FA device registered, it's required during login.

### configuration

The current 2FA configuration in Teleport only applies to local users and is
always required. We need to allow this to be optional (for users to migrate)
and allow to mix different 2FA methods.

Existing configuration options must be backwards-compatible - no change in
behavior unless config values are changed.

New values for `auth_service.authentication.second_factor` for this:
- `off` (existing) - no 2FA can be enrolled
- `otp` (existing) - only OTP can be enrolled and is required for all local
  users
- `u2f` (existing) - only U2F can be enrolled and is required for all local
  users
- `on` (new) - users can enroll both OTP and U2F devices, and 2FA is required
  for all local users
- `optional` (new) - users can enroll both OTP and U2F devices, and 2FA is
  required only for users with 2FA enrolled
- `session_only` (new) - users can enroll both OTP and U2F devices, and 2FA is
  required only for [sessions](0014-session-2fa.md) but **not** for logins
  - this mode is for users with SSO integration that want 2FA per session, but
    not when logging in because their SSO already performs a 2FA check

### backend storage

Each Teleport `User` object has a `LocalAuth` proto field:

```
message LocalAuthSecrets {
    bytes PasswordHash = 1 [ (gogoproto.jsontag) = "password_hash,omitempty" ];
    string TOTPKey = 2 [ (gogoproto.jsontag) = "totp_key,omitempty" ];
    U2FRegistrationData U2FRegistration = 3 [ (gogoproto.jsontag) = "u2f_registration,omitempty" ];
    uint32 U2FCounter = 4 [ (gogoproto.jsontag) = "u2f_counter,omitempty" ];
}
```

To support multiple 2FA devices, we'll modify it:

```
message LocalAuthSecrets {
    bytes PasswordHash = 1 [ (gogoproto.jsontag) = "password_hash,omitempty" ];

    // Deprecated MFA fields.
    string TOTPKey = 2 [ deprecated = true, (gogoproto.jsontag) = "totp_key,omitempty" ];
    U2FRegistrationData U2FRegistration = 3 [ deprecated = true, (gogoproto.jsontag) = "u2f_registration,omitempty" ];
    uint32 U2FCounter = 4 [ deprecated = true, (gogoproto.jsontag) = "u2f_counter,omitempty" ];

    repeated MFADevice MFA = 5;
}

message MFADevice {
    string ID = 1;
    string Name = 2;
    google.protobuf.Timestamp LastUsed = 3;
    oneof Device {
        TOTPDevice TOTP = 4;
        U2FDevice U2F = 5;
    }
}

message TOTPDevice {
    string Key = 1;
}

message U2FDevice {
    // Copied from U2FRegistrationData
    bytes Raw = 1;
    bytes KeyHandle = 2;
    bytes PubKey = 3;

    uint32 U2FCounter = 4;
}
```

#### migration

The above `LocalAuthSecrets` will be migrated by Teleport at startup in v6.
In v7, we will remove the deprecated MFA fields from `LocalAuthSecrets.`

### audit log

All 2FA device operations (create/delete) will emit an audit log entry. The
entry should contain the user, device UUID and device name at a minimum.

Logging in with a 2FA check will add a `With2FA` field containing the device
UUID to `UserLogin` event.
