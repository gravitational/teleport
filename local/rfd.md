---
authors: Aleksei Kozadaev (aleksei.kozadaev@goteleport.com)
state: draft
---

# RFD 0324 - Scheduled access requests

## Required Approvers

- Engineering: @r0mant && @smallinsky
- Security: rjones

## What

Introduce an opt-in scheduled timing mode for Access Requests. This mode allows a user to request a fixed access window that starts at an arbitrary future time without counting the delay before that window against `request.max_duration`.

## Why

The interaction between `start date`, `access duration`, and request TTL is confusing because the current `max_duration` calculation starts when the request is created. There is no way to schedule the request at an arbitrary future point while independently limiting its effective duration, for example, "I want the request to be valid for eight hours starting at 08:00 next Friday." Configuring a long `max_duration` to reach the future start also creates a long access window.

### Use Cases
Users need to perform maintenance during an upcoming weekend, when approvers are typically offline. They need to file a request early enough for review, while ensuring that approved access cannot begin before the scheduled maintenance window or continue after its fixed end.

TO BE EXTENDED TO ACTUAL USE CASES WITH UX WORKFLOWS.

### Additional considerations

Currently, the access-expiry calculation starts when a request is raised. Time spent waiting for approval or for a future start reduces the usable access window and can cause the request to expire before the planned work begins.

## Details

### Current model

Currently, scheduled access is implemented by adding an earliest assumption time to an otherwise normal Access Request. The request’s expiry is still calculated from when the request was created, using `request.max_duration`. The scheduled start only prevents assumption before that time; it does not move the expiry. As a result, scheduling access further into the future consumes part of the available access window. Approval delays have the same effect.

The request also has a separate TTL controlling how long it can remain pending for approval, currently capped at seven days and constrained by the access expiry. Once approved and past its scheduled start, the request can be assumed repeatedly until it expires. Individual sessions are limited by role and certificate TTLs, but ending one elevated session does not consume the request or prevent it from being assumed again.

Current data structure for the AccessRequestSpecV3 (omitting non-relevant fields)

```go
// AccessRequestSpec is the specification for AccessRequest
type AccessRequestSpecV3 struct {
	// Created encodes the time at which the request was registered with the auth
	// server.
	Created time.Time `protobuf:"bytes,4,opt,name=Created,proto3,stdtime" json:"created,omitempty"`

	// Expires constrains the maximum lifetime of any login session for which this
	// request is active.
	Expires time.Time `protobuf:"bytes,5,opt,name=Expires,proto3,stdtime" json:"expires,omitempty"`

    ...

	// MaxDuration indicates how long the access should be granted for.
	MaxDuration time.Time `protobuf:"bytes,17,opt,name=MaxDuration,proto3,stdtime" json:"max_duration,omitempty"`

	// SessionTLL indicated how long a certificate for a session should be valid for.
	SessionTTL time.Time `protobuf:"bytes,18,opt,name=SessionTTL,proto3,stdtime" json:"session_ttl,omitempty"`

	// AssumeStartTime is the time the requested roles can be assumed.
	AssumeStartTime *time.Time `protobuf:"bytes,21,opt,name=AssumeStartTime,proto3,stdtime" json:"assume_start_time,omitempty"`

	// ResourceExpiry is the time at which the access request resource will expire.
	ResourceExpiry *time.Time `protobuf:"bytes,22,opt,name=ResourceExpiry,proto3,stdtime" json:"expiry,omitempty"`

    ...
}
```

The flow is as follows:

At creation, the server sets `created` to its current time. Client-provided `max_duration` is actually an absolute timestamp, so the server converts it back into a duration by subtracting `created`, then applies the request-role limit and the global 14-day limit. The resulting value is stored as an absolute timestamp in both `max_duration` and `expires`:


```text
access_duration = effective(requested duration, role request.max_duration, 14-day limit)
max_duration = created + access_duration
expires      = created + access_duration
```

The per-session limit is calculated separately from the requester’s remaining login lifetime, the requested session TTL, and the requested roles’ `max_session_ttl`. Despite its name, `session_ttl` is also stored as an absolute timestamp:

```text
session_duration = effective session/certificate duration
session_ttl      = created + min(session_duration, access_duration)
```

`assume_start_time` is an optional lower bound within that fixed creation-to-expiry interval. It does not participate in calculating `expires`. Therefore, scheduling the start later reduces the usable interval:

