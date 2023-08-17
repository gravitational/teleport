---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 0145 - API Client support for Credential Reload

## Required Approvals

* Engineering: ??
* Product: ??
* Security: @reedloden || @jent

## What

Support reloading credentials without the recreation of the Teleport API client.

This was initially specified in "RFD 0010 - API and Client Libraries":

> Credential loaders should detect underlying file changes and automatically reload credentials and update the grpc transport.

However, this did not make it into the implementation.

## Why

Teleport encourages the use of short-lived certificates, but consuming these
with our API client is difficult. 

In the current state, we rely on consumers to implement their own reload 
mechanism which detects the change to the file and then creates a new client, 
they then must propagate the new client through their application. This is a 
relatively challenging feat for less experienced Go engineers and raises the 
barrier of entry to those who wish to create custom automations.

This not only affects the custom automations, but also our Teleport Access
Plugins.

This issue has raised in prominence with the growth of Machine ID but as
Machine ID produces short-lived certificates it is incompatible with usage
of the API client and Access plugins in their current state.

## Details

One of the challenges with implementing this is the complexity of transport
to the Teleport API - we not only need to reload the TLS credentials which are
presented during the TLS handshake, but also the SSH credentials which are
used in some circumstances to open a tunnel to the Auth Server.

We should also aim to support the rotation of Certificate Authorities.

It may be helpful to reduce the scope of this to focus on Identity Files. This 
simplifies the implementation as:

- Identity Files contain all relevant material within a single file. This
  reduces the likelihood of reloading a partially updated directory of keys,
  certificates and CAs which could result in failure.