---
authors: Krzysztof Skrzętnicki (krzysztof.skrzetnicki@goteleport.com)
state: draft
---

# RFD 151 - Database Permission Management

## Required Approvals

- Engineering: @r0mant && @smallinsky
- Product: @xinding33 || @klizhentas
- Security: @reedloden || @jentfoo

## What

Extends [automated db user provisioning](0113-automatic-database-users.md) with
permission management capabilities.

## Why

Managing database-level permissions is a natural extension of current Teleport
RBAC capabilities. The described model should also integrate seamlessly with
TAG, providing a future-proof solution.

Database administrators will be able to use Teleport for both user and
permission management.

## Details

### UX

A set of global "database object import rules" resources are defined for the
Teleport cluster. At certain points in time, the database schema is read and
passed through the import rules. The individual import rules may apply custom
labels. An database object resource will only be created if there is at least
one import rule that matched it.

Only object attributes (standard ones like `protocol` or custom from the
`attributes` field), which are sourced from the object spec, are subject to
matching against the import rules. Any labels present on an object, either from
an import rule or another source, are not subject to be matched by the import
rule. The permissions matching is different in this regard.

Example: the `sales-prod` import rule, which imports all tables from `sales`
Postgres database in prod:

```yaml
kind: db_object_import_rule
metadata:
  name: sales-prod
spec:
  db_labels:
    env: prod
  mappings:
    - object_match:
        - database: sales
          object_kind: table
          protocol: postgres
      add_labels:
        env: prod
        product: sales
  priority: 10
version: v1
```

Example database object, with applied labels:

```yaml
kind: db_object
metadata:
  labels:
    env: prod
    product: sales
  name: sales_main
spec:
  attributes:
    attr1: custom attr1 value
    attr2: custom attr2 value
  database: sales
  name: sales_main
  object_kind: table
  protocol: postgres
  schema: public
  service_name: sales-prod-123
version: v1
```

The permissions to particular objects are defined in a role using the new field
`db_permissions`. The permission specifies the object kind (e.g. `table`),
permission to grant/revoke (`SELECT`) as well as labels to be matched.

The matching is performed against database object:

- attributes: standard and custom, sourced from the object spec, provided by
  database-specific schema import code
- resource labels: aside from standard labels, these can be manipulated by the
  import rules.

If both attributes and labels provide the same non-empty key, the attributes
take preference.

Example role `db-dev`, which grants `SELECT` permission to some objects, and
revokes `UPDATE` from all others.

```yaml
kind: role
metadata:
  name: db-dev
spec:
  allow:
    db_permissions:
      - match:
          product: sales
          protocol: postgres
        object_kind: table
        permission: SELECT
  deny:
    db_permissions:
      - object_kind: table
        permission: UPDATE
```

The database objects are imported during the connection, to be used for
permission calculation.

Additionally, the imports will be done on a predetermined schedule (e.g. every
10 minutes), and stored in the backend.

The database objects stored in the backend will be used by TAG.

The permissions will be applied to the user after the user is provisioned in the
database. After the session is finished, the user is removed/deactivated, and
all permissions are revoked.

#### Configuration

##### Resource: `db_object_import_rule`

A new kind of resource `db_object_import_rule` is introduced.

The import rules are processed in order (defined by `priority` field) and
matched against the database labels.

If the database matches, the object-level mapping rules are processed.

```protobuf
// DatabaseObjectImportRuleV1 is the resource representing a global database object import rule.
message DatabaseObjectImportRuleV1 {
   option (gogoproto.goproto_stringer) = false;
   option (gogoproto.stringer) = false;

   ResourceHeader Header = 1 [
      (gogoproto.nullable) = false,
      (gogoproto.jsontag) = "",
      (gogoproto.embed) = true
   ];

   DatabaseObjectImportRuleSpec Spec = 2 [
      (gogoproto.nullable) = false,
      (gogoproto.jsontag) = "spec"
   ];
}

// DatabaseObjectImportRuleSpec is the spec for database object import rule.
message DatabaseObjectImportRuleSpec {
  // Priority represents the priority of the rule application. Lower numbered rules will be applied first.
  int32 Priority = 1 [(gogoproto.jsontag) = "priority"];

  // DatabaseLabels is a set of labels which must match the database for the rule to be applied.
  map<string, string> DatabaseLabels = 2 [(gogoproto.jsontag) = "db_labels,omitempty"];

  // Mappings is a list of matches that will map match conditions to labels.
  repeated DatabaseObjectImportRuleMapping Mappings = 3 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "mappings,omitempty"
  ];
}
```

Individual objects are matched with against all object matches defined in
`DatabaseObjectImportRuleMapping` message. If any of the matches succeeds (or if
the list of matches is empty), the labels specified in the mapping are applied
to the object. The processing continues with the next mapping: multiple mappings
can match any given object.

```protobuf
// DatabaseObjectImportRuleMapping is the mapping between object properties and labels that will be added to the object.
message DatabaseObjectImportRuleMapping {
  // ObjectMatches is a set of object matching rules for this mapping.
  // For a given database object, each of the matches is attempted.
  // If any of them succeed, the labels are applied.
  repeated DatabaseObjectSpec ObjectMatches = 2 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "object_match"
  ];

  // AddLabels specifies which labels to add if any of the previous matches match.
  map<string, string> AddLabels = 3 [(gogoproto.jsontag) = "add_labels"];
}
```

##### Resource: `db_object`

