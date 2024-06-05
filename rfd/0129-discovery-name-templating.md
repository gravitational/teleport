---
authors: Gavin Frazar (gavin.frazar@goteleport.com)
state: implemented
---

# RFD 0129 - Avoid Discovery Resource Name Collisions

## Required Approvers

- Engineering: `@r0mant && @smallinsky && @tigrato`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden || @jentfoo`

## What

Auto-Discovery shall name discovered resources such that other resources of
the same kind are unlikely to have the same name.

In particular, discovered cloud resource names shall include uniquely
identifying metadata in the name such as region, account ID, or sub-type name.

`tsh` sub-commands shall allow users to use a prefix of the resource name when
the prefix unambiguously identifies a resource.

Additionally, `tsh` sub-commands shall support using label selectors to
unambiguously select a single resource.

This RFD does not apply to ssh server instance discovery, since servers are
already identified within the Teleport cluster by a UUID.

## Why

Multiple discovery agents can discover resources with identical names.
For example, this happened when customers had databases in different AWS
regions or accounts with the same name. When a name collision occurs, only one
of the databases can be accessed by users.

Name collisions can be avoided with the addition of other resource metadata
in the resource name.

Since discovered resource names will be longer and more tedious to use, we
should support resource name prefixes and label matching in `tsh`, Teleport
Connect, and the web UI for better UX.

Relevant issue:
- https://github.com/gravitational/teleport/issues/22438

## Details

#### AWS Discovery

Discovered database and kube cluster names shall have a lowercase suffix
appended to it that includes:

- Name of the AWS matcher type
  - `eks`, `rds`, `rdsproxy`, `redshift`, `redshift-serverless`, `elasticache`,
    `memorydb` (as of writing this RFD)
  - additionally, the RDS subtype `rds-aurora` is used to distinguish RDS
    instances vs RDS Aurora clusters.
- AWS region
- AWS account ID

All of these AWS resource types require a unique name within an AWS account
and region.

By including the region and account ID, resources of the same kind
in different AWS accounts or regions will avoid name collision with each-other.

By including the Teleport matcher type in the name, resources of different
sub-kinds will also avoid name collision.

By combining these properties, resource names will not collide.

The reason for including `eks` in kube cluster names, even though this is the
only "kind" of kube cluster we discover in AWS, is to clearly distinguish the
cluster further from clusters in other clouds, although this isn't strictly
necessary.

Example:
```yaml
discovery_service:
  enabled: true
  aws:
    - types: ["eks", "rds", "redshift"]
      regions: ["us-west-1", "us-west-2"]
      assume_role_arn: "arn:aws:iam::111111111111:role/DiscoveryRole"
      external_id: "123abc"
      tags:
        "*": "*"
    - types: ["eks", "rds", "redshift"]
      regions: ["us-west-1", "us-west-2"]
      assume_role_arn: "arn:aws:iam::222222222222:role/DiscoveryRole"
      external_id: "456def"
      tags:
        "*": "*"
