---
authors: Vitor Enes (vitor@goteleport.com)
state: draft
---

# RFD 0076 - Track Monthly Active Users

## Required Approvals

* Engineering: @r0mant && @jimbishopp
* Product: @xinding33 || @klizhentas

## Table of Contents

* [What](#what)
* [Why](#why)
  * [Goals](#goals)
  * [Non\-Goals](#non-goals)
* [Details](#details)
  * [Detect user activity](#detect-user-activity)
    * [Extending Teleport](#extending-teleport)
  * [Track active users in the backend](#track-active-users-in-the-backend)
  * [Export metric to Prometheus](#export-metric-to-prometheus)
  * [Concerns and open questions](#concerns-and-open-questions)
* [Alternatives considered](#alternatives-considered)

## What

This RFD proposes a way to extend Teleport so that the number of monthly active users can be tracked.
In summary, this RFD proposes that:
- audit events are intercepted when they're being emitted
- the user is extracted from each event intercepted and written to a backend key `metrics/active_users/USER` with a 30-day TTL
- periodically, a Prometheus gauge is updated with number of active users in the last 30 days (i.e. the number of keys with the `metrics/active_users/` prefix)

## Why

The Cloud team wants to start tracking the number of monthly active users.
This is needed not only to help us understand the usage and growth of Teleport Cloud, but also potentially for displaying a warning on `tsh`, `tctl` and `WebUI` when the license is out of compliance ([#1673](https://github.com/gravitational/cloud/issues/1673)).

### Goals

* Track the number of monthly active users in Teleport

### Non-Goals

* Specify how the number of monthly active users is going to be reported and visualized (one idea is to have this in a Grafana dashboard)

## Details

In this section we detail how [open-source Teleport](https://github.com/gravitational/teleport) and [Teleport Enterprise](https://github.com/gravitational/teleport.e) can be extended to achieve [our goal](#goals):
- [Detect user activity](#detect-user-activity) explains how we determine that a user is active
- [Track active users in the backend](#track-active-users-in-the-backend) explains how the backend can be used to track the number of active users in the last 30 days
- [Export metric to Prometheus](#export-metric-to-prometheus) proposes a simple mechanism for this number to be exported to Prometheus

### Detect user activity

In order to determine when a user is active, we intercept audit events when they're being emitted to the audit log.
This occurs when the [`IAuditLog.EmitAuditEvent`](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/lib/events/api.go#L683-L688) function is called:

```go
type IAuditLog interface {
    // EmitAuditEvent emits audit event
    EmitAuditEvent(context.Context, apievents.AuditEvent) error

    // ...
}
```

Teleport Enterprise already [wraps the audit log](https://github.com/gravitational/teleport.e/blob/21b2440ecd6ef64755785cc26a38658787b53ec7/lib/pro/auditlog.go#L28-L36), so open-source Teleport does not have to be extended for us to intercept the event in Teleport Enterprise:
```go
// EmitAuditEvent emits the specified event.
func (l *AuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return trace.Wrap(l.Inner.EmitAuditEvent(ctx, event))
}
```

After intercepting the audit event, we extract from the event the user responsible for it.
With the exception of the events `AppSessionRequest`, `CertificateCreate`, `DesktopRecording`, `SessionPrint`, `SessionUpload` and `SessionConnect`, [all events](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/api/types/events/events.proto) have a [`UserMetadata`](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/api/types/events/events.proto#L58-L61) containing a `User` field:
```protobuf
// UserMetadata is a common user event metadata
message UserMetadata {
    // User is teleport user name
    string User = 1 [ (gogoproto.jsontag) = "user,omitempty" ];

    // ...
}
```

Note that any user that produces an event with `UserMetadata` is considered an active user.

For us to extract the user from the event, Teleport has to be extended with a `UserMetadataGetter` interface (similar e.g. to the [`SessionMetadataGetter`](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/lib/events/api.go#L577-L582)):
```go
// GetUser returns event user
func (m *UserMetadata) GetUser() string {
	return m.User
}

// UsersMetadataGetter represents interface
// that provides information about the user
type UserMetadataGetter interface {
	// GetUser returns the event user
	GetUser() string
}

// GetUser pulls the user from the events that have a UserMetadata.
// For other events an empty string is returned.
func GetUser(event events.AuditEvent) string {
	var user string

	if g, ok := event.(UserMetadataGetter); ok {
		user = g.GetUser()
	}

	return user
}
```

With this, we can modify the `EmitAuditEvent` in Teleport Enterprise so that it extracts the user and records it as active (the `ActiveUsers` struct below is detailed in the [next subsection](#track-active-users-in-the-backend)):
```go
// EmitAuditEvent emits the specified event.
func (l *AuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	l.setUserAsActive(ctx, event)
	return trace.Wrap(l.Inner.EmitAuditEvent(ctx, event))
}

// setUserAsActive tries to extract a user from the event and sets it as active in case it's found.
func (l *AuditLog) setUserAsActive(ctx context.Context, event apievents.AuditEvent) {
	user := events.GetUser(event)
	if user != "" {
		err := l.ActiveUsers.SetAsActive(ctx, user)
		l.Log.WithError(err).Error("Failed to set user as active.")
	}
}
```

As we detail in the [next subsection](#track-active-users-in-the-backend), `ActiveUsers.SetAsActive` makes a call to the backend, slowing down this codepath responsible for emitting audit events.
If performance is a concern here, we can call `ActiveUsers.SetAsActive` asynchronously.

### Track active users in the backend

In order to track and count the number of active users, we provide an `ActiveUsers` struct with two methods: `SetAsActive` and `Count`.

The `SetAsActive` method receives the `user` and upserts the key `metrics/active_users/USER` into the backend using [`Backend.Put`](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/lib/backend/backend.go#L46-L48) with a 30-day TTL.
Independently of whether the user has been active in the last 30 days or not, this ensures that the user is considered as active during the next 30 days.

```go
// SetAsActive registers a user as active
func (a *ActiveUsers) SetAsActive(ctx context.Context, user string) error {
	_, err := a.Backend.Put(ctx, a.backendItem(user))
	return trace.Wrap(err)
}

func (a *ActiveUsers) backendItem(user string) backend.Item {
	// item expires in 30 days
	ttl := 30 * 24 * time.Hour
	now := a.Clock.Now().UTC()
	return backend.Item{
		Key:     activeUsersKey(user),
		Value:   []byte(now.Format(time.RFC3339)),
		Expires: now.Add(ttl),
	}
}

func activeUsersKey(user string) []byte {
	return backend.Key("metrics", "active_users", user)
}
```

The `ActiveUsers` struct also provides a `Count` method that computes the number of active users during the last 30 days.
This method leverages the [`Backend.GetRange`](https://github.com/gravitational/teleport/blob/8a27614b83590056e0d43394b926cf6db29b190b/lib/backend/backend.go#L60-L61) method to count the number of keys with the `metrics/active_users` prefix.

```go
// Count counts the number of active users during the last 30 days
func (a *ActiveUsers) Count(ctx context.Context) (int, error) {
	startKey := activeUsersPrefix()
	result, err := a.Backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return len(result.Items), nil
}

func activeUsersPrefix() []byte {
	return backend.Key("metrics", "active_users")
}
```

Note that we store the current time in `backend.Item.Value` when upserting.
This allows us to later extend the mechanism proposed in this RFD to also track the number of active users during e.g. the last day or last week (by filtering the `backend.Item`s returned by `Backend.GetRange` based on their `Value`).

### Export metric to Prometheus

A exporter task will be spawn in order to periodically update the following [Prometheus gauge](https://prometheus.io/docs/concepts/metric_types/#gauge) with the result of `ActiveUsers.Count`:
```go
metric := prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "active_users",
		Help: "Number of active users during window",
	},
	[]string{"window"},
)
```

The above allows us to extend the mechanism proposed in this RFD to also track the number of active users during different time windows (e.g. the last day or last week) by setting different label values:
```go
metric.WithLabelValues("7d").Set(count)
```

### Concerns and open questions

- Is `UserMetadata.User` the correct identifier to be used?
- After intercepting the audit event, should `ActiveUsers.SetAsActive` be called asynchronously?
- How frequently should the Prometheus gauge be updated?
- Should this feature be solely implemented in [open-source Teleport](https://github.com/gravitational/teleport)?

## Alternatives considered
- This alternative doesn't seem to be feasible, but it would be nice to implement this feature solely using Prometheus.
However, Prometheus metrics don't have a TTL attached to them.
- There's already a [usage reporter](https://github.com/gravitational/teleport.e/blob/21b2440ecd6ef64755785cc26a38658787b53ec7/lib/cloud/usagereporter/reporter.go) that periodically reports usage (counts of users, servers, databases, applications, kubernetes clusters, roles and auth connectors) to the Sales Center.
One alternative to the one suggested here would be to also include in this report the number of active users in the last 30 days.
However, afterwards it would be necessary for our reporting tool (e.g. Grafana) to be given access to the Sales Center. We might want to do this the other way around and instead start tracking all these metrics using Prometheus.