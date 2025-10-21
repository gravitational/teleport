---
Authors: Maxim Dietz (maxim.dietz@goteleport.com)
State: draft
---

# RFD 230 - Component Feature Advertisement

## Required Approvers
- Engineering: (@r0mant && @rosstimothy)

## Related
- [RFD 228](https://github.com/gravitational/teleport/blob/master/rfd/0228-resource-scoped-constraints-in-access-requests.md) - Resource-Scoped Constraints in Access Requests
- [RFD 225](https://github.com/gravitational/teleport/blob/master/rfd/0225-teleport-extended-support.md) - Teleport Extended Release Support

## What
This RFD proposes a small, versioned feature advertisement payload that any Teleport component can attach to its
existing heartbeat/status message. Consumers such as Proxy, Auth APIs, or the Web UI can read these flags and enable UX
that requires a given capability when every component on the request path advertises support. The initial application is
for App Service and Proxy to signal support for Resource Constraints in certificates (RFD 228), allowing the Web UI to
hide "request specific constraints" flows when unsupported.

> [!IMPORTANT]
> This mechanism is intended as a non-security signaling layer, with its initial use being for UX/API gating and
> coordination between components. It does not replace secure design or fail-closed behavior. All security-relevant
> behavior MUST remain correct in the absence of flags and MUST fail closed. Treat flags as hints for conditional UX or
> workflows and not as authorization signals.

## Why
Relying solely on version numbers to determine feature support is fragile, especially in a distributed system with
multiple components that can be upgraded independently, and where certain features may be backported to older versions.
A small set of Boolean feature flags provides an explicit, composable signal that the UI or other consumers can
intersect across components, avoiding guesswork and making UX conditionality predictable.

## Design
The payload is simple and forward-compatible by design. In v1, features are Boolean. Presence means "supported", absence
means "not supported". Each feature is identified by an append-only enum value so IDs remain stable over time.

```protobuf
enum ComponentFeatureID {
  COMPONENT_FEATURE_ID_UNSPECIFIED           = 0;
  // v1 addition: understands Resource Constraints in certs (RFD 228)
  COMPONENT_FEATURE_RESOURCE_ACCESS_IDS = 1;
}

message ComponentFeatures {
  repeated ComponentFeatureID features = 1;
}
```

Each component implements a small `Features()` helper that returns its current set. A component should advertise a flag
only when the implementation is present in the build and is usable with the current configuration and dependencies.
The heartbeat path attaches this payload wherever heartbeats already flow, and Auth surfaces it alongside the existing
status information so downstream consumers can read it in the same places they read heartbeats today.

On the consumer side, the logic is straightforward. For a target operation, the consumer gathers participating
components (e.g., the Proxy and App Service(s) that will serve an app session) and intersects their feature sets to
determine feature support.

Unknown feature IDs are ignored by consumers (optionally logged once per component identity). Because senders only
include `true` values, the payloads stay compact, and omission naturally encodes "not supported".

### Operator visibility

Auth should surface each component's `ComponentFeatures` via `tctl inventory list` / `tctl inventory get` (for example,
as a comma-separated `features` column and in the JSON/YAML output). This gives operators a simple way to confirm which
agents support a capability and to debug mixed-version rollouts.

## Compatibility
Backwards compatibility is straightforward. Components that do not yet publish a features payload are treated as
supporting no features. Older readers that encounter an unknown enum value simply ignore it, so intersections naturally
degrade to the set of features both sides understand.

Forward compatibility follows the same pattern. New enum values can be appended without affecting older readers and
intersections remain safe. If we ever need non-Boolean data or a different shape, we can introduce a new field within
`ComponentFeatures` (e.g., `FeatureDetails`).

### Feature lifecycle & deprecation
When a feature becomes ubiquitous across all supported releases, we should retire both checks and advertisement on a
predictable schedule:

- Once all supported minors in major `M` implement feature `X`, it is considered ubiquitous.
- In `M+1.0.0`, remove consumer-side checks for `X` (treat `X` as "always present").
- In `M+2.0.0`, stop advertising `X` in heartbeats.

*Example:* If `X` is present in all supported v19 minors, remove checks in `v20.0.0`, and drop advertisement in
`v21.0.0`.

Removing checks first avoids accidental regressions with mixed versions; delaying removal of advertisement by one
additional major gives consumers a full major cycle to detect that `X` is no longer explicitly signaled.

## Caveats and guidance
**No dynamic flags:** Do not use this mechanism to advertise runtime settings or mutable state. A component’s advertised
feature set must be stable for the lifetime of the process (or at least between heartbeats without mid-cycle toggling).

**Not a config/health channel:** Do not encode configuration values, environment health, or per-tenant state as
"features". If richer or non-boolean data is ever needed for coordination, we should introduce a separate, purpose-built
payload or evolve `ComponentFeatures` with a new field that does not change per-request.
