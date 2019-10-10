## Overview

TODO: This doc is in-progress not at reviewable stage

This is a conceptual doc about the Teleport Users. This doc will explain what users are and how they related to OS-level users and externally-configured users.

[TOC]

## Types of Users

Teleport supports two types of user accounts: **Local Users** and **External Users**

### Local users

Local users are created and stored in Teleport's own identity storage. A cluster
administrator has to create account entries for every Teleport user.
Teleport supports second factor authentication (2FA) and it is enforced by default.
There are two types of 2FA supported:
* [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
  is the default. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator) or
  [Authy](https://www.authy.com/) or any other TOTP client.
* [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor).

### External users

External users are users stored elsewhere within an organization. Examples include
Github, Active Directory (AD) or any identity store with an OpenID/OAuth2 or SAML endpoint.

!!! tip "Version Warning":
    SAML, OIDC and Active Directory are only supported in Teleport Enterprise. Please
    take a look at the [Teleport Enterprise](enterprise.md) chapter for more information.

It is possible to have multiple identity sources configured for a Teleport cluster. In this
case, an identity source (called a "connector") will have to be passed to `tsh --auth=connector_name login`.
Local (aka, internal) users connector can be specified via `tsh --auth=local login`.

## User Mappings

Every Teleport User must be associated with a list of machine-level OS usernames it can authenticate as during a login. This list is called "user mappings"

Unlike traditional SSH, Teleport introduces the concept of a User Account. A User Account is not the same as SSH login. For example, there can be a Teleport user "johndoe" who can be given permission to login as "root" to a specific subset of nodes.

## User Roles

Unlike traditional SSH, each Teleport user account is assigned a `role`.
   Having roles allows Teleport to implement role-based access control (RBAC), i.e. assign
   users to groups (roles) and restrict each role to a subset of actions on a subset of
   nodes in a cluster.