```text
usable interval = [assume_start_time, expires]
```

While the request is pending, `resource_expiry` represents its approval deadline. It defaults to creation plus up to seven days, but is capped by `expires`. During approval, the request state becomes approved and `resource_expiry` is normally changed to `expires`. The server does not recalculate `max_duration`, `expires`, or `session_ttl` from the approval time. A reviewer may replace `assume_start_time`, but the end remains unchanged. Once approved, the request can be assumed repeatedly between the start and expiry, with each issued session separately capped by the session and access-expiry limits.

### Desired model

#### Scope
This RFD retains the current workflow for backward compatibility and adds one new workflow: a fixed scheduled mode. Approval-relative floating windows and single-activation Access Requests are out of scope.

For fixed scheduled mode:

- The requester supplies an absolute start and a requested duration.
- The delay between request creation and the scheduled start does not count against the access duration.
- The effective duration is constrained by applicable role `request.max_duration` limits and the global maximum access duration.
- The fixed end is the scheduled start plus the effective duration.
- Approval never moves the start or end.
- Approval before the start preserves the complete window.
- Approval after the start but before the end leaves only the remainder of the window (or as another option the start of the window can be implicit approval deadline).
- Approval at or after the end is rejected.
- Session TTL calculation and credential issuance remain unchanged, with credentials still capped by the fixed access end.
- An approved request remains reusable within the fixed window, matching current Access Request behavior.

### Proposal

#### Requirements
1. Retain the current workflow for backward compatibility.
2. Gate scheduled timing behind a feature flag.
3. Treat an absent timing context as legacy timing.
4. When the cluster does not advertise scheduled timing, clients omit the timing context and continue using legacy timing.
5. Keep single activation out of scope; scheduled requests remain reusable until the fixed end.

#### Updated schema
```proto
// AccessRequestSpec is the specification for AccessRequest
message AccessRequestSpecV3 {
  ...
  < existing 26 fields >
  ...

  // Timing defines the requested timing semantics for this
  // Access Request. If unset, the Access Request uses legacy timing semantics.
  AccessRequestTiming Timing = 27 [
    (gogoproto.nullable) = true,
    (gogoproto.jsontag) = "timing,omitempty"
  ];
}

// AccessRequestTiming contains requester-controlled timing configuration. The
// selected oneof field determines how the access window is calculated.
message AccessRequestTiming {
  oneof mode {
    // Scheduled requests a fixed access window. Approval never moves the
    // window.
    AccessRequestScheduledTiming Scheduled = 1 [
      (gogoproto.jsontag) = "scheduled,omitempty"
    ];
  }
}

// AccessRequestScheduledTiming requests a fixed access window.
message AccessRequestScheduledTiming {
  // Start is the fixed beginning of the access window.
  google.protobuf.Timestamp Start = 1 [
    (gogoproto.stdtime) = true,
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "start"
  ];

  // Duration is the requested access-window duration.
  google.protobuf.Duration Duration = 2 [
    (gogoproto.stdduration) = true,
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "duration"
  ];
}
```

#### Timing calculation

For a scheduled request, the timing context is authoritative:

```text
start = timing.scheduled.start
effective_duration = min(
    timing.scheduled.duration,
    applicable role request.max_duration limits,
    global maximum access duration,
)
end = start + effective_duration
```

The server populates the existing fields as compatibility projections:

```text
assume_start_time = start
max_duration = end
expires = end
```

The server must reject non-positive durations, timestamp overflow, conflicting client-provided legacy fields, and a computed end that is not after the start. The existing seven-day `assume_start_time` boundary does not apply to scheduled timing because the delay before the start is independent of the access duration.

While the request is pending, the existing resource expiry remains its review deadline. On approval, resource expiry is moved to the fixed end. Approval must fail if either the pending request has expired or the fixed end has passed.

Reviewer APIs that only override `assume_start_time` must not modify a scheduled timing context. Supporting reviewer changes to a scheduled window would require replacing and revalidating the complete timing context atomically and is out of scope.

#### Approval and expiry concurrency

The Auth server must use its own clock to validate the pending review deadline and fixed end during every approval path, including review-based approval, direct state updates, and privileged creation of an already-approved request. These checks must occur in the same revision-conditional update that changes the request state. Asynchronous cleanup is not sufficient because an expired pending request can remain in the backend until the next expiry sweep.

