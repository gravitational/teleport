---
authors: Lisa Kim (lisa@goteleport.com)
state: implemented
---

# RFD 0111 - Support connection testers when per-session MFA is enabled

## What

Add a [MFAAuthenticateResponse](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/api/proto/teleport/legacy/client/proto/authservice.proto#L1089) field to [GenerateUserCerts](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/api/proto/teleport/legacy/client/proto/authservice.proto#L2259) request.

### Related issues

- [#16702](https://github.com/gravitational/teleport/issues/16702)

## Why

As mentioned in the related issue, when a role or config has enabled the [require_session_mfa](https://goteleport.com/docs/access-controls/guides/per-session-mfa) field, users were not able to proceed testing connections to their newly added resource in the web UI, because we didn't implement a way for users to provide and authenticate their MFA device.

## Details

The [Test Connection](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/lib/client/conntest/connection_tester.go#L30) feature requires establishing a brief session with the target resource which requires generating a short lived user certificate. If the `require_mfa_session` is enabled, the certs [mfaVerified](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/lib/auth/auth.go#L1123) field must be set.

Upon testing, the [mfaVerified](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/lib/auth/auth.go#L1123) field could potentially be set to any string value (and still be qualified as verified), so it's important how we set this field. By accepting a [MFAAuthenticateResponse](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/api/proto/teleport/legacy/client/proto/authservice.proto#L1089), the [GenerateUserCerts](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/api/proto/teleport/legacy/client/proto/authservice.proto#L2259) request will be responsible for validating the response (if provided), and upon success will capture the verified MFA device ID which will be used to set the `mfaVerified` field. If validation failed, the request will return an authentication error.

## How it relates to web UI

In the web UI, when a user clicks on the `test connection` button, we will make a call to this existing endpoint [IsMFARequired](https://github.com/gravitational/teleport/blob/d94fed7b0dd6098affa2101e7ab775b173ba612f/api/proto/teleport/legacy/client/proto/authservice.proto#L2266) that checks whether MFA is required to access the specified resource.

Then depending on the response:

- If MFA wasn't required, proceed to make a request to test connection as we did before
- If MFA is required, we will ask the user to enter their MFA credentials, take the response and send it off with the request to test connection
