---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0127 - Resource versioning

## Required Approvers

- Engineering: TBD ("@r0mant || @zmb3 || @russjones"?)

## What

To define best practices around updating resources and choosing versions for them.

## Why

So far, all resources have used a single increasing version number (generally `v2` or `v3` with legacy `v1` resources not defined in protobuf form but referring to some older bespoke format that's no longer in use or supported - but with exceptions like `network_restrictions` having jumped from `v1` directly to `v4`, or `role` supporting up to `v6`). That, combined with the custom of renaming the message type used to store it, leads to frequent renaming of struct types across the entire codebase for no benefit other than to be able to know at a glance that the `FooV6` struct is intended to hold `foo` resources of version up to `v6` (but with no idea of how low of a version is actually supported). This RFD proposes a more useful versioning scheme with fewer tradeoffs.

## Details

A Teleport resource contains, at a minimum, a `kind` defining what the resource is (like `role` or `web_session` or `kube_server`) and a `version`, which is a string that's currently always one of `v1`, `v2`, `v3`, `v4`, `v5`, `v6` (but is actually treated as a completely opaque identifier) and that is used to allow for changes in format or semantics across different versions of Teleport, to deliberately prevent the misinterpretation of a resource that might end up being accidentally compatible in terms of data. This is particularly relevant for resources such as `role`, whose semantics MUST be fully understood for a cluster component to be able to make access control decisions, lest they accidentally grant more access than intended.

The flexibility of JSON as a storage format and ProtoBuf as an exchange format (as part of gRPC) lets us reuse essentially the same code to support different versions at once in almost all cases; this RFD proposes that we encode that as part of the `version` string of the resource, as such:

- if fields are added, discarded or changed in such a way that older versions of Teleport will still behave "correctly enough" when interpreting the new resource data as the old one, the version doesn't need to change. This can and should be done deliberately whenever possible; for instance, a new field with broader semantics can be replicated onto an older, deprecated field, so that older versions of Teleport can still understand and use the data - see [#27018](https://github.com/gravitational/teleport/pull/27018) for an example of this in action.

- if the format of the data stays compatible (in terms of JSON and ProtoBuf unmarshaling) but the meaning of the data changes enough that we want older versions of teleport to refuse parsing it, we bump the "minor" component of the version string; if the latest version of `role` is `v6`, such a change would introduce a `role` with version `v6.1`. This is not intended to be [Semantic Versioning](https://semver.org/), as there shouldn't be any need for interoperability outside of Teleport in the first place, so we are going to have a list of all the supported versions anyway.

- if the format of the data requires a total compatibility break in either the JSON or ProtoBuf serialization, a new "major" version of the resource should be introduced, with a new ProtoBuf message definition (together with a set of new RPCs using the resource).

### Rejected alternatives

Instead of relying on the version string, we could introduce a `sub_version` or `revision` field; the problem with that is that for each resource that needs to start using it, we are going to need a new _major_ version anyway, since older versions of Teleport are not going to know about the new field, and the `version` field would still need to be a string for older versions of teleport to correctly report a mismatch in supported versions rather than a deserialization error.