Approval changes resource expiry from the pending review deadline to the fixed end. The expiry service must not delete a request using a stale view that predates this transition. Deletion must assert the request revision and expiry observed by the sweep, or reload and revalidate the request immediately before deletion. A compare failure causes the expiry service to leave the request in place for re-evaluation.

Assumption continues to load the request by ID and revalidate its approved state, user and role eligibility, scheduled start, and fixed end. The timing context is immutable after creation, while the legacy timing fields are server-managed projections and must remain consistent with it.

#### Scheduling window and retention

Scheduled timing does not introduce a product-level limit on how far in the future the fixed window may start. Timestamps must still be valid protocol values, and start-plus-duration arithmetic must be checked for overflow.

A pending request remains subject to the existing review deadline. Once approved, the request is retained until the fixed end, which may be substantially later than its creation time. This increases backend, cache, and listing retention for far-future requests. Existing request quotas and pagination continue to apply, and assumption-time role revalidation prevents a stale approval from bypassing later role-policy changes. Administrators can delete a scheduled request before its window if the approval is no longer appropriate.

#### Compatibility and rollout

An absent timing context retains legacy behavior. Scheduled timing is disabled by default and enabled through a cluster-wide feature flag after all Auth and Proxy instances have been upgraded. Auth enforces the flag when creating scheduled requests, while Web and Connect use it to control whether scheduled timing is offered. Older clients do not send a timing context and continue using legacy timing.

Disabling the flag prevents creation of new scheduled requests but does not change the behavior of existing scheduled requests. Downgrading to a version that does not understand persisted timing contexts is unsupported while scheduled requests remain active. This however depends on whether the feature flag is local to auth instance or global for a cluster. The former may make the upgrades risky if the instances disagree within the cluster.

If necessary a feature advertising and proper compatibility enforcement can be implemented but I keep it out of the scope of this RFC for now.

#### Security considerations

TBD

#### UX

The UX must make scheduled timing an explicit opt-in and must not silently reinterpret existing flags or requests. The following options preserve legacy behavior while mapping to the same scheduled timing context.

#### tsh options.

1. ##### `tsh` option 1: scheduled start with existing duration

    Add a new `--scheduled-start` flag and continue using `--max-duration` as the requested duration:

    ```bash
    tsh request create \
      --roles=maintenance \
      --scheduled-start=2026-08-16T08:00:00Z \
      --max-duration=4h
    ```

    The presence of `--scheduled-start` selects scheduled timing. It requires `--max-duration` and is mutually exclusive with the legacy `--assume-start-time` flag. Without `--scheduled-start`, `--max-duration` retains its current behavior.

    This option adds fewest CLI modifications and preserves existing automation. Its drawback is that `--max-duration` uses different arithmetic depending on whether `--scheduled-start` is present.

1. ##### `tsh` option 2: explicit timing mode

    Add a mode flag and reuse the existing timing flags:

    ```bash
    tsh request create \
      --roles=maintenance \
      --timing=scheduled \
      --assume-start-time=2026-08-16T08:00:00Z \
      --max-duration=4h
    ```

    The mode makes the arithmetic explicit and can support future timing modes. It is more verbose, retains the internal term `assume-start-time`, and permits more invalid flag combinations that clients must reject.

1. ##### `tsh` option 3: explicit access-window flags

    Add new flags that map directly to the scheduled timing context:

    ```bash
    tsh request create \
      --roles=maintenance \
      --access-start=2026-08-16T08:00:00Z \
      --access-duration=4h
    ```

    This option provides the clearest terminology and does not overload legacy flags. It duplicates `--assume-start-time` and `--max-duration`, increasing the CLI and documentation size and confusion due to seemingly duplicated flags.

1. ##### `tsh` option 4: scheduled subcommand

    Introduce a command dedicated to scheduled requests:

    ```bash
    tsh request create scheduled \
      --roles=maintenance \
      --start=2026-08-16T08:00:00Z \
      --duration=4h
    ```

    This option strongly separates scheduled timing from legacy behavior and leaves room for scheduled-specific arguments. IIt duplicates `--assume-start-time` and `--max-duration`, increasing the CLI and documentation size and confusion due to seemingly duplicated flags.

##### `tsh` output and approval waiting

Regardless of the input option, creation first produces a pending request. `tsh` should display the server-normalized fixed window and make the pending state explicit:

```text
Access Request created: 019...
Status: Pending
Access starts: Sun, Aug 16 2026 08:00 UTC
Access ends:   Sun, Aug 16 2026 12:00 UTC
Access duration: 4h
Approval required within: 7d
```

