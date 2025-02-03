---
authors: Noah Stride (noah.stride@goteleport.com)
state: Draft 
---

# RFD 00200 - Certificate-attribute-less HTTP Application Access Routing 

## Required approvers

- Engineering: @zmb3
- Product: @klizhentas || @xinding33
- Security: @reedloden || @jentfoo

## What

Following RFD0185, which adjusted Kubernetes Access to route without encoding
the intended route into the client certificate, the same will now be applied to
application access.

## Why

Today, a Machine ID output can only generate a client certificate for a single
application that has been named ahead of time. This has proved problematic for a
number of use-cases where the user wishes to authenticate a machine to connect
to many applications clusters. At smaller scales, it is not a problem to 
configure an output per application, but at larger scales, this becomes a
problem.

The root of this behaviour in `tbot` comes from the fact that a distinct
client certificate must be issued for each application that you wish to connect
to. This is because an attribute is encoded within the certificate to
assist with routing.

The need to issue a certificate per cluster becomes a problem when wishing to
connect to a large number of applications for the following reasons:

- Increased pressure on the Auth Server. Certificate signing is an expensive
  operation and requesting the signing of hundreds of certificates within a
  short period of time threatens to overwhelm the Auth Server.
- Less flexibility. If a short-lived application is spun up, then a new
  certificate must be issued. This means that `tbot` must be
  reconfigured or re-invoked.
- Increased pressure on `tbot`. If `tbot` were to issue certificates for a large
  number of applications, then it would require a larger amount of resources to
  make the increased number of requests.
- Decreased reliability. The more certificates that must be issued, the more
  likely that one of the set will fail for some ephemeral reason.

## Details

### Today

Today, we encode the intended application within the user X.509 certificate
using three key attributes:

- AppClusterName (1.3.9999.1.5)
- AppPublicAddr (1.3.9999.1.6)
- AppName (1.3.9999.1.10)

These attributes are used by the Proxy and Application Access agent to determine
where to route the requests.

The target Application Access agent performs authorization checks based on the
user's identity (e.g roleset, attributes) rather than relying on these
attributes.

SessionID is automatically encoded by auth server - how do we get around needing
this. Special certs generated for Proxy to forward to app access agent tied to
App Accesss./