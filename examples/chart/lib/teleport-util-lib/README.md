# teleport-util-lib

Shared helper templates used by multiple teleport charts.

See `examples/chart/lib/README.md` for the repo-wide library-chart conventions.

## Public templates

### `teleport-util-lib.version`

Resolves the Teleport version. Call with the root context:

```
{{ include "teleport-util-lib.version" . }}
```

Reads:

- `.Values.teleportVersionOverride` — optional explicit override.
- `.Chart.Version` — used when no override is set.

Returns the version string.

### `teleport-util-lib.majorVersion`

Major version number (e.g. `19`) derived from `teleport-util-lib.version`. Call
with the root context; same inputs as above.

```
{{ include "teleport-util-lib.majorVersion" . }}
```

### `teleport-util-lib.resource-quantity`

Parses a Kubernetes resource quantity into a plain number. Unlike the templates
above it takes the quantity value itself, not the root context:

```
{{ include "teleport-util-lib.resource-quantity" "10.5Gi" }}
```

Accepts IEC (`Ki`/`Mi`/`Gi`/…), SI (`k`/`M`/`G`/…) and decimal/scientific
notation.

### `teleport-util-lib.gomemlimit`

Computes a `GOMEMLIMIT` byte value from a container's resources. Takes a
values-shaped dict, not the root context:

```
{{ include "teleport-util-lib.gomemlimit" .Values }}
```

Reads, from the dict passed in:

- `.resources` — standard k8s container resources; `.limits.memory` is used.
- `.goMemLimitRatio` — ratio of `GOMEMLIMIT` to the memory limit.
- `.extraEnv` — if it already defines a `GOMEMLIMIT` entry, returns `""` (the
  explicit value wins).

Returns the computed limit in bytes, or `""` when it should not be set (no
ratio, no memory limit, or already set via `extraEnv`).