`tsh request show` should distinguish `Access Starts`, `Access Ends`, `Access Duration`, and the pending approval deadline. Access expiry must not be labeled as session TTL.

Without `--nowait`, `tsh request create` continues watching the pending request for a later approval or denial. The following output is printed only after a reviewer approves the request. When approval occurs before the scheduled start, `tsh` must not attempt credential reissue or continue waiting until a start that may be days away. It should return successfully and print the command used to assume the request later:

```text
Approval received.
Access becomes available at Sun, Aug 16 2026 08:00 UTC.

Assume it at that time with:
  tsh login --request-id=019...
```

If the request remains pending, no approval message is printed and the default command continues waiting. With `--nowait`, `tsh` exits immediately after displaying the pending request. If approval arrives after the start but before the end, `tsh` can immediately reissue credentials using the existing flow.

#### UI options.

1. ##### Web and Connect option 1: timing mode cards

    Present `Standard request` and `Scheduled window` as explicit modes. Selecting `Scheduled window` reveals access start date, start time, duration, computed end, and pending approval duration.

    This option makes the semantic boundary clear, preserves a visible legacy path, and leaves room for future timing modes. It adds an explicit choice before the existing timing fields.

1. ##### Web and Connect option 2: fixed-window toggle

    Add a `Schedule a fixed access window` toggle to the existing form. Enabling it reveals the scheduled start and duration fields.

    This option is compact and works well on small screens. Its drawback is that the distinction between the existing delayed start and the new fixed-window semantics may remain unclear unless the legacy start control is moved or clearly relabeled.

1. ##### Web and Connect option 3: unified start selector

    Present two choices under `Access starts`: `As soon as approved` and `At a scheduled time`. Selecting the latter creates a scheduled timing context.

    This option uses natural terminology and builds on the existing start selector. Its drawback is that `As soon as approved` does not fully describe legacy creation-relative duration arithmetic, and it removes an obvious UI path for the legacy delayed-start behavior.

##### Scheduled request form

All Web and Connect options should show the complete requested window before submission:

```text
Access starts
[Aug 16, 2026] [08:00] [UTC]

Access duration
[4 hours]

Requested access window
Sun, Aug 16, 08:00-12:00 UTC

Approval
Request expires if not reviewed within [7 days]
```

If the window has started, the reviewer should be warned that approval grants only the remainder and does not move the end. At or after the end, approval must be disabled in the client and rejected by the server. Scheduled timing is read-only during review because replacing the complete window is out of scope.

Before the start, an approved request should show when it becomes available and disable the assume action. During the window, it should show the remaining time and enable assumption. After the window, it should show the fixed end and an expired state. Because single activation is out of scope, the UI must not describe scheduled requests as one-time or consumed.

#### Shared UX requirements

- Display the local time and timezone, with UTC available in request details.
- Require timezone-aware input to avoid daylight-saving ambiguity.
- Show the computed fixed end before submission and review.
- Keep the pending approval deadline separate from access-window duration and credential session TTL.
- Do not apply the legacy seven-day start-date limit to scheduled timing.
- Obtain effective duration limits through scheduled dry-run validation rather than deriving them from a creation-relative expiry.
- Hide scheduled timing when the cluster does not advertise support, omit the timing context, and continue offering the existing legacy request workflow.

All UX options produce the same resource representation:

```yaml
spec:
  timing:
    scheduled:
      start: "2026-08-16T08:00:00Z"
      duration: 4h
```

#### Future work

Single activation is orthogonal to scheduled timing and is intentionally deferred. A future proposal can add an independent usage policy and internal, server-owned activation marker without changing the scheduled timing context. Existing scheduled requests would remain reusable for backward compatibility unless they explicitly opt into that future policy.

Approval-relative floating windows are also deferred. They require a separate timing mode and an authoritative server-owned approval timestamp; `assume_start_time` must not be reused as the approval timestamp.

### Open questions
- Grant the remainder after late approval, or require approval before the start?
- Feature flag is local to the Auth instance or cluster-wide? There are possible caviats in case of the former (see compatiblity section for details).

## References
- https://github.com/gravitational/customer-sensitive-requests/issues/539
- https://github.com/gravitational/teleport/issues/46001
- https://github.com/gravitational/teleport/issues/50011
- https://github.com/gravitational/teleport/issues/66504
