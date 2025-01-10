---
authors: Sasha Klizhentas (sasha@goteleport.com)
state: implemented
---

# RFD 29 - Account Life-cycle: Recovery and Cancellation

## What

Introduces cloud account recovery and cancellation flow.

## Why

Having a secure self-management account recovery and cancellation
flow improves user experience and reduces support load on teams.

## Details

The recovery flow focuses on local accounts.
SSO account recovery is covered by "All is lost" scenario.

The flow will work for cloud and non-cloud accounts. For cloud it should be impossible
to turn off second factor. For self-hosted, if second factor is off, the account recovery
should not be possible.

Each local user will receive 3 account recovery tokens on sign up.
Each token is a crypto-randomly generated passphrase string that could be used only once.
We will use the vendored EFF long list via diceware library

https://github.com/sethvargo/go-diceware

The token is 8 word token resulting in roughly [~100 bits of entropy](https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases).

Once signed up or invited, users will register with their username, password and second factor.
Then users will be presented with the recovery tokens displayed on the screen with a message:

```
Recovery tokens generated on January 15th, 2020.

Please save these account recovery tokens for example.teleport.sh in a safe offline place.

You can use each token once if you lose your second factor or a password.

* tele-cosponsor-happening-moocher-undrilled-royal-depravity-frustrate-starting
* tele-factoid-shorten-koala-explore-retinal-lazy-coauthor-brethren
* tele-thinness-suspect-shrink-dwindle-awning-engulf-thrift-spoiling
```

The user interface should present an option to download and print the tokens on the same screen
to make it more convenient for a user to print the tokens.

Teleport cloud scanners can look for `tele-` prefix and alert customers with
domain names in the prefix about leaked tokens.

* [ ] All 3 tokens are regenerated in batch every time account recovery is successful.
* [ ] Each token can only be used one time successfully or unsuccessfully.
* [ ] Teleport should store tokens hashed with `crypto/bcrypt` and compare using constant-time compare.
Teleport should store the time when the tokens were generated alongside the hashed tokens.
* [ ] The reset endpoints should be rate-limited by account and IP to prevent brute-force scans.
* [ ] UI should make it clear that these tokens are important and if lost, the account can not be recovered.
* [ ] Set the following headers on the web page presenting the tokens:

```
Set:
Cache-Control: no-cache, no-store, max-age=0, must-revalidate
Pragma: no-cache
Expires: Mon, 01 Jan 1990 00:00:00 GMT
```
* [ ] Account lock should result in email notification to account owners and the user who is locked.

A locked user should get an email:

```
An account with email user@example.com has tried to recover the account unsuccessfully and was locked.
This account can not be recovered. Please request the system administrator to re-create the account.
```

Account owners should receive the email:

```
An account with email user@example.com has tried to recover the account unsuccessfully and was locked.
This account can not be recovered. Verify that the user initiated the account recovery,
delete the account and invite the user again.
```

### Recovery scenarios

The first two recovery scenarios work for every system user, reducing the load
on Teleport system administrators as well.

**Emails and usernames**

Teleport Cloud assumes that username is a user email.
When sending emails, Teleport cloud will send an email to the username if it contains @ sign.
Self hosted Teleport will not send any emails.

**Account owners**

Account owner is any user who can request access to account editor role
and review such requests.

