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

The Database Permission Management is a proposal to introduce a new permission management system for databases integrated with Teleport. It aims to provide a structured and secure way to manage access permissions for users connecting to databases using Database Access.

## Why

As organizations increasingly rely on Teleport to access and manage their databases, there is a growing need for a unified and robust permission management system. The Database Permission Management feature will simplify the process of granting, revoking, and auditing database access permissions, ensuring that security and compliance requirements are met.

In the most common mode of operation, Database Access connects the user to the database as a requested user, but does not verify if the user exists or has the appropriate rights. This task is left for the database administrator to perform. Automatic user creation is a relatively new Database Access feature that automates part of this task in supported database configurations. On connection, a new database user is created (or an old one reactivated) with a name matching that of the Teleport user. This new user is given roles from a fixed list of roles. Teleport expects the roles to be managed manually and has no insight into their definition.

This RFD introduces a mechanism for _database permission management_, a way to define the effective database user permissions within Teleport roles and automatically apply them as needed. This provides visibility into the effective permissions and eases the burden on database administrators.

## Details

### UX

#### Usage

Teleport administrators modify the role definition assigned to the user, configuring both automatic user creation and individual permissions for the database.

On connection, the engine calculates the effective permissions from the roles and database schema.

After automatically creating the database user for the Teleport user who initiated the connection, the calculated set of permissions is applied to the database user.

After the session is finished, the user is removed/deactivated, and all permissions are revoked.

#### Configuration

1. User roles will be extended with a new `db_permissions` field in both `allow` and `deny` parts of the spec.
2. The new field will contain a list of DB permissions to be applied for the user.
3. Each individual DB permission will consist of four fields:

   ```protobuf
   // DatabasePermission is the single permission to be granted.
   message DatabasePermission {
   // Version is the version of permission. Combined with protocol it dictates the meaning of other fields.
   string Version = 1 [(gogoproto.jsontag) = "version"];
   // Protocol is the database protocol of permission.
   string Protocol = 2 [(gogoproto.jsontag) = "protocol"];
   // Target is the target of the permission, for example the name of database table.
   string Target = 3 [(gogoproto.jsontag) = "target"];
   // Access is the level of access granted by this permission to target.
   string Access = 4 [(gogoproto.jsontag) = "access"];
   }
   ```

   An example instance:

   ```yaml
   spec:
     allow:
       db_permissions:
         - protocol: postgres
           version: v1
           target: /public/customers
           access: SELECT
   ```

#### Database Permission Semantics

Each protocol-specific engine is responsible for handling the application of permissions to the database engines it supports. It is a decision of the engine to respect or ignore a particular permission based on the `protocol` and `version` fields. The precise interpretation of each individual DB permission will depend on the protocol and version, allowing for system evolution in a backwards-compatible way. It is explicitly allowed for a single role to reference different versions of the DB permission for the same protocol or any mixture of different protocols.

