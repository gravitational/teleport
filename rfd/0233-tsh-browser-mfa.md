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