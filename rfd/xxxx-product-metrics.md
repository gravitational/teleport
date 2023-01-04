---
authors: Alexander Klizhentas (sasha@goteleport.com), Xin Ding (xin@goteleport.com)
state: draft
---

# RFD 101 - Kubernetes Per-pod RBAC

## Required Approvers

- Engineering: `@espadolini && @michellescripts && @timothyb89`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden`

## What

This RFD defines principles and architecture for product metrics for cloud, self-hosted enterprise and open-source Teleport.

## Why

Teleport's product and engineering team needs to know accurate product usage metrics for billing purposes and to improve product based on user's behavior.
Our target audience is security-minded engineer, and the way we collect metrics should reflect their expectations.
Our metrics and reporting must be built with proper privacy and anonymization protocols in place.

## Metrics Anonymity Invariant

All metrics reported by the Cloud, Self-hosted OSS and Enterprise must be anonymized. As a rule of thumb, if anyone intercepts the metrics stream,
they should not be able to de-anonymize the emails, or any other personally identifiable information.

To achieve that, all data sent as a part of the metrics stream must be anonymized with SHA256 hash of each sensitive field with a randomly 
generated prefix identifying cluster id.

For example,

Anonymized user ID: HMAC SHA256 hash of a username with a randomly generated cluster-id.
Anonymized server ID: HMAC SHA256 hash of a server IP with a randomly generated cluster id.

To make sure customer's of the platform can trust the logic, the anonymization code must be published in OSS and reviewed by independent security auditors.

Current usage reporting code is:

https://github.com/gravitational/teleport/blob/master/lib/services/usagereporter.go

Anonymizer code:

https://github.com/gravitational/teleport/blob/master/lib/utils/anonymizer.go


## Classification of Metrics

Teleport will emit two types of product metrics - product usage metrics and user behavior metrics.
There will be two ways to emit metrics - authenticated metrics and unauthenticated metrics submitted to unauthenticated collection endpoint.

## OSS users promise

We understand that OSS users have strong preferences of freedom, anonymity and privacy. That is why all Teleport's OSS metrics, regardless of their type, will be opt-in.

### User behavior metrics

User behavior metrics help product team to understand most used product features, and help to identify places in the product where users 
struggle. These metrics are always opt-in. All Teleport clients (web, connect, tsh) must always ask user if they are OK to send anonymized usage data.

Here is an example of Teleport Connect's opt-in usage message:

"Are you OK sending anonymized usage data about Teleport Connect? This will help us to improve product".

Unauthenticated metrics are useful to send data directly from clients, e.g. Teleport Connect.
To prevent flooding backend with a large number of small requests, events will be batched before sending to a service `prehog`.

Here are some examples of usage event submitted by Teleport connect:

connect.cluster.login
Successful login to a cluster.

Event properties:

cluster_name: string (anonymized)
user_name: string (anonymized)
connector_type: string
os: string (set once on a user properties)
arch: string (set once on a user properties) - CPU architecture
os_version: string (set on a user properties)
connect_version: string (set on a user properties)
distinct_id: string

### Product usage metrics

Product usage metrics main goal is to unify billing for cloud and self-hosted. These metrics must be send to authenticated `posthog` endpoint from the 
Auth server's side.

These metrics must be anonymized and sent in batches to prevent flooding the backend.

Because these metrics are sent to authenticated endpoint, Teleport's analytics backend will be able to associated these metrics with each individual 
tenant for billing purposes.

These metrics will be mandatory for some self-hosted Enteprise plans, all cloud deployments and will be opt-in for OSS users.


