---
authors: Dan Share (daniel.share@goteleport.com)
state: draft
---

# RFD 233 - `tsh` Browser MFA

## Required Approvers

* Engineering: @zmb3
* Security:
* Product:

## What

This RFD proposes a new method for users of `tsh` to be able to authenticate
themselves using their browser-based MFA.

## Why

We encourage our users to use the strongest methods of MFA when signing up for
an account through the web UI, such as passkeys and hardware keys. However, If
a user only provides a passkey as their MFA method, they won't be able to
authenticate when using `tsh`. 

This RFD aims to describe how we can allow `tsh` to delegate its MFA checks to
the web UI to enable easier access to biometrics and passkeys from both browsers
and password managers. We will also be one step closer to the ultimate goal of
removing support for TOTP in Teleport.

## Details

### UX

#### User stories

**Alice logs in to their cluster using `tsh`**

Alice is a new user who has created her account with a passkey as her second
factor. She would like to log in to her cluster using `tsh`. She runs the
following command:

```
tsh login --proxy teleport.example.com --user alice
```

She is asked for her password, which is then sent to Teleport. Teleport verifies
her username and password, and checks for valid methods of second factor
authentication. It finds her passkey returns a URL which `tsh` opens in the
default browser for her to complete the challenge. If a browser cannot be opened
the URL is printed out for her to click.

The browser will open to a page that contains a modal prompting her to verify it
is her by completing the MFA check. Once this is completed, the browser will
redirect back to `tsh`.

Alice is now authenticated and able to interact with resources on that cluster.

**Alice connects to a resource that requires per-session MFA**

Alice is already authenticated with her cluster, but wants to access a resource
that requires per-session MFA. She runs the following command:

```
tsh ssh alice@node
```

She is then redirected to the browser, or given a URL, to authenticate with her
MFA. Upon success, she is redirected back and the ssh session can continue.

### Design

### Security

### Scale

### Backward Compatiblity

### Test Plan