Review [Access requests with dual authorization](https://goteleport.com/docs/access-controls/guides/dual-authz/)
for more details on access requests to understand this RFD better.

Teleport will create `account-owner` and `account-editor` role presets.

A sample `account-owner` role preset:

```yaml
allow:
  request:
    roles: ['account-editor']
  review_requests:
    roles: ['account-editor']
```

Teleport's access requests by do not allow any user to review their own requests, so the
role above is secure.

Sample `account-editor` role preset:

```yaml
allow:
  rules:
    - resources: ['account']
      verbs: ['update', 'delete']
```


We will recommend to have several account owners, users who do not have system administration
privileges and can't modify roles, or users.

Any other users are referred to as simply users.

**Locksmiths**

Locksmith is any user who can request access to lock-editor role
and review such requests.

This feature relies on not-implemented yet [session and user locks](https://github.com/gravitational/teleport/issues/3360).
We will assume that `lock` is a new resource that can place a lock on account or a set of users.

Teleport cloud will create a `locksmith` and and `lock-editor` role presets.

A sample `locksmith` role:

```yaml
allow:
  request:
    roles: ['lock-editor']
  review_requests:
    roles: ['lock-editor']
```

Teleport's access requests by do not allow any user to review their own requests, so the
role above is secure.

Sample `lock-editor` role:

```yaml
allow:
  rules:
    - resources: ['lock']
      verbs: ['create', 'list', 'read', 'update', 'delete']
```

We will recommend to have several `locksmiths`, users who do not have system administration
privileges and can't modify roles, or users.

Any other users are referred to as simply users.

**User lost a second factor**

If any user lost a second factor, but not password, they can add a new second factor using
a recovery token.

* Request second factor recovery on the login form.
* Teleport Cloud generates a recovery link and emails this link to the user's email.

```
This is Teleport cloud account recovery link initiated <date> from <device> and <location>.
If this activity was not imitated by you, please contact your system administrator.
Otherwise, follow the <link> to proceed with account recovery.
```

* The link uses a crypto-random 32 byte hex that activates account recovery flow.
* User has to login with a username and password.
* Instead of pressing second factor, enter a one-time recovery token.
* Enroll new second factor.
* All recovery tokens are no longer valid. The new set of recovery tokens are printed to the user.
* User has to confirm that they have copied recovery tokens.
* The session is not initiated. A user is redirected to a login form
with UI displaying a message to login using new credentials.
* If the operation is aborted after user enters the token, but fails to complete recovery,
the used token is no longer valid, but remaining tokens can be used.
* If a user tries to reuse the token, it is rejected.
* All second factors set up prior to the reset are removed from the system.

This flow ensures two factors are verified: the valid password and a recovery token.
Teleport cloud sends a recovery link to an email address to protect from a physical compromise -
someone stealing the laptop and the printed tokens.

Recovery triggers an email notification to a user:

```
Your Teleport Cloud account has been successfully recovered on <date> from <device> and <location>.
If this activity was not imitated by you, please contact your system administrator. Otherwise,
no further action is necessary.
```

Account owners should get an email:

```
Teleport Cloud account for user@example has been successfully recovered on <date> from <device> and <location>.
No further action is necessary.
```

**User lost a password**

If user lost a password, but not a second factor, they could recover using
a second factor and a recovery token to reset it:

* Request second factor recovery on the login form.
* Teleport Cloud generates a recovery link and emails this link to the user's email.
* The link uses a crypto-random 32 byte hex that activates account recovery flow.
* User enters their username and clicks forgot password.
* User has to present a valid second factor and a recovery token,
after that they will be able to reset their password.
* All recovery tokens are no longer valid. The new set of recovery tokens are printed to the user.
* User has to confirm that they have copied recovery tokens.
* The session is not initiated. A user is redirected to a login form
with UI displaying a message to login using new credentials.

This flow ensure that two factors are verified:

* Second factor TOTP token or U2F device.
* A valid account recovery token.
* Email step protects from a physical compromise and bad actor stealing the recovery tokens and a computer.

Recovery triggers an email notification to a user:

```
Your Teleport Cloud account has been successfully recovered on <date> from <device> and <location>.
If this activity was not imitated by you, please contact your system administrator. Otherwise,
no further action is necessary.
```

Account owners should get an email:

```
Teleport Cloud account for user@example has been successfully recovered on <date> from <device> and <location>.
No further action is necessary.
```

**Authorizing privileged operations**

To confirm some privileged operations like account cancellation or recovery tokens
regeneration Teleport has to confirm that user is near their computer and actively authorizes the action.

If a user has logged in the last 5 minutes, Teleport asks for any valid second factor
token authorization.

If the login hasn't been done in the last 5 mins, Teleport asks Alice to re-login completely -
by asking for password and second factor. This is done to step up the verification on a period of inactivity
before executing privileged operation.

We will refer to this flow as *Authorizing privileged operations* flow.

**User has lost recovery tokens**

If any user is logged into Teleport UI, they can regenerate new password recovery tokens:

* Alice goes to her account settings and chooses to generate a new set of recovery tokens.
* Teleport asks for a second factor using *Authorizing privileged operations flow* before generating tokens and makes it clear that once
selected, old sets of tokens will not be valid, they will be printed once and Alice
should be prepared to store them in offline place.
* A new set of tokens is printed for Alice. The message makes it clear that once closed,
the tokens are no longer going to be revealed and that any previous tokens are invalid.

**Internal request for canceling the account**

Let's say Bob would like to cancel the account. Bob should first

* Have a role that allows to request `account: delete` permission.
* Log into Teleport UI.
* Navigate to account details page and request account cancellation.
* Teleport asks Bob prove the ownership of a valid second factor using *Authorizing privileged operations* flow.
* All users who have `locksmith` role except Bob, who initiated the lock request receive an email or chat bot notification:

```
User <email> has initiated account cancellation on <time> at <device> <location>.

Here are the lock details:

* <lock description>

To proceed with initiation, log-in to Teleport UI and approve the operation.
Otherwise, do nothing and the request will expire in 1 hour.
```

Behind the scenes, Teleport creates access request to get the role `account-editor` with TTL of 1 hour.

A second account owner, Alice, has to log into Teleport to approve/deny the operation.

* Alice logs into Teleport and approves the operation using existing access request UI or CLI.
  + If Alice denies the operation Teleport uses existing `DENY` operation on access requests.
* Teleport asks to prove Alice's ownership of a valid second factor using *Authorizing privileged operations* flow.
* Bob receives the notification that request was approved or denied and has to re-login in to Teleport UI to assume
  role or use the "Assume role" existing user interface.
* Bob can press "cancel" using *Authorizing privileged operations* flow.

Teleport sends an email:

```
We have marked your account as inactive. In 10 days, we are going to delete the account and all it's data.
To prevent account from being deleted, go to `My Account` and activate `Revert account cancellation`.
```

The account enters inactive state.

**Inactive account state**

* Teleport should not disconnect any resources connected to the cloud account during the grace period.
Otherwise it is possible for attacker overtaking an account owner to cause a system-wide outage.

* Any account owner can login into the account and is greeted with a warning:

```
Your account was marked as inactive at the request of `email`.
To prevent account from being deleted, go to `My Account` and activate `Revert account cancellation`.
```

All other account owners should receive the email with the message:

```
Your account was marked as inactive at the request of `email`.
To prevent account from being deleted, login into teleport, go to `My Account` and activate `Revert account cancellation`.
```

This email notification should be sent every 3 days to all account owners until the account is deleted.

Any account owner should be able to revert account cancellation.

Teleport will ask a logged in account owner for a second factor proof to revert account cancellation to prevent
a possible account overtake by a malicious actor who wishes to keep the account for accessing
the resources.

Once system leaves inactive account state, all account owners
should receive an email:

```
Your account was marked as active at the request of `email`.
```

The possible vectors of attack is for malicious actor to overtake someone's
account with a request to delete all the data. The grace period and active privileged
user should mitigate against that. Email sent to other account owners
should alert other owners about account cancellation.

Frequently, when users are evaluating the cloud, there are no other account owners.
In this case Teleport cloud will approve the access request. This creates a delete user attack vector -
when one account owner can delete another account owner and self-approve. That's why to separate
privileges and prohibit account owners to be user and account editors.

Teleport will encourage to set up two account owners using notifications and separate privileges.
See the sections below.

Even in the case of the delete user attack, all other active users will notice the message of the day in the UI and CLI.

Teleport cloud support should be able to delay account cancellation by 10 days if they see a zendesk
issue from any zendesk account.

**When all is lost**

This flow is only relevant for Cloud.

If any user or account owner lost their password and second factor, they won't be able to recover
their account with us, but can request cancellation. We cannot trust account recovery
using email verification alone.

Account owners should:

* Request account cancellation by clicking on "cancel my account link".
* We email crypto random 32 byte hex cancel account link to their email address.
  See requirements for security token URLs below.
* In case if there are more than 2 active account owners, the second account owner should approve the
  request as described below.
* In case if there is only one account owner, Teleport approves access request
  and sets account to inactive state.

The account enters inactive state described above.

**Security tokens URLs**

Apply web security mechanisms to all security tokens referenced in this RFD:

* No caching.
* No referrer leak.
* One time use.
* One valid token at a time.

**Account locks**

There is a possibility of bad actor taking over the account and performing some malicious actions on it.
The first mitigation of any incident response would be to cease all operations except critical ones on the account.

The locked account state is independent from inactive account state and users
can activate and de-activate locks independently from requesting to cancel the account.

In this state, authorized user will put locks on all users except selected exception users.
By session lock refer to a [session lock](https://github.com/gravitational/teleport/issues/3360) feature
that is not implemented yet.

To implement dual authorization this feature will rely on existing [Access requests with dual authorization](https://goteleport.com/docs/access-controls/guides/dual-authz/).
We will use `locksmith` roles described above.

Taylor would like to lock the account. Taylor should:

* Be a member of the role `locksmith` (or a similar role that can request access to any role granting privileges
to create roles).
* Taylor navigates to the UI and activates "Lock this account" flow.
* The UI should allow a user to specify whether this lock is account global, locks out certain users
  and specify exceptions to users who are not locked.
* Teleport verifies second factor using Teleport asks for a second factor using *Authorizing privileged operations flow*.
* Teleport creates an access request for a user requesting a locksmith role.

* All locksmith users except Taylor receive an email or chat bot notification:

```
User <email> has initiated lock on <time> at <device> <location>.

Here are the lock details:

* <lock description>

To proceed with initiation, log-in to Teleport UI and confirm the lock.
Otherwise, do nothing and the request will expire in 1 hour.
```

Bob, a second locksmith logs into Teleport and reviews the request using existing access requests flow.

* A second user with `locksmith` or similar role logs in to account and confirms the lock.
* Teleport verifies second factor using Teleport asks for a second factor using *Authorizing privileged operations flow*.

As soon as request is granted, Taylor assumes the role using `Access requests` UI or CLI.

* Logs in the UI, presses "confirm lock" gets their request executed.

Teleport sends an email:

```
User <email> and <email> have initiated lock at <time> at <devices> <locations>.

* <Lock details>

To remove the lock, login to the locks UI and unfreeze the account.
```

To remove the lock, users should follow the same procedure, but in reverse order:

* If the lock permission is granted to the user, it can be used right away:
* A user can proceed to remove the lock in the UI after confirming their second factor using *Authorizing privileged operations flow*.
* If the privilege has expired, a user has to request privilege using access requests UI.

**Warnings**

Cloud will display several warnings and notifications to users to suggest
best practices and prevent misconfigurations.

To avoid scaring off account evaluations start warnings after 10 days
of active usage of the account.

* If there is only one account owner, display a system notification that is active until
the second account owner is added:

```
Teleport cloud account <name> has only one owner.

To improve security and prevent account loss please add a second account owner.

Please check documentation: https://goteleport.com/docs/cloud/owners for more details.
```

* If there is only one account locksmith, display
notification that is active until the second account owner is added:

```
Your account has only one locksmith - a user who can lock the account.

To improve security and prevent account loss please add a second account locksmith.

Please check documentation: https://goteleport.com/docs/cloud/locksmiths for more details.
```

* If account owner or locksmith can edit or create roles and users, display notification:

```
A user <username> can lock the account or cancel it, but has too many privileges and can modify roles
to override protections.

To improve security and prevent account loss please add a second account locksmith.

Please check documentation: https://goteleport.com/docs/cloud/least-privilege for more details.
```

**Audit events**

Emit audit events for recovery.token life-cycle:

* Recovery tokens regenerated
* Recovery tokens used successfully and unsuccessfully

**Admin reset**

When admin uses reset command: `tctl users reset sam@example.com`,
all second factors, passwords and recovery tokens are deleted.

Reset is an alias for `tctl users rm ..` and `tctl users add`

### Flows

[GitHub account recovery](https://docs.github.com/en/github/authenticating-to-github/recovering-your-account-if-you-lose-your-2fa-credentials)
