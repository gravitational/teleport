---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0127 - Resource versioning

## Required Approvers

- Engineering: @r0mant && @zmb3

## What

To define best practices around updating resources and choosing versions for them.

## Why

So far, all resources have used a single increasing version number (generally `v2` or `v3` with legacy `v1` resources not defined in protobuf form but referring to some older bespoke format that's no longer in use or supported, and some exception like `network_restrictions` having jumped from `v1` directly to `v4`, or `role` supporting up to `v6`), and the ProtoBuf message types used to store and handle resources have been renamed over and over to follow the highest supported version, causing unnecessary churn and making it impossible (or at least very hard) to run certain classes of automatic checks for backwards and forwards compatibility (which is what ProtoBuf would otherwise excel at). This RFD proposes a change in how we update resources, change versions, and define the underlying types.

## Details

A Teleport resource contains, at a minimum, a `kind` defining what the resource is (like `role` or `web_session` or `kube_server`) and a `version`, which is a string that's currently always one of `v1`, `v2`, `v3`, `v4`, `v5`, `v6` (but is actually a completely opaque identifier) and that is used to allow for changes in format or semantics across different versions of Teleport, to deliberately prevent the misinterpretation of a resource that might end up being accidentally compatible in terms of how the data is serialized. This is particularly relevant for resources such as `role`, whose semantics MUST be fully understood for a cluster component to be able to make access control decisions, lest they accidentally grant more access than intended.

The flexibility of JSON as a storage format and ProtoBuf as an exchange format (as part of gRPC) lets us reuse essentially the same code to support different versions at once in almost all cases; this RFD proposes the following:

- if fields in a resource are added, discarded or changed in such a way that older versions of Teleport will still behave "correctly enough" when interpreting the new resource data as the old one, the version doesn't need to change. This can and should be done deliberately whenever possible; for instance, a new field with broader semantics can be replicated onto an older, deprecated field, so that older versions of Teleport can still understand and use the data - see [#27018](https://github.com/gravitational/teleport/pull/27018) for an example of this in action.

- if the format of the data stays compatible (in terms of JSON and ProtoBuf unmarshaling) but the meaning of the data changes enough that we want older versions of teleport to refuse parsing it, we must bump the version string; if the latest version of `role` is `v6`, such a change would introduce a `role` with version `v7`. It's advisable to document which fields became available in which resource version, and to have checks in place so that resources that are marked to have a version don't actually end up specifying behavior or options that only became available in later versions - for instance, it would be advisable to reject any `v6` `role` that sets a field that was only introduced in `v7` (as that same role resource would be interpreted incorrectly by an older Teleport agent that only understood up to `v6`).

  Since it offers no real benefit, we should not rename the underlying message type (from `RoleV6` to `RoleV7`, for instance); doc comments on the message struct will suffice to document which versions are handled by which concrete type (this is already true for the oldest supported version for a type). Existing resource types should keep their name, but we should expect brand new resources to be handled by the `FooV1` type almost all the time.

- if the format of the data requires a total compatibility break in either the JSON or ProtoBuf serialization, a new version of the resource should be introduced, with a new ProtoBuf message definition (together with a set of new RPCs using the resource). The new type should take the name `FooVX` where `X` is the new version that should be handled by the new type; `FooV1` (or whatever the previous name was) is going to be used for versions `1` through `X-1`, and can be removed after appropriate deprecation notices, automatic migrations and time. This should be avoided in all but the most extreme situations.

When bumping the version of a resource it should be kept in mind that older cluster components and clients will not be able to handle the new version. Depending on which features the new resource is intended to support, this can be ok - but we should write code in such a way that the breakage is kept to a minimum when such a thing happens, for instance by skipping resources that have an unknown version in listings whenever appropriate - but for more pervasive things such as roles, it's also possible to downgrade the entire resource on the fly based on the version of the client asking for it. This is only possible when the intent behind the newer version of the resource can be expressed as an older version of the resource, or when this transformation results in deliberate disruption with less impact than what would happen with a failure to understand the new ; for instance, if a newer version of `role` adds extra restrictions to access, it can be downgraded to an older version by replacing the entire role with some appropriately scoped deny rules, to still grant some access to users bearing that role (a role that fails to be parsed because it's of an unsupported version is effectively a blanket deny, which prevents _all_ access).

It's advisable to signal that such a downgrade has happened in some way, for instance [by adding an internal label](https://github.com/gravitational/teleport/pull/27244) to the resource being downgraded.
