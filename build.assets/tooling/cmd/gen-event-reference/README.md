# Audit event reference generator

The audit event reference generator produces a list of audit events for
inclusion in the documentation.

## Background

Teleport audit events have unique codes (e.g., `TDB01I`) as well as types (e.g.,
`user.login`) that are not necessarily unique. Teleport transmits audit events
as protocol buffer messages that Teleport audit log represents as JSON messages.
Therefore, to represent audit events, we need to associate codes and types with
JSON schemas.

The Teleport source assigns audit event codes and types as struct values when
initializing an audit event. Otherwise, there is nothing intrinsic to an audit
event that associates its code, type, and schema. Finding all parts of the
source that emit an audit event is infeasible, so the best we can do is to take
advantage of the naming conventions we use for declarations of types, codes, and
schemas.

The reference generator assumes that event codes are constants declared in a
single file with a declaration name in the following format:

```
CamelCaseName(Success|Failure)?Code
```

`CamelCaseName` is a prefix we expect to find across the names of event codes,
types, and schemas.

The event type that corresponds to a code, also declared as a constant in a
single Go file, has a declaration name with the following format:

```
CamelCaseNameEvent
```

Finally, the generator expects audit event schemas to be declared as protobuf
messages with declaration names in the following format:

```
CamelCaseName
```

Not all audit event codes, types, and schemas follow this convention, but enough
events do follow it that we can iterate to either adjust the naming convention
or edit the names of event codes, types, and schemas.

To provide JSON schemas to the generator, we use the `protoc-gen-eventschema`
tool (`./build.assets/tooling/cmd/protoc-gen-eventschema`).

## Running the generator

Navigate to `build.assets` and run:

```bash
make generate-event-reference
```

After producing the reference, the generator prints a list of warnings
indicating event codes and types that deviate from the naming convention.