```

If the discovery service is configured like the above, the discovery agent will
discover AWS EKS clusters and AWS RDS and Redshift databases in the `us-west-1`
and `us-west-2` AWS regions, in AWS accounts `111111111111` and `222222222222`.

Now suppose that an EKS cluster, RDS database, and Redshift database all named
`foo` exist in both regions in both AWS accounts.
If the discovery service applies the new naming convention, the discovered
resources should be named:

- `foo-eks-us-west-1-111111111111`
- `foo-eks-us-west-2-111111111111`
- `foo-eks-us-west-1-222222222222`
- `foo-eks-us-west-2-222222222222`
- `foo-rds-us-west-1-111111111111`
- `foo-rds-us-west-2-111111111111`
- `foo-rds-us-west-1-222222222222`
- `foo-rds-us-west-2-222222222222`
- `foo-redshift-us-west-1-111111111111`
- `foo-redshift-us-west-2-111111111111`
- `foo-redshift-us-west-1-222222222222`
- `foo-redshift-us-west-2-222222222222`

This naming convention does not violate our database name validation regex,
`^[a-z]([-a-z0-9]*[a-z0-9])?$`,
and does not violate our kube cluster name validation regex `^[a-zA-Z0-9._-]+$`.

#### Azure Discovery

Azure resources have a resource ID that uniquely identifies the resource, e.g.:
`/subscriptions/00000000-1111-2222-3333-444444444444/resourceGroups/<group name>/providers/<provider name>/<name>`

We could use this ID as the database name, but it is unnecessarily verbose.
It will also fail to match our database name validation regex:
`[a-z]([-a-z0-9]*[a-z0-9])?`.

Additionally, all of the Azure databases, that Teleport currently supports
*except Redis* require globally unique names (within the same type of database),
because Azure assigns a DNS name:

- Redis: `<name>.redis.cache.windows.net`.
- Redis Enterprise: `<name>.redisenterprise.cache.azure.net`.
- SQL Server: `<name>.database.windows.net`.
- Postgres: `<name.postgres.database.azure.com`.
- MySQL: `<name>.mysql.database.azure.com`.

MySQL/Postgres server names must be unique among both single-server and
flexible-server instances.

Therefore, we can form a uniquely identifying name among Azure resources just by
adding the kind of matcher to the resource name.
However, AKS kube clusters do not require globally unique names - they only need
to be unique within the same resource group in the same subscription.

Additionally, resource group names may contain characters that are not valid
in Teleport database/kube names, so we must either omit the resource group name
in those cases or perform some kind of string transform.
If we include the resource region, it will serve as a heuristic to avoid name
collision when resource group names contain invalid characters.
Including resource region will also be consistent with the other cloud naming
schemes.

To make the naming convention consistent, and to "future-proof" it, the
naming convention will be to append a suffix that includes:

- Name of the Azure matcher type
  - `aks`, `mysql`, `postgres`, `redis`, `sqlserver` (as of writing this RFD)
  - additionally, `redis-enterprise` will be used to subtype Redis enterprise
    databases to distinguish them from non-enterprise Redis databases.
- Azure region
- Azure resource group name
  - resource group names may contain characters that we do not allow in database
    or kube cluster names.
    The resource group name should be checked for invalid characters and dropped
    from the name suffix if it is invalid.
    This is only a heuristic, but any approach here will be a heuristic, and
    this is the simplest string transform we can do, which avoids confusing
    users with strange resource group names they don't recognize.
- Azure subscription ID
  - subscription IDs only contains letters, digits, and hyphens.

Example:
```yaml
discovery_service:
  enabled: true
  aws:
    - types: ["aks", "mysql", "postgres"]
      regions: ["eastus"]
      subscriptions:
        - "11111111-1111-1111-1111-111111111111"
        - "22222222-2222-2222-2222-222222222222"
      resource_groups: ["group1", "group2", "weird-)(-group-name"]
      tags:
        "*": "*"
