---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD 16 - Dynamic Configuration

## What

This RFD presents several possible scenarios involving the interaction of
explicitly-managed dynamic and static configuration.  Design decisions are made
as to which branches of these scenarios are to be supported by Teleport.
Finally, the actual implementation is discussed.

## Why

Some resources like `types.AuthPreference` or `services.ClusterConfig` are
live representations of parts of the `auth_service` configuration section.
These resources can be understood to constitute *dynamic* configuration of
an auth server, as opposed to *static* configuration defined by the
`teleport.yaml` file.

The current behaviour is that Teleport creates dynamic configuration implicitly
by deriving it from static configuration during auth server initialization.
This RFD allows the dynamic configuration to be created/updated explicitly via
`tctl`, independently of static configuration, for two main reasons:

1. To faciliate automated/programmatic management of Teleport clusters.

2. To add configuration capability to Teleport-as-a-service offerings that do
   not allow direct access to `teleport.yaml` files.

## Scenarios

We use the example of the `authentication` subsection of `auth_service`
in `teleport.yaml` (as "the" static configuration) and the corresponding
resource `types.AuthPreference` (as "the" dynamic configuration).
The latter is assumed to be manipulable via `tctl` using the identifier `cap`.

### Scenario 1: static configuration specified

1. The `authentication` section is specified in `teleport.yaml`.
   The backend can have any `AuthPreference` data stored from the last
   auth server run.

2. The auth server initialization procedure MUST commit data derived from the
   static configuration to the backend, overwriting any potential resource data
   already stored there.

3. `tctl get cap` returns the resource data in line with `teleport.yaml`.

Therefore, static configuration is always given precedence.  This should also
ensure backward compatibility with the already established workflows.

### Scenario 2: static never specified & dynamic not updated by user

1. The `authentication` section has never been specified in `teleport.yaml`.
   No user changes have been made to the dynamic configuration.

2. The auth server initialization procedure MAY or MAY NOT commit the default
   settings to the backend (in the latter case a structure holding the default
   settings is returned in place of the missing resource when the resource
   is requested).

#### Choice 2.A

In this option, dynamic-configuration resources are understood to exist only
if they have been committed as a result of having been specified in static
configuration or via `tctl create`.

3. The command `tctl get cap` would therefore return an error saying
   "authentication preference not found".  To view the default state, `tctl
   get` would have to be invoked with a new CLI flag.

#### Choice 2.B

3. `tctl get cap` returns the resource data corresponding to the default
   settings.

### Scenario 3: static reverted to unspecified & dynamic not updated by user

1. The `authentication` section used to be specified in `teleport.yaml` but
   it has been removed and left unspecified since the last auth server init.
   No user changes have been made to the dynamic configuration.

#### Choice 3.A

2. The auth server initialization procedure retains the last stored dynamic
   configuration which had been derived from the last specified static
   configuration.  In particular, the backend MUST NOT be overwritten
   with the default settings.

3. `tctl get cap` returns the resource data corresponding to the last specified
   static configuration.

#### Choice 3.B

2. The auth server initialization procedure MUST NOT retain the last stored
   dynamic configuration.  The resource data are either overwritten with the
   default settings or deleted (in the latter case a structure holding the
   default settings is returned in place of the missing resource when the
   resource is requested).

3. `tctl get cap` then behaves as in Scenario 2.

### Scenario 4: static never specified & first `tctl create`

1. The `authentication` section has never been specified in `teleport.yaml`.
   No user changes have been made to the dynamic configuration.

2. A user issues the command `tctl create authpref.yaml` with valid
   authentication preferences stored in the YAML file.
   (Issuing `tctl create -f authpref.yaml` would have the same effect.)

3. The dynamic configuration is updated: the resource data read from
   `authpref.yaml` MUST be written to the backend (potentially overwriting
   the default data already stored there).

4. The command returns with success.

5. Upon the next auth server restart with `authentication` still unspecified
   in `teleport.yaml`, the auth server initialization procedure retains
   the last stored dynamic configuration, i.e. the one from `authpref.yaml`.

6. `tctl get cap` returns the resource data corresponding to `authpref.yaml`.

### Scenario 5: static never specified & repeated `tctl create`

1. The `authentication` section has never been specified in `teleport.yaml`.
   A user has already made explicit changes to the dynamic configuration
   using `tctl create`.

2. A user issues the command `tctl create authpref.yaml` with valid
   authentication preferences stored in the YAML file.

#### Choice 5.A

3. The command is rejected with an "already exists" error, recommending
   to use the `-f`/`--force` flag to overwrite the already existing
   `AuthPreference` resource.

#### Choice 5.B

3. The command is accepted and returns with success.

### Scenario 6: static specified & `tctl create` attempt

1. The `authentication` section is specified in `teleport.yaml`.
   The auth server is running with static configuration as in Scenario 1.

2. `tctl create authpref.yaml` returns with an error since the command
   cannot be used to overwrite non-default configuration.

#### Choice 6.A

