---
authors: Sasha Klizhentas (sasha@goteleport.com)
state: draft
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
Please save these account recovery tokens in a safe offline place.
You can use each token once if you loose your second factor or a password.

* tele-example-teleport-sh-cosponsor-happening-moocher-undrilled-royal-depravity-frustrate-starting
* tele-example-teleport-sh-factoid-shorten-koala-explore-retinal-lazy-coauthor-brethren
* tele-example-teleport-sh-thinness-suspect-shrink-dwindle-awning-engulf-thrift-spoiling
```

Customers can check the recovery token prefix in case if they forget the domain name.
Teleport cloud scanners can look for `tele-` prefix and alert customers with
domain names in the prefix about leaked tokens.

* [ ] All 3 tokens are regenerated in batch every time account recovery is successful.
* [ ] Each token can only be used one time successfully or unsuccessfully.
* [ ] Teleport should store tokens hashed with `crypto/bcrypt` and compare using constant-time compare.
* [ ] Trying a wrong token 3 times in a row should lead to a temporary account lock following
the same failed account login rate-limiting that exists for local users right now.
* [ ] The reset endpoints should be rate-limited to prevent brute-force scans.
* [ ] UI should make it clear that these tokens are important and if lost, the account can not be recovered.

### Recovery scenarios

The first two recovery scenarios work for every system user, reducing the load
on Teleport system administrators as well.

**Emails and usernames**

Teleport Cloud assumes that username is a user email.
When sending emails, Teleport cloud will send an email to the username if it contains @ sign.
Self hosted Teleport will not send any emails.

**Account owners**

Teleport considers any user with delete `account` resource permission
as account owner. This doc refers to those users as account owner users.

The `account` resource is a new resource that only allows to request
account cancellation.

We will recommend to have several account owners, users who do not have system administration
privileges and can't modify roles, or users.

Any other users are referred to as simply users.

**User lost a second factor**

If any user lost a second factor, but not password, they can add a new second factor using
a recovery token:

* Login with a username and password.
* Instead of pressing second factor, enter a one-time recovery token.
* Enroll new second factor.
* All recovery tokens are no longer valid. The new set of recovery tokens are printed to the user.
* User has to confirm that they have copied recovery tokens.
* The session is not initiated. A user is redirected to a login form
with UI displaying a message to login using new credentials.
* If the operation is aborted after user enters the token, but fails to complete recovery,
the used token is no longer valid, but remaining tokens can be used.
* If a user tries to reuse the token, it is rejected.

This flow ensures two factors are verified: the valid password and a recovery token.

**User lost a password**

If user lost a password, but not a second factor, they could recover using
a second factor and a recovery token to reset it:

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

**User has lost recovery tokens**

If any user is logged into Teleport UI, they can regenerate new password recovery tokens:

* Alice goes to her account settings and chooses to generate new set of recovery tokens.
* Teleport asks for a second factor before generating tokens and makes it clear that once
selected, old sets of tokens will not be valid, they will be printed once and user
should be prepared to store them in offline place.
* A new set of tokens is printed for Alice. The message makes it clear that once closed,
the tokens are no longer going to be revealed and that any previous tokens are invalid.

**Internal request for canceling the account**

To request account cancellation, any account owner should:

* Be logged into Teleport UI.
* Press a "cancel my account" button in the UI.
* UI should verify that they own a second factor.

```
We have marked your account as inactive. In 10 days, we are going to delete the account and all it's data.
To prevent account from being deleted, go to `My Account` and activate `Revert account cancellation`.
```

The account enters inactive state:

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

If there are no other owners, other active users will notice the message of the day in the UI and CLI.

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
* They click the link and we mark the account as "inactive".

The account enters inactive state described above.

**One owner warning**

For cloud, if there is only one account owner, display a system
notification that is active until the second account owner is added:

```
Your account has only one owner.

To improve security and prevent account loss please add a second account owner.

Please check documentation: https://goteleport.com/docs/cloud/owners for more details.
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
