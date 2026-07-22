---
authors: Krzysztof Skrzętnicki (krzysztof.skrzetnicki@goteleport.com)
state: implemented
---

# RFD 151 - Database Permission Management

## Required Approvals

- Engineering: @r0mant && @greedy52
- Product: @xinding33 || @klizhentas
- Security: @reedloden || @jentfoo

## What

Extends [automated db user provisioning](0113-automatic-database-users.md) with
permission management capabilities.

## Why

Database-level permission management is a natural extension of current Teleport
RBAC capabilities. The described model should integrate seamlessly with TAG,
providing a future-proof solution.

Database administrators will be able to use Teleport for both user and
permission management.

## Details

### Database object import rules

Teleport will fetch database objects and store them as resources in the backend,
using a set of global "database object import rules". At certain points in time,
the database schema will be read and passed through the import rules. Each
import rule defines a set of database labels that must match in order for the
import rule to fire. The rules are processed in order (as defined by the
`priority` field).

Each import rule defines a set mappings. Individual mapping consists of a scope
definition (`database_names`, `schema_names`), object names to match
(`procedure_names`, `table_names`, ...) and a list of labels to apply. The
expected values for object names are specified as a list, while the matching
uses glob-like semantics, with `*` matching any set of characters. Empty list of
names matches nothing, while an empty scope matches everything.

The labels applied by the import rules can reference the object properties, like
`schema` or `database`, to populate the labels with dynamic, object-dependent
values.

#### Examples

The `import_all_objects` below is a catch-all import rule which tags all objects
in all databases with values taken from database object spec. This also a
default import rule that will be created on startup if there are no other rules
present.

```yaml
kind: db_object_import_rule
metadata:
  name: import_all_objects
  namespace: default
spec:
  database_labels:
    - name: '*'
      values:
        - '*'
  mappings:
    - add_labels:
        database: '{{obj.database}}'
        kind: '{{obj.object_kind}}'
        name: '{{obj.name}}'
        protocol: '{{obj.protocol}}'
        schema: '{{obj.schema}}'
        service_name: '{{obj.database_service_name}}'
      match:
        procedure_names:
          - '*'
        table_names:
          - '*'
        view_names:
          - '*'
version: v1
```

As another example, the `widget-prod` import rule applies to databases with
`env: prod` label. It has the priority 10. If there is a database object that
matches the clauses in `spec.mappings.match`, the import rule will apply the
specified labels to the database object (`env: prod`,
`product: WidgetMaster3000`). The `local_id` label references multiple values,
which will be concatenated to produce a single string.

```yaml
kind: db_object_import_rule
version: v1
metadata:
  name: rule_widget_prod
spec:
  priority: 10
  database_labels:
    env: prod
  mappings:
    - scope:
        database_names:
          - Widget*
        schema_names:
          - widget
          - sales
          - public
          - secret
      match:
        procedure_names:
          - '*sales*'
        table_names:
          - '*sales*'
        view_names:
          - '*sales*'
      add_labels:
        env: prod
        product: WidgetMaster3000
        schema_with_prefix: 'schema-{{obj.schema}}'
```

There is a Postgres database with a matching labels. Parsing the schema, a
number of objects is found, including the following one which matches the import
rule above (pattern matches are noted):

```yaml
kind: db_object
version: v1
metadata:
  name: widget-sales
spec:
  database: WidgetUltimate # matches 'Widget*'
  db_service_name: all-things-widget
  name: widget-sales # matches '*sales*'
  object_kind: table
  protocol: postgres
  schema: sales
```

After processing, the import rule has applied the labels to the object. As there
are no other import rules matching this object, it is stored in the backend in
this form.

```yaml
kind: db_object
version: v1
metadata:
  name: widget-sales
  labels:
    env: prod
    product: WidgetMaster3000
spec:
  database: WidgetUltimate
  db_service_name: all-things-widget
  name: widget-sales
  object_kind: table
  protocol: postgres
  schema: sales
```

Any particular attribute can be omitted from the import rule; an import rule
with empty `match` part will match all objects.

A single rule can also specify multiple sets of labels to be applied. This is
useful for applying a label with different set of values, depending on database
object attributes.

```yaml
kind: db_object_import_rule
version: v1
metadata:
  name: mark_confidential
spec:
  database_labels:
    env: prod
  mappings:
    - scope:
        schema_names:
          - private
          - sales
          - secret
      add_labels:
        confidential: 'true'
    - scope:
        schema_names:
          - public
      add_labels:
        confidential: 'false'
  priority: 20
```

As another example, a wide rule to import all tables in schema "public" from all
staging databases may look as follows:

```yaml
kind: db_object_import_rule
version: v1
metadata:
  name: import_all_staging_tables
spec:
  database_labels:
    env: staging
  mappings:
    - add_labels:
        custom_label: my_custom_value
        env: staging
      match:
        table_names:
          - '*'
      scope:
        schema_names:
          - public
  priority: 30
```

A more fine-grained rule, targeting a specific set of tables:

```yaml
kind: db_object_import_rule
version: v1
metadata:
  name: import_specific_tables
spec:
  database_labels:
    env: dev
  mappings:
    - add_labels:
        custom_label: my_custom_value
        env: dev
      match:
        table_names:
          - table1
          - table2
          - table3
      scope:
        schema_names:
          - public
  priority: 30
```

#### Import process

The database objects are imported by the database agent when establishing a new
user session to the database. In this context, they are immediate inputs for
permission calculation; the desired per-object permissions are subsequently
written back to the database.

Imported objects are also stored in the backend, where TAG can access them. This
enables the TAG to visualize the permissions.