```

If the discovery service is configured like the above, the discovery agent will
discover Azure AKS kube clusters, Azure MySQL, and Azure PostgreSQL databases.

Now suppose that four AKS kube clusters named `foo` exist in each combination of
resource group and subscription ID, and a MySQL database and Postgres database
both named `foo` exist in the `1111..` subscription and `group1`.
If the discovery service applies the new naming convention, the discovered
resources should be named:

- `foo-eastus-aks-group1-11111111-1111-1111-1111-111111111111`
- `foo-eastus-aks-group2-11111111-1111-1111-1111-111111111111`
- `foo-eastus-aks-group1-22222222-2222-2222-2222-222222222222`
- `foo-eastus-aks-group2-22222222-2222-2222-2222-222222222222`
- `foo-eastus-mysql-group1-11111111-1111-1111-1111-111111111111`
- `foo-eastus-postgres-group1-11111111-1111-1111-1111-111111111111`

If resources exist within the Azure resource group `weird-)(-group-name`,
then we simply drop the resource group name from the resource name:

- `foo-eastus-aks-11111111-1111-1111-1111-111111111111`
- `foo-eastus-aks-22222222-2222-2222-2222-222222222222`
- `foo-eastus-mysql-11111111-1111-1111-1111-111111111111`
- ...

Unfortunately, this would allow name collisions across resource groups.

Alternatively, we could apply a transformation to the resource group name to
make it valid.
For example, base64 encode it, make the string lowercase, and replace the
`[+/=]` characters with valid characters, maybe even truncating the result:
(another heuristic, although less likely to collide names):

```sh
$ echo "weird-)(-group-name" | base64 | sed 's#[+/=]#x#g' | tr '[:upper:]' '[:lower:]' | cut -c1-8 
d2vpcmqt
$ echo "other-weird-)(-group-name" | base64 | sed 's#[+/=]#x#g' | tr '[:upper:]' '[:lower:]' | cut -c1-8 
b3rozxit
```

- `foo-eastus-aks-d2vpcmqt-11111111-1111-1111-1111-111111111111`
- `foo-eastus-aks-b3rozxit-11111111-1111-1111-1111-111111111111`
- ...

Each database name will be unique, since `foo` must be globally unique among
all Azure MySQL databases and globally unique among all Azure Postgres databases.

Even if a new database type is added that doesn't have this globally unique
name property, the resource group name and subscription ID will avoid name
collisions, and the databases will be distinguished from databases in other
clouds.
If resource group name has invalid characters, the Azure region will make name
collisions even more unlikely.

Likewise, the discovered AKS clusters will avoid colliding with other kube
clusters in Azure or other clouds.

This naming convention does not violate our database name validation regex,
`^[a-z]([-a-z0-9]*[a-z0-9])?$`,
and does not violate our kube cluster name validation regex `^[a-zA-Z0-9._-]+$`.

#### GCP Discovery

GCP discovery currently supports discovering only GKE kube clusters.

GKE cluster names are unique within the same GCP project ID and location/zone.

The discovery naming convention for GKE clusters shall be to append a suffix to
the cluster name that includes:

- Name of the Teleport GCP matcher type
  - `gke`
- GCP project ID
  - These can be custom, but will only consist of characters, digits, hyphens.
- GCP location

```yaml
    gcp:
    - types: ["gke"]
      locations: ["us-west1", "us-west2"]
      tags:
        "*": "*"
      project_ids: ["my-project"]
```

If the discovery service is configured like the above, the discovery agent will
discover GCP GKE kube clusters in "my-project" in the `us-west1` and `us-west2`
locations.

Now suppose GKE clusters named `foo` exist in each region.
If the discovery service applies the new naming convention, the discovered
resources should be named:

- `foo-gke-us-west1-my-project`
- `foo-gke-us-west2-my-project`

This naming convention avoids name collisions between GKE clusters and does not
collide with discovered AWS/Azure clusters.

This naming convention does not violate our kube cluster name validation regex:
`^[a-zA-Z0-9._-]+$`

### `tsh` UX

Users will be frustrated if they are forced to type out verbose resource names
when using `tsh`.
To avoid this poor UX, sub-commands should support prefix resource names, label
matching, or using a predicate expression to select a resource.

The same UX should apply to all `tsh` sub-commands that take a resource name
argument. These commands shall support
`tsh <sub-command> [--labels keys=val1,key2=val2,...] [--query <predicate>] [name | prefix]` syntax:

- `tsh db login`
- `tsh db logout`
- `tsh db connect`
- `tsh db env`
- `tsh db config`
- `tsh kube login`
- `tsh app login`
- `tsh app logout`
- `tsh app config`
- `tsh proxy db`
- `tsh proxy kube`
- `tsh proxy app`

To support prefix names, we add a new predicate expression function
`hasPrefix`, and change the `tsh` API calls to use `hasPrefix(name, "<prefix>")`
rather than the current predicate expression `name == "<name>"`.

The `--query` flag provides the full power of the predicate language, which
includes label matching.
The `--labels` flag provides a less powerful, but more convenient notation for
selecting a resource by matching labels.

We already support both of these cli features as either a flag or positional arg
in other `tsh` commands, e.g. `tsh db ls --query="..." key1=val1,key2=val2,...`

When `--query` is used along with a positional arg for the resource name or
prefix, we will need to combine the two as a single predicate expression, e.g.
`tsh db connect --query='labels.env == "prod"' foo-db`
will be combined into the predicate expression `hasPrefix(name, "foo-db") && (labels.env == "prod")`

#### `tsh` examples

To illustrate the new UX for `tsh` sub-commands, here is an example using
`tsh db connect` to select a database (the same applies for other commands):

```sh
$ tsh db ls
Name   Description         Allowed Users       Labels                      Connect 
------ ------------------- ------------------- --------------------------- ------- 
bar    RDS instance in ... [*] account-id=123456789012,region=us-west-1,env=dev,...
bar    RDS instance in ... [*] account-id=123456789012,region=us-west-2,env=dev,...
foo    RDS instance in ... [*] account-id=123456789012,region=us-west-1,env=prod,...

# connect by prefix name
$ tsh db connect --db-user=alice --db-name-postgres foo
#...connects to "foo-rds-us-west-1-123456789012" by prefix...

# ambiguous prefix name is an error
$ tsh db connect --db-user=alice --db-name-postgres bar
error: ambiguous database name could match multiple databases:
Name                           Description               Protocol Type URI                                                   Allowed Users Labels                                                                                                                                    Connect 
------------------------------ ------------------------- -------- ---- ----------------------------------------------------- ------------- ----------------------------------------------------------------------------------------------------------------------------------------- ------- 
bar-rds-us-west-1-123456789012 RDS instance in us-west-1 postgres rds  bar.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432 [*]           account-id=123456789012,endpoint-type=instance,engine-version=13.10,engine=postgres,env=dev,region=us-west-1,teleport.dev/origin=dynamic          
bar-rds-us-west-2-123456789012 RDS instance in us-west-2 postgres rds  bar.abcdefghijklmnop.us-west-2.rds.amazonaws.com:5432 [*]           account-id=123456789012,endpoint-type=instance,engine-version=13.10,engine=postgres,env=dev,region=us-west-2,teleport.dev/origin=dynamic          

Hint: try addressing the database by its full name or by matching its labels (ex: tsh db connect key1=value1,key2=value2).
Hint: use `tsh db ls -v` or `tsh db ls --format=[yaml | json]` to list all databases with verbose details.

# resolve the error by connecting with an unambiguous prefix 
$ tsh db connect --db-user=alice --db-name-postgres bar-rds-us-west-2
#...connects to "bar-rds-us-west-2-123456789012" by prefix...

# or connect by label(s) using --labels
$ tsh db connect --db-user=alice --db-name-postgres --labels region=us-west-2
#...connects to "bar-rds-us-west-2-123456789012" by matching region label...

# or connect by label(s) in a --query predicate
$ tsh db connect --db-user=alice --db-name-postgres --query 'labels.region == "us-west-2"'
#...connects to "bar-rds-us-west-2-123456789012" by matching region label...

# ambiguous label(s) match is also an error
$ tsh db connect --db-user=alice --db-name-postgres --query 'labels.region == "us-west-1"'
error: ambiguous database query matches multiple databases:
Name                           Description               Protocol Type URI                                                   Allowed Users Labels                                                                                                                                    Connect 
------------------------------ ------------------------- -------- ---- ----------------------------------------------------- ------------- ----------------------------------------------------------------------------------------------------------------------------------------- ------- 
bar-rds-us-west-1-123456789012 RDS instance in us-west-1 postgres rds  bar.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432 [*]           account-id=123456789012,endpoint-type=instance,engine-version=13.10,engine=postgres,env=dev,region=us-west-1,teleport.dev/origin=dynamic          
foo-rds-us-west-1-123456789012 RDS instance in us-west-1 postgres rds  foo.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432 [*]           account-id=123456789012,endpoint-type=instance,engine-version=13.10,engine=postgres,env=prod,region=us-west-1,teleport.dev/origin=dynamic         

Hint: try addressing the database by its full name or by matching its labels (ex: tsh db connect key1=value1,key2=value2).
Hint: use `tsh db ls -v` or `tsh db ls --format=[yaml | json]` to list all databases with verbose details.

# resolve the error by using either more specific labels or adding a prefix name
$ tsh db connect --db-user=alice --db-name-postgres --query 'labels.region == "us-west-1"' foo
#...connects to "foo-rds-us-west-1-123456789012" by prefix and label...
$ tsh db connect --db-user=alice --db-name-postgres --query 'labels.region == "us-west-1" && labels.env == "prod"'
#...connects to "foo-rds-us-west-1-123456789012" by multiple labels...
```

### Web UI and Teleport Connect UX

Both the web UI and Teleport Connect already support searching for substrings
in resource names and labels.

Searching by substring is a "fuzzier" kind of search than prefix-based name
search (like this RFD proposed prefix-based search for `tsh`) - it's more
likely to match more than one resource.
However, GUI UX is fundamentally different from CLI - users can search and then
interactively select from multiple matching resources.
So this kind of search is appropriate for the web UI and Teleport Connect, but
not for `tsh`.

Both web UI and Teleport Connect also support label-based searching with
the predicate language, e.g.:

```
labels["env"] == "dev" && labels["region"] == "us-west-1"
```

Therefore, no UX changes are required for these user interfaces.

### Security

No security concerns I can think of.

### Backward Compatibility


If the Teleport Discovery service is upgraded, but `tsh` is not, then
we may break backwards compatibility with user automation scripts, and/or
frustrate users with long names they must type fully, since their `tsh` does
not have the UX improvements.

Solution: backport `tsh` UX changes to prior versions and reserve changes to 
the Teleport Discovery naming schema for v14.
This way users can continue to type the old names of discovered resources and
connect by prefix match.

`tsh` UX changes will add a new predicate expression `hasPrefix` to the
server-side predicate resource parser.
If a user has a newer `tsh` version than the server, then `hasPrefix` may not
be supported by the server and `tsh` will get an error.
To avoid issues, we can make `tsh` fallback to listing resources without a
predicate expression and filter the results by matching prefix name.

### Audit Events

N/A

### Test Plan

We should test that discovering multiple resources with identical names does not
suffer name collisions.

Setup identically named RDS databases and kube clusters in different AWS regions
and a discovery agent to discover them.

Check that the resources in each region are discovered and differentiated by
region in their name.