Another new resource is the database object (`db_object`), imported using the
import rules from the database.

The spec for this resource is the `DatabaseObjectSpec` message, which is also
used in the `DatabaseObjectImportRuleMapping`.

The spec is equivalent to a `map<string, string>`, except it predefines a few
optional properties.

In case of an `db_object`, the entirety of the spec is provided by
database-specific schema introspection tool. The custom attributes provide a way
to express additional properties important to the object, which cannot be
rightly placed in other attributes. The `object_kind` property is mandatory for
`db_object` resources.

```protobuf
// DatabaseObjectSpec is the spec for the database object.
message DatabaseObjectSpec {
   string Protocol = 1 [(gogoproto.jsontag) = "protocol,omitempty"];
   string ServiceName = 2 [(gogoproto.jsontag) = "service_name,omitempty"];
   string ObjectKind = 3 [(gogoproto.jsontag) = "object_kind"];
   string Database = 4 [(gogoproto.jsontag) = "database,omitempty"];
   string Schema = 5 [(gogoproto.jsontag) = "schema,omitempty"];
   string Name = 6 [(gogoproto.jsontag) = "name,omitempty"];
   // extra attributes for matching
   map<string, string> Attributes = 7 [(gogoproto.jsontag) = "attributes,omitempty"];
}
```

#### Role extension: `spec.{allow,deny}.db_permissions` fields

The role is extended with a `db_permissions` field, which consists of a list of
permissions to be applied against particular database objects provided the
permission properties (object kind, match labels) match the given object.

```protobuf

  // ...
  // DatabasePermission specifies a set of permissions that will be granted
  // to the database user when using automatic database user provisioning.
  repeated DatabasePermission DatabasePermissions = 38 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "db_permissions,omitempty"
  ];
}

// DatabasePermission specifies the database object permission for the user.
message DatabasePermission {
  // ObjectKind is the database object kind: table, schema, etc.
  string ObjectKind = 1 [(gogoproto.jsontag) = "object_kind"];
  // Permission is the string representation of the permission to be given, e.g. SELECT, INSERT, UPDATE, ...
  string Permission = 2 [(gogoproto.jsontag) = "permission"];
  // Match is a list of labels (key, value) to match against database object properties.
  map<string, string> Match = 3 [(gogoproto.jsontag) = "match,omitempty"];
}

```

### Permission Semantics

The precise meaning of individual permissions is left to the database engine.
The sole exception is the interaction between the `deny` and `allow` parts:
denied permissions are removed, and the comparison is performed in a
case-insensitive way after trimming the whitespace. As a special case, `*` is
allowed as a permission in `deny` part of the role.

For example, if the `allow` permission is `SELECT`, then it can be removed with
`select`, `SELECT` or `*`.

The engines should prohibit invalid permissions. This is a fatal error and
should cause connection error.

### Permission Lifecycle

Permissions are applied after the user is automatically provisioned.
Deprovisioning (deletion/deactivation) of the user should remove ALL permissions
from the user, without the need to reference the particular user permissions.

The permission synchronization is performed at least once, on connection time:

- database schema is read,
- import rules are applied,
- effective permissions are calculated based on user roles,
- database-side permissions are updated.

Optional, additional syncs may happen as needed (e.g. when a schema change is
detected).

### TAG Integration

Periodic application of import rules will populate `db_object` resources in the
backend. These can be imported to TAG, where permission calculation may happen.
The permission algorithm is based on label matching, which should be a primitive
operation that is easy to support in TAG. Performance wise it may be necessary
to filter the set of objects - a single database can viably produce thousands of
objects - but it should remain a viable approach.

### Backward Compatibility

The new `db_permissions` field should be automatically ignored by Teleport
versions that don't support it. The new resources will likewise be ignored.

### Audit Events

The [RFD 113](0113-automatic-database-users.md) introduces events
`db.user.created` and `db.user.disabled`, but these are yet to be implemented.

The `db.user.created` should be extended with an effective list of applied
permissions. Since the list of database objects may be long, it may be necessary
to summarize the list of changes in the audit event.

### Observability

The expectation is that permission changes should occur swiftly. If necessary,
we may consider monitoring the latency of schema queries and the time required
to apply permissions. This becomes particularly relevant when permissions are
managed external to the database instance, for instance, through a call to AWS
Security Token Service (STS).

To enhance observability, it is advisable to introduce appropriate logging.
Debug-level logs can be used to detail individual permissions granted, while
keeping in mind that the list might encompass thousands of entries.

### Product Usage

As of the current point in time, there are no plans for the introduction of
telemetry.

### Test Plan

In the test plan, a new section should be incorporated, positioned alongside the
coverage for the "automated user provisioning" feature. This section should
enumerate and test each supported configuration separately. At the time of
writing this RFD, the coverage for the "automated user provisioning" feature in
the test plan is absent, and it should be added.

### Security

The introduction of the new feature has no direct impact on the security of
Teleport itself. However, it does have implications for the security of
connected databases within the supported configurations. For environments
requiring heightened security, there may be value in explicitly excluding
specific databases from this feature, potentially using a resource label for
this purpose.

It's important to recognize that with sufficiently broad permissions, a user
might have the potential to elevate their database permissions further via
database-specific means, including creating additional users and granting
permissions. Given that this feature does not attempt to model the implications
of individual database permissions, there is no foolproof mechanism to prevent
such excessive permissions. Therefore, it falls to the system administrator to
ensure that the granted permissions are kept to a minimum.