At this point, no addressing scheme for database objects is mandated across the protocols (see discussion [below](#database-object-addressing)). The meaning of `access` field is also database-dependent; some may only be valid for specific versions of databases (e.g. [`SELECT INTO S3`](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/AuroraMySQL.Integrating.SaveIntoS3.html)).

```yaml
spec:
  allow:
    db_permissions:
      - protocol: postgres
        version: v1
        target: /./customers
        access: SELECT
      - protocol: postgres
        version: v2
        target: '{ type: "schema", name: "public"}'
        access: ALL
```

The deny section specifies permission forbidden for the user. Since the interpretation of fields is left to individual engines, it is the engine's duty to perform the permission calculations, which involve subtracting denied permissions from allowed ones. Utilizing the underlying database engine for this process is encouraged as it leads to a more robust implementation. In the example below, the Postgres engine itself doesn't need to know that `ALL` subsumes `SELECT`. It can simply execute `GRANT SELECT` first, followed by `REVOKE ALL`.

```yaml
spec:
  allow:
    db_permissions:
      - protocol: postgres
        version: v1
        target: '/*/*'
        access: SELECT
  deny:
    db_permissions:
      - protocol: postgres
        version: v1
        target: '/secret/*'
        access: ALL
```

The effective combination of `spec.allow.db_permissions` fields can originate from multiple roles, as long as the `db_labels` field matches the current database. This applies to both the `allow` and `deny` sections, and each is tracked separately.

Notably, it is not considered an error to specify a permission for a database object that does not exist, or to use a wildcard that matches no objects. This flexibility allows for changes to the schema without needing to modify permissions concurrently.

However, specifying a non-existent _permission_ is likely an error and should be disallowed, unless a compelling argument exists to do otherwise.

The engine may ignore `db_permissions` with a version it doesn't support, but it must log a warning about this fact. Notably, this mechanism also applies to `spec.deny.db_permissions`. The security implications are discussed [below](#unsupported-permission-versions).

### Database object addressing

Depending on the specific database engine or permission version, various hierarchies of database objects may exist. It is essential to establish a method for addressing one or more objects within these hierarchies.

One potential approach is to adopt a straightforward filepath-like scheme as a foundation, such as `/public/customers`. However, it's important to allow for the quoting of identifiers, as some object names, like "foo/bar" in Postgres, may contain special characters. For example: `/public/"foo/bar"`. In cases where it's necessary to specify the object type within a particular hierarchy, it's possible to use a notation like `/public/table:*` to differentiate between object types, each requiring distinct permissions. For example, you may "EXECUTE" a stored procedure, but you "SELECT" from a table.

Another option is to base the addressing scheme on URIs. For instance, to select all tables in the default (`.`) schema, one might use `table://./*`. This approach conveniently handles the encoding of special characters like slashes or double quotes in object names.

Lastly, it's feasible to leverage existing standards like [JsonPath](https://github.com/json-path/JsonPath) for object addressing.

This RFD does not mandate a specific addressing scheme. However, several desirable properties should be considered when designing an addressing scheme:

- **Universality:** The scheme should be applicable to any database protocol.
- **Intuitiveness:** Users should be able to understand it correctly through casual observation, without requiring additional tools.
- **Conciseness:** It should be relatively straightforward to write and create by hand, if necessary.
- **Future-Proof:** The scheme should accommodate unforeseen changes in the future, such as the addition of additional levels within the hierarchy (e.g., schemas encompassing tables or columns within tables).
- **Unambiguity:** If a wildcard character like `*` is allowed, it should have a clear and consistent meaning. Similarly, when specifying an object like "customers," it should be evident whether it refers to a table or a schema.

### Wildcard Support

Specifying permissions individually for each database object can be tedious and challenging to maintain. Wildcards provide a succinct and stable alternative, especially in the face of evolving schemas. It is encouraged that database engines support wildcard functionality for targets. This implementation may involve enumerating database objects and applying permissions to those matching specific patterns. Additionally, if a new object is created during a user session, the corresponding wildcard-based permission will not be automatically applied until a permission synchronization occurs. Implementing an automated permission synchronization triggered by schema changes can enhance the user experience.

Unlike targets, permissions typically originate from a fixed set. While the inclusion of wildcards, as seen in the `ALL` permission, can still be beneficial, doing it at Teleport-level may overlap with database-level functionality, potentially diminishing utility.

### Permission Lifecycle

The feature is based on the automated database user creation feature, simplifying the analysis of permission lifetime.

We define "permission synchronization" as the process of applying a predetermined set of permissions to the user. These permissions are calculated based on their current roles in conjunction with the existing database schema.

1. Initially, when the user does not exist, they possess no permissions except for default ones.
2. Upon connection, we create a user in a manner specific to the database. This involves applying a fixed set of roles to the user, as specified in `spec.allow.db_roles`.
3. Permission synchronization occurs after the user is created or reactivated.
4. Additional permission synchronizations can be triggered as necessary, such as on new connections, detected schema changes (which may be detected using database-specific methods), or based on a scheduled timer.
5. For consistency, we ensure that new connections receive an equivalent set of permissions. Notably, permissions for other protocols or unsupported versions do not affect what is applied to the database. Changes in the database schema can lead to previously "identical" permission sets diverging, for example, when a new table is added. The permissions are equivalent if they are stable under schema changes.

6. Once the last connection for the user is terminated, they are either removed or deactivated.
7. When a user is removed, all permissions are automatically revoked. In cases where full removal is not possible (example: a user is the owner of database objects in Postgres), a process is in place to ensure that all permissions are removed from the user, regardless of their source. This approach acts as a precaution against potential lingering permissions and simplifies the management of permissions.

### TAG Integration

Fundamentally, TAG operates on the basis of graph nodes interconnected by edges. To align our permissions with TAG, we'll represent each potential target as an individual node within the graph.

Each target node will have its distinct set of relevant permissions, contingent on its type. For instance, tables may be associated with "SELECT" permissions, while procedures might involve "EXECUTE" permissions. These permissions will be represented as edges in the graph, each tagged with the appropriate permission type.

The representation of allowed but unapplied permissions remains unclear within TAG. Expanding wildcards in permissions into individual nodes and edges is essential since TAG's representation is flat and explicit, lacking further interpretation.

Certain targets may contain others; for example, a schema may encompass tables, and tables may include columns. This hierarchical relationship will form directed edges between nodes representing various database objects.

An important concern in TAG integration is the overall system performance. For instance, in a scenario with 1000 identical databases, each containing 1 schema and 100 tables with 10 columns each, we anticipate at least 1+100+100\*10 = 1101 graph nodes per database, or 1101000 in total. Managing over a million graph nodes can strain the graph database and visualization engine. While the need for a method to prune or compress the representation is evident, it remains unclear how to achieve this without compromising functionality.

The model described assumes a fixed representation between entities. This makes it a challenge to represent a policy-based permissions, such as AWS' IAM policies or row-level security policies in Postgres.

This section may be expanded into a standalone RFD in the future, to discuss this topic in more detail as TAG becomes more settled.

### Alternative Designs

Several alternative designs were considered for this feature, with one noteworthy option being the "managed roles" design, outlined below:

1. The concept of "managed database roles" would be introduced. These roles would closely resemble the structure of the `db_permissions` field, and each managed database role would be stored as an individual resource within the backend.

2. The database definition would be expanded to allow for referencing any number of these managed database roles.

3. A database agent responsible for connecting to the database would synchronize the expected and actual database roles triggered by various events, mirroring the approach used for user permission synchronization.

4. User roles, however, would remain unchanged. The existing mechanism for granting roles to automatically created users upon connection would still be employed.

The primary distinction in this alternative design is the entity responsible for holding the database permissions. In the main design, it's the user, while in this alternative approach, it's a specifically designed role. This difference has implications for various aspects, including implementation, user experience, reliability, and security.

The primary advantage of this alternative design is its ability to create database-level roles such as "sales," "accounting," or "devs" and grant permissions as needed. This functionality could be valuable on its own, even outside the context of Database Access, and might be integrated as part of a separate "Database Management" product or feature.

However, this alternative design introduces added complexity, including the need for a new resource type, the requirement to establish the capability to reference this new resource from existing resources, and the need to coordinate the interaction between databases agents. Additionally, a mechanism for cleaning up old roles or "deactivating" them would be necessary, particularly concerning disconnected databases with outdated roles. This approach would also introduce a new task for the database agent related to "Database role management," which would be separate from the core proxy functionality.

### Backward Compatibility

The new `db_permissions` field should be automatically ignored by Teleport versions that don't support it.

### Audit Events

Since there are no specific audit events in place for the user creation feature, there is currently no requirement to extend any existing events. However, in the event that audit events are introduced, it would be prudent to contemplate enhancing these events by including a list of the applied permissions. This list would be a combination of pertinent entries in both `spec.allow.db_permissions` and `spec.deny.db_permissions`.

### Observability

The expectation is that permission changes should occur swiftly. If necessary, we may consider monitoring the latency of schema queries and the time required to apply permissions. This becomes particularly relevant when permissions are managed external to the database instance, for instance, through a call to AWS Security Token Service (STS).

To enhance observability, it is advisable to introduce appropriate logging. Debug-level logs can be used to detail individual permissions granted, while keeping in mind that the list might encompass thousands of entries.

### Product Usage

As of the current point in time, there are no plans for the introduction of telemetry.

### Test Plan

In the test plan, a new section should be incorporated, positioned alongside the coverage for the "automated user creation" feature. This section should enumerate and test each supported configuration separately. At the time of writing this RFD, the coverage for the "automated user creation" feature in the test plan is absent, and it should be created.

### Security

The introduction of the new feature has no direct impact on the security of Teleport itself. However, it does have implications for the security of connected databases within the supported configurations. For environments requiring heightened security, there may be value in explicitly excluding specific databases from this feature, potentially using a resource label for this purpose.

It's important to recognize that with sufficiently broad permissions, a user might have the potential to elevate their database permissions further, including creating additional users and granting permissions. Given that this feature does not attempt to model the implications of individual database permissions, there is no foolproof mechanism to prevent such excessive permissions. Therefore, it falls to the system administrator to ensure that the granted permissions are kept to a minimum.

#### Unsupported Permission Versions

The protocol-specific engine is allowed to disregard a database permission with a version it does not support. However, if an unsupported permission originates from `spec.deny.db_permissions`, it may lead to permissions wider in scope than initially expected for the user. To address this, it is advisable to raise a warning and an associated audit event in such cases.
