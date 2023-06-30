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

The disadvantages here:

- Manually examining the backend is more difficult as the stored values will no longer be
  human readable.
- For support cases we will occasionally need to perform surgery in the backend, which
  requires manual editing of objects. This will be significantly more difficult with the
  backend serialized in the wire format.

##### Write custom storage objects for the backend

Rather than use gRPC's wire format or serialized JSON format, we should consider writing
a storage object format. This has a number of benefits:

- The backend continues to be easy to read and modify.
- We decouple our protobuf definitions from our storage representation, which makes changes
  to the frontend less impactful.
- This can be done without making any changes to the proto definitions.
- It gives us more control over storage representation.
- It makes migration easier, as we can easily maintain multiple versions of an object within
  the Teleport codebase.

The downsides:

- It adds to the boilerplate necessary for an object.

##### Write custom marshalers for objects

In order to keep our user facing representations of the objects, we will need to write
custom JSON and YAML marshalers for our objects. This may be time intensive but not overall
a significant amount of work.

#### Embedding types (removing `gogoproto.embed`)

We heavily use `gogoproto.embed` for generating objects with a consistent structure or interface.
We particularly use these with our `kind`, `version`, and `metadata` fields. This will be
difficult to mitigate without some heavy lifting. As such, I recommend the custom storage object
format listed above and the addition of `FromV*` functions in order to convert protobuf objects
into this object. For example:

```go
  accessRequest := accessrequest.FromV1Proto(request.AccessRequest)
  user := user.FromV6Proto(request.User)
```

#### Casting types (removing `gogoproto.casttype`)

We additionally rely heavily on `gogoproto.casttype`, which allows us to avoid doing explicit
casts or conversions within go code. We will have to take on this conversion work, unfortunately.

#### Other gogoproto tags

##### `gogoproto.nullable`

We will need to remove the `gogoproto.nullable` tags, which overall shouldn't be a huge impact.
We will need to rely on our `CheckAndSetDefaults` functions, which should generally handle this
today.

##### `gogoproto.stdtime`

We will need to migrate to using `timestamppb` directly instead of relying on gogo's `stdtime` tag.

### Proposed solution

#### Create internal objects

Currently, in `api/types` we define a large number of interfaces that are subsequently used to wrap
our protobuf created messages in a nicer, more user friendly way, abstracting away much of the protobuf
bits and pieces.

We should break this relationship and create actual internal objects to replace interfaces. From here, we
should use conversion functions to translate between protobuf messages and our internal objects.

It's imperative that these objects are compatible with the current JSON/YAML representations in Teleport.
That is, the objects that are in Teleport today should unmarshal properly into our new internal objects.

All objects should remain in `api/types` as they do today. (Open question: should this be the case? Should
we relocate objects?)

#### Packaging

The following packages should be used for this approach:

| Package name | Description |
|--------------|-------------|
| `api/types`  | The location of the internal objects. |
| `api/gen/proto/go/<proto-pkg>` | The location of generated protobuf messages. |
| `api/convert/proto/go/<proto-pkg>` | The location of conversion functions for protobuf messages into internal types.

##### Example

Let's take `UserGroup` as an example, as it's a very simple message. `UserGroup`'s interface as it is
currently defined in `api/types`:

```go
// UserGroup specifies an externally sourced group.
type UserGroup interface {
	ResourceWithLabels

	// GetApplications will return a list of application IDs associated with the user group.
	GetApplications() []string
	// SetApplications will set the list of application IDs associated with the user group.
	SetApplications([]string)
}
```

For this message, I propose we replace the above interface with a struct:

```go
type UserGroup struct {
  // ResourceHeader will contain the version, kind, and metadata fields.
  ResourceHeader

  Spec *userGroupSpec `json:"spec" yaml:"spec"`
}

func NewUserGroup(metadata Metadata, spec Spec) (*UserGroup, error) {
  userGroup := &UserGroup{
    ResourceHeader: &ResourceHeader{
      Metadata: metadata
    }
    Spec: spec,
  }

  if err := userGroup.CheckAndSetDefaults(); err != nil {
    return trace.Wrap(err)
  }

  return userGroup, nil
}

func (u *UserGroup) CheckAndSetDefaults() error {
  ...
}

func (u *UserGroup) GetApplications() string {
  return u.Spec.Applications
}

func (u *UserGroup) SetApplications(applications []string) {
  u.Spec.Applications = applications
}
...
```

To go between the protobuf message and the internal implementation, we should have:

```go
func FromV1(msg usergroupv1.UserGroupV1) (types.UserGroup, error) {
  return NewUserGroup(types.Metadata{
    // Convert metadata object here
    ...
  }, &UserGroupSpec{
    // Convert user group spec here
    ...
  })
}
```

##### Strategy

We would need to do this object by object, adding in tests to ensure that marshaling
is not broken between the current messages and the new structs.

I recommend the following path:

1. Create the new implementation object as described above.
2. Write tests that verify that the current messages marshal/unmarshal properly into
   the new object. This will require creating message conversion functions as well.
3. Replace existing references to the object with the new version.

Step 3 may also require modifying `e`, which may momentarily break the build. It is
recommended to have all relevant modifications made and approved in PRs, including in `e`,
before merging anything.

Once these steps are accomplished for all objects, we can then start removing `gogoproto`
extensions from `types.proto`. As they'll no longer have an impact on Teleport's business
logic, the impact should be contained to modifying conversion functions. To handle the new
output of the generated objects.

### UX

There should be no difference in UX between Teleport pre-migration and post-migration. The process
must be transparent to the user.

### Security

As this is primarily a migration of data, there should be no security impacts with this effort.

### Implementation plan

#### Migrate individual objects

We'll need to migrate our individual objects that currently sit in `api/types`. This list is currently
not in any prioritized order.

- AccessRequest
- AppServer
- Application
- AuthPreference
- CertAuthority
- ClusterAssert
- ClusterAuditConfig
- ClusterMaintenanceConfig
- ClusterName
- ClusterNetworkingConfig
- ConnectionDiagnostic
- Database
- DatabaseServer
- DatabaseService
- Device
- GithubConnector
- HeadlessAuthentication
- Installer
- Instance
- Integration
- Jamf
- KeepAlive
- KubernetesCluster
- KubernetesServer
- Lock
- MFADevice
- Metadata
- Namespace
- NetworkRestrictions
- OIDCConnector
- OktaAssignment
- OktaImportRule
- Plugin
- PluginData
- PluginStaticCredentials
- ProvisionToken
- RecoveryCodes
- RemoteCluster
- ResourceHeader
- ResourceID
- ReverseTunnel
- Role
- SAMLConnector
- SAMLIdPServiceProvider
- Semaphore
- Server
- ServerInfo
- SessionRecordingConfig
- SessionTracker
- StaticTokens
- TrustedCluster
- TunnelConnection
- TunnelStrategy
- UIConfig
- User
- UserGroup
- UserToken
- UserTokenSecrets
- WatchStatus
- WebSession
- WindowsDesktopService

#### Migrate events

The objects in `api/proto/teleport/legacy/types/events` must be migrated.

#### Migrate webauthn

The objects in `api/proto/teleport/legacy/types/webauthn` must be migrated.