3. `tctl create -f authpref.yaml` allows the user to temporarily set the
   dynamic configuration until it gets replaced by the static configuration
   during a restart as in Scenario 1.  In case the static configuration section
   is removed before the restart, the dynamic configuration corresponding to
   `authpref.yaml` is retained.

#### Choice 6.B

3. `tctl create -f authpref.yaml` is rejected as it constitutes a security
   risk and may cause consistency issues in HA environments.

## Configuration source preferred by auth server init

The following table summarily depicts a key aspect of the scenarios elaborated
above, namely the question of which configuration source is to be preferred by the
auth server initialization procedure:

|                        | **dynamic updated by user** | **dynamic not updated by user** |
|          :---:         |            :---:            |              :---:              |
|  **static specified**  |            static           |              static             |
| **static unspecified** |           dynamic           |   defaults OR last static [?]   |

(The state marked [?] depends on the resolution of Scenario 3.)

## Implementation

The resource label `teleport.dev/origin` (below shortened to `origin` for
brevity) is used as the key indicator when determining the configuration
sources and their precedence.

The label can be associated with three values:

1. `defaults`: for hard-coded resource objects consisting of default settings;
2. `config-file`: for resource objects derived from static configuration;
3. `dynamic`: for resource objects created "dynamically" (e.g. via `tctl`).

The following table captures the ordinary means of performing the "fastest"
transition between a pair of `origin` values.  The leftmost column is the
current/source `origin` value of a resource while the top row is the
desired/target `origin` value:

|    *from \ to*    |        **`defaults`**       |           **`config-file`**          |          **`dynamic`**          |
|       :---:       |            :---:            |                 :---:                |              :---:              |
|   **`defaults`**  |             n/a             | specify in `teleport.yaml` & restart |          `tctl create`          |
| **`config-file`** | remove from `teleport.yaml` |  change in `teleport.yaml` & restart | `tctl create --force --confirm` |
|   **`dynamic`**   |          `tctl rm`          | specify in `teleport.yaml` & restart |      `tctl create --force`      |

The `teleport.dev/origin` label is reserved for system use.  Resources derived
from `teleport.yaml` are necessarily labelled either as `defaults` (when the
relevant section is left unspecified) or as `config-file` (when the section is
explicitly specified).  Auth server API handlers automatically disregard the
supplied `origin` value and reset it to `dynamic`.

### Auth server initialization

1. If `teleport.yaml` section is specified, store it as a resource in the
   backend with `origin: config-file`.

2. If `teleport.yaml` section is not specified, attempt to fetch the resource
   currently stored in the backend.
   - If the fetching attempt fails with a not-found error, store the default
     resource in the backend with `origin: defaults`.
   - If the fetching attempt fails for another reason, return the error
     immediately.
   - If the fetched resource has `origin: config-file` or `origin: defaults`,
     store the default resource in the backend with `origin: defaults`.
   - If the fetched resource has another `origin` value, keep the backend as-is.

This logic implies Choices 2.B and 3.B.

### `tctl create`

1. Fetch the resource currently stored in the backend; on error return
   immediately.

2. If called without `--force` and the fetched resource does not have
   `origin: defaults` then print an error:
   ```
   non-default cluster auth preference already exists, use -f or --force flag
   to overwrite
   ```

3. If called with `--force` and the fetched resource does not have `origin:
   config-file` then store the resource specified in the provided YAML file
   in the backend.

4. If called with `--force` and the fetched resource has `origin: config-file`
   then print an error:
   ```
   This resource is managed by static configuration. We recommend removing
   configuration from teleport.yaml, restarting the servers and trying this
   command again.

   If you would still like to proceed, re-run the command with both --force and
   --confirm flags.
   ```

   Invocation with both `--force` and `--confirm` will replace the
   stored resource with the YAML one until the auth server restart.

This logic implies Choice 5.A and an adaptation of Choice 6.A.

### `tctl rm`

The `tctl rm` subcommand can be used to reset dynamic resources back to their defaults.

1. Fetch the resource currently stored in the backend; on error return
   immediately.

2. If the fetched resource does not have `origin: config-file`, replace the
   stored resource with the default one.

3. If the fetched resource has `origin: config-file` then print an error:
   ```
   This resource is managed by static configuration. We recommend removing
   configuration from teleport.yaml and restarting the servers in order to
   reset the resource to its default.
   ```

### RBAC verbs

Reading any configuration resource requires the RBAC verb `read`.

Performing any dynamic overwrite of a configuration resource requires the RBAC
verb `update`.  In addition, if the overwrite is to replace a resource with
`origin: config-file`, the RBAC verb `create` is also required.

|                                     |     **`read`**     |    **`update`**    |    **`create`**    |
|                :---:                |        :---:       |        :---:       |        :---:       |
|            **`tctl get`**           | :heavy_check_mark: |                    |                    |
|          **`tctl create`**          |                    | :heavy_check_mark: |                    |
|      **`tctl create --force`**      |                    | :heavy_check_mark: |                    |
| **`tctl create --force --confirm`** |                    | :heavy_check_mark: | :heavy_check_mark: |
|            **`tctl rm`**            |                    | :heavy_check_mark: |                    |
