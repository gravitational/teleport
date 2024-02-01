---
authors: Mike Jensen (mike.jensen@goteleport.com)
state: draft
---

# Required Approvers
* Engineering @zmb3 && (@codingllama || @nklaassen)
* Product: (@xinding33 || @klizhentas || @russjones)

# RFD 0158 - Account Recovery Protections

## What

This RFD addresses a flaw from a prior RFD [0029-account-lifecycle.md](0029-account-lifecycle.md) when an account is under a targeted attack.  This RFD clarifies how accounts are recovered when they are being targeted to by an attacker trying to prevent authentication.  In order to accomplish this we only need a small change in the product behavior, the removal of the account lock on failed recovery attempts.

## Why

The [Account Lifecycle RFD](0029-account-lifecycle.md) defines how an account can be recovered after the loss of credentials.  However it does not ensure that an account is recoverable under the condition that it is under a targeted attack.

Account authentication can (and should be) locked out after invalid attempts.  Because passwords are user supplied, there are real brute force risks that must be considered.  However this lockout mechanism can also prevent accounts from being accessible by the legitimate user.  We must allow users access to their accounts even under conditions where they are under a targeted attack.

## Details

### Failed Auth Reset Mechanism

In many applications (including Teleport) using a password reset will also reset any potential account auth lockouts.  Because a user may have locked themselves out after failed attempts prior to the reset, this is a very intuitive UX.

### Issues with RFD 0029 - Account Lifecycle

Prior to this RFD the reset mechanism also incorporated a lockout after 3 failures.  This means that if an attacker wants to prevent access for an account they simply need to deny these two mechanisms.  A failed reset lockout combined with a failed authentication lockout means that a legitimate users will have no ability to access their account.

### Fix

Because reset tokens are not user controlled, and neither tokens nor MFA devices are able to be brute forced, this lockout on reset is unnecessary.  We should remove this mechanism so that legitimate users who have control over reset tokens or associated MFA can maintain access even under attack conditions.

### API Changes

The API for retrieving and deleting recovery attempt records will be removed, as they exclusively support the now-obsolete lock mechanism.  Additionally, the user status will no longer include a recovery attempt lock expiration.

### Audit Events

An event is already sent when an account recovery fails.  That auditing will be preserved, no new metrics will be added.

### Security

Upon implementing this change, all accounts previously locked due to account recovery issues will be immediately unlocked, making them available for recovery.