Additionally, the imports will be done on a predetermined schedule (e.g. every
10 minutes), and stored in the backend. If the database engine supports it, the
sync may also happen when a schema change is detected. For example, in Postgres
we case use
[trigger+notify](https://medium.com/launchpad-lab/postgres-triggers-with-listen-notify-565b44ccd782).

#### Import result: the `db_object` resource

Import process creates a number of `db_object` resources with applied labels.
Aside from standard metadata fields, the object spec consists of a number of
predefined attributes:

```protobuf
// DatabaseObjectSpec is the spec for the database object.
message DatabaseObjectSpec {
  string protocol = 1;
  string database_service_name = 2;
  string object_kind = 3;
  string database = 4;
  string schema = 5;
  string name = 6;
}
```

All of the above fields are optional, and more may be added in the future if
needed. The database-specific implementation is responsible for populating these
fields from the database schema.

### Database object permissions

The permissions for particular objects are defined in a role using the new
`db_permissions` field, found under `spec.allow` and `spec.deny` respectively.
Each permission specifies:

- A list of labels the database object must match.
- A list of permissions that will be given to the object for the user.

As an example, the role `db_read_non_confidential` allows access to tables and
views with a label `confidential: false` and explicitly disallows any access to
those labeled `confidential: true`.

```yaml
kind: role
version: v7
metadata:
  name: db_read_non_confidential
spec:
  allow:
    db_permissions:
      - match:
          confidential: 'false'
          kind:
            - table
            - view
        permissions:
          - SELECT
          - INSERT
          - UPDATE
  deny:
    db_permissions:
      - match:
          confidential: 'true'
        permissions:
          - '*'
```

The object attributes are not replicated automatically as labels. In the example
above, `kind` is a label which must be applied by an import rule, for example
one as follows:

```yaml
kind: db_object_import_rule
metadata:
  name: object-kind
spec:
  database_labels:
    '*': '*'
  mappings:
    - add_labels:
        kind: table
      match:
        table_names:
          - '*'
    - add_labels:
        kind: view
      match:
        view_names:
          - '*'
    - add_labels:
        kind: procedure
      match:
        procedure_names:
          - '*'
  priority: 100
version: v1
```

#### Applying permissions

The permissions will be applied to the user after the user is provisioned in the
database. The exact mechanism will be database-specific; for example, in SQL
databases like Postgres or MySQL this will be done through appropriate `GRANT`
statements, executed through a helper stored procedure.

After the session is finished, the user is removed/deactivated, and _all_
permissions must be revoked. Again, for SQL databases, a corresponding `REVOKE`
statements will be issued. To ensure complete removal of all permissions, the
stored procedure will iterate over all schemas and objects within, revoking the
access to each individual object.

Updating the permissions due to schema change is a feature that is out of scope
of initial implementation.

Similarly, the permissions will not be updated due to changed role definition.
Finally, the list of roles for a given user is unchanging in the scope of a
single connection, so this is not an element that may change.

To avoid confusion regarding the source of access, `db_permissions` will be
mutually exclusive with `db_roles`.

The precise meaning of individual permissions is database-specific.

However, we mandate the interaction between the `deny` and `allow` parts:

- permissions found in `deny` remove matching permissions in `allow`;
- the permissions are compared as strings in case-insensitive manner, with
  trimmed whitespace;
- `*` in `deny` matches all permissions in `allow`.

For example, if the `allow` permission contains `SELECT`, then it can be removed
with `select`, `SELECT` or `*`.

Invalid permissions are prohibited. This is a fatal error and should cause
connection error.

### TAG Integration

The `db_object` resources can be imported to TAG, which can reimplement the
permission semantics described above. The permission algorithm is based on label
matching, which should be an operation that is easy to support in TAG, given its
widespread usage in Teleport RBAC. Performance wise it may be necessary to avoid
queries spanning objects from multiple databases - a single database can viably
produce thousands of objects - but nevertheless, it should remain a scalable
approach.

### Backward Compatibility

The new `db_permissions` field should be automatically ignored by Teleport
versions that don't support it. The new resources will likewise be ignored.

### Audit Events

The [RFD 113](0113-automatic-database-users.md) introduces events
`db.user.created` and `db.user.disabled`, but these are yet to be implemented.

The `db.user.created` should be extended with a summarized list of applied
permissions (permission types, object types, counts).

### Observability

The expectation is that permission should be applied swiftly. If necessary, we
may consider monitoring the latency of schema queries and the time required to
apply permissions. This becomes particularly relevant when permissions are
managed in the systems external to the database instance, for instance through a
call to AWS Security Token Service (STS).

To enhance observability, appropriate logging will be added in key points.
Debug-level logs can be used to detail individual permissions granted, while
keeping in mind that the full list might be thousands of entries long.

### Product Usage

A new PostHog event should be added, summarizing the information from the
`db.user.created` audit event: protocol, number of affected objects of each
kind, number of permissions.

### Test Plan

The feature shall be tested using automated e2e tests. Each supported
configuration should be tested separately.

### Security

The introduction of the new feature has no direct impact on the security of
Teleport itself, as the permissions will be applied to resourced managed with
Teleport, not any of the internal services. However, it does have implications
for the security of connected databases. As a hardening measure, a blanked deny
role can be used to deny all possible permissions:

```yaml
kind: role
version: v7
metadata:
  name: db_deny_all_permissions
spec:
  deny:
    db_permissions:
      - match:
          '*': '*'
        permissions:
          - '*'
```

It's important to recognize that with sufficiently broad permissions, a user
might have the potential to elevate their database permissions further via
database-specific means, including creating additional users and granting
permissions. It is responsibility of the system administrator to ensure the
permissions are not excessive, and this fact should be reflected in Teleport
documentation for this feature. However, a suitable IGS report may be helpful in
this context.
