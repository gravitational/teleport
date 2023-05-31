---
authors: Michael Wilson (michael.wilson@goteleport.com)
state: draft
---

# RFD 129 - Gogo Must Go

### Required Approvers

* Engineering @r0mant, @justinas
* Security @reed
* Product: @klizhentas

## What

Migrate Teleport away from the unmaintained gogo protobuf implementation.

## Why

When Teleport was initially implemented, `gogo/protobuf` was a reasonable choice for Teleport's
gRPC support, as it was up to date, performant, and well maintained. However, since then, gogo has
been deprecated and there's no clear path to update to one of the more recent protobuf
implementations.

Furthermore, we've come to rely on several features of gogo that are not replicated in other
protobuf implementations. In particular, those features are:

- custom types that allow for specifying custom types during code generation.
- JSON tags that allow for explicitly labeled JSON tags when marshaling/unmarshaling protobuf
  messages.

As we continue to lag the more modern releases of protobuf behind, we run the risk of falling prey
to vulnerabilities with no easy mitigation strategy, and the burden of migration grows with
additional messages and features that we add.

## Details

### Case studies

The following are case studies of other projects which have migrated away from `gogo/protobuf` and
related tooling that could be useful for our migration.
not go into thorough detail about each, but address some takeaways that I view as useful.

#### containerd's `gogo/protobuf` migration

[Relevant blog post](https://thoughts.8-p.info/en/containerd-gogo-migration.html)

containerd migrated away from `gogo/protobuf` starting in 2021 and finishing in April 2022.

**Important Takeaways**:

- Removing all `gogoproto` extensions ahead of time made the final migration significantly easier.

#### Thanos's `gogo/protobuf` migration

[Relevant blog post](https://giedrius.blog/2022/02/04/things-learned-from-trying-to-migrate-to-protobuf-v2-api-from-gogoprotobuf-so-far/)

This contains some general advice, but nothing particularly applicable in terms of specifics.

#### CrowdStrike's `csproto` library

https://github.com/CrowdStrike/csproto

CrowdStrike used `gogo/protobuf` to overcome various problems with Google's original protobuf
library and subsequently ran into issues during `gogo/protobuf`'s deprecated. `csproto`
is a library that elects to use the proper protobuf runtime for the appropriate message.
As a result, you could have an intermingling of different protobuf libraries without compatibility
issues.

### Our complications

Teleport has a number of complications tied in with our use of `gogo/protobuf` that will make
our migration unique. Along with our complications will be listed a number of potential
mitigation strategies to discuss.

**The primary issue here is our use of `gogo/protobuf` extensions.** We should strive
to remove these extensions while maintaining backwards compatibility.

#### JSON/YAML (de)serialization (removing `gogoproto.jsontag`)

We rely heavily on `gogo/protobuf`'s custom JSON tags for serialization our messages into JSON.
Additionally, we use a library called [`ghodss/yaml`](https://github.com/ghodss/yaml) to create YAML,
which also relies on these JSON tags.

Each resource is serialized into JSON before being pushed into our backend, which means the
protobuf JSON tags have an impact quite far down our stack. There are a number of potential
mitigations here with differing levels of impact. The primary goal of any mitigation strategy
here should be **migrate away from gogoproto.jsontag extensions.**

##### Serialize objects in backend as the gRPC wire format

Serializing the objects in the backend as the gRPC wire format has a number of benefits:

- Seems to remove the need for most unmarshaling functions in `lib/services`. (confirm?)
- The backend has no need for JSON marshaling/unmarshaling, so JSON changes will not
  affect the backend.
- Allows us to remove gogoproto extensions without affecting backend storage.
- Allows us to utilize fields in the backend more efficiently and store larger objects.

The primary disadvantage here is that manually examining the backend is more difficult as
the stored values will no longer be human readable.

##### Write custom marshalers for objects

In order to keep our user facing representations of the objects, we will need to write
custom JSON and YAML marshalers for our objects. This may be time intensive but not overall
a significant amount of work.

#### Casting types (removing `gogoproto.casttype`)

We additionally rely heavily on `gogoproto.casttype`, which allows us to avoid doing explicit
casts or conversions within go code. We will have to take on this conversion work, unfortunately.

#### Other gogoproto tags

##### `gogoproto.nullable`

We will need to remove the `gogoproto.nullable` tags, which overall shouldn't be a huge impact.
We should migrate to using `optional` as defined [here](https://protobuf.dev/programming-guides/proto3/#field-labels).

##### `gogoproto.stdtime`

We will need to migrate to using `timestamppb` directly instead of relying on gogo's `stdtime` tag.

### UX

There should be no difference in UX between Teleport pre-migration and post-migration. The process
must be transparent to the user.

### Implementation plan

TBD