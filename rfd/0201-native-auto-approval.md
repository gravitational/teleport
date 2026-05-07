---
authors: Bernard Kim (bernard@goteleport.com)
state: implemented
---

# RFD 0201 - Native Automatic Review

## Required Approvers
* Engineering: @r0mant && @fheinecke
* Product: @klizhentas && @roraback

## What
This document describes how Teleport will support native automatic reviews for
access requests.

## Why
Currently, Teleport supports automatic reviews of access requests, but support
is limited. Automatic reviews requires a separate plugin, and only a subset of
the plugins support automatic reviews. Requesting users do not wish to
integrate with a separate service just to utilize this feature. Teleport does
also enable users to build their own access request plugins, but users do not
want the responsibility of building and maintaining a separate piece of
software. They believe this should be a supported use case built-in to Teleport.
In order to support a wider range of use cases, Teleport should support
automatic reviews natively.

The initial use case for automatic reviews was to support on-call engineers.
Automatic reviews can be configured to allow on-call engineers to troubleshoot
production issues when access request reviewers are not available.

Automatic reviews also enables teams to enforce zero standing privileges,
while allowing users to get access to their pre approved resources for a limited
period of time. Access lists could be used to achieve similar behavior, but some
users prefer the just-in-time access request flow.

This feature has also been requested for internal use at Teleport. The Tools
team enforces gated access to certain pipeline activity. The team would like the
dev environment to closely mimic the production environment and would like to
enforce the same gated access, but it would reduce a lot of friction if access
requests can be automatically approved in the dev environment.

## Goals
1. Native support for automatic reviews. Integration with external service is
not required.
2. Automatic reviews can be configured to allow users with certain traits to
be automatically approved. For example, all `L1` engineers on team `Cloud` in
location `Seattle` are pre-approved for the access request.
3. Automatic reviews can be configured to approve access to select resources.
For example, the requesting user is pre-approved to access all resources in the
`dev` environment.
4. Automatic reviews can be configured using Access Monitoring Rules.
5. Automatic reviews can be configured using the Teleport Web UI.
6. Automatic review rules are easily reviewable through the Teleport Web UI.
7. User experience is the focus. The feature should be easy to use and configure.
8. Implementation should be compatible with future plugin interface refactoring.
9. Support configuration of automatic reviews using the Terraform provider.
10. Support automatic reviews of access requests created by a Machine ID bot
user.

Note: Access monitoring rules do not currently support access requests by
resources, in any capacity. It will require more than just extending the access
monitoring rule predicate language to support this use case. Goal [3] will be
left out of scope for this RFD. It probably deserves its own separate RFD. As a
workaround to support this use case, automatic review rules can be configured to
approve a role that only has the permission to access resources in a specific
environment. Resources can be mapped to an environment using a label, and
roles can be allowed access to specific resources with a label selector.

Note: Development is in progress to refactor the access plugins and implement a
unified set of interfaces. This will help achieve feature parity across access
plugins. Automatic review support for other plugins is out of scope for this RFD,
but it is something to consider while we decide how to implement native automatic
reviews. This is why [8] is included in the goals list. See
https://github.com/gravitational/teleport/issues/47150 for more details.

Note: Regarding goal [10], if access requests are compatible with bot users,
then automatic reviews will be compatible with bot users. However, access
requests are not quite compatible with bot users right now. The Teleport access
request validation logic uses only the user's "statically assigned" roles to
check if the user is permitted to create an access request.

Deciding how to address this issue is out of scope for this RFD, but some
options that can be considered:
- Grant the bot user "statically assigned" permissions to create access requests.
- Modify the access request validation logic to permit users to create access
requests based on impersonated/dynamically assigned roles.

## User Stories
Some example use cases that should be supported.
- "As a Teleport administrator, I want to be able to grant my team zero standing
access by default, but allow them to get access to low-risk resources whenever
they need. This access flow is needed for compliance reasons."
- "As a Teleport administrator, I want my super-users that have role "superuser"
to get their access requests approved automatically".
- "As an on-call engineer, I want to be able to troubleshoot a production server
on a weekend when approvers are not available."
- "As a Teleport administrator, I want to be able to grant my team zero standing
access by default, but allow them to get access to resources based on the user's
traits, and based on the resource's labels.

## Web UI Access Monitoring Rules
The Teleport Web UI now provides a more user friendly approach to configuring
automatic reviews. Users are now able to navigate to the **Access Requests**
page and configure automatic reviews, similarly to how notification routing is
configured.

A new `Create New Automatic Review Rule` form can now be used to configure rules
for automatic reviews.
- `Access Request Condition` configures the `access_monitoring_rule.spec.condition` field.
  - `Requested Roles` input accepts a set of roles. These roles configure which
    requested roles will be automatically approved.
  - `User Traits` input accepts a map of traits. These are used to match a
    requesting user's Teleport traits.
- `Notifications` optionally configures `access_monitoring_rules.spec.notification`
  field. The initial implementation will not include the notifications section,
  but we'd like to support configuration of both automatic reviews and notifications
  within the same rule.

![create-amr](assets/0201-create-amr.png)

Note: We have considered using one single form for configuring both
notification rules and automatic review rules, but the match conditions for
notification rules cannot be applied to automatic review rules. Teleport must
enforce a stricter match condition for automatic review rules compared to
notification rules.

The submitted form will be converted into an access monitoring rule that would
look like the following yaml. More details about the access monitoring rule
changes will be provided in following sections.

The `Requested Roles` input is converted into a `contains_all` expression. In this
example, we have the roles "cloud-dev" and "cloud-stage". The set of requested
roles that can match this condition are either `set("cloud-dev")`, `set("cloud-stage")`,
or `set("cloud-dev", "cloud-stage")`.

Note that the converted conditions for requester traits use AND logic across traits, and OR
logic within a trait. The user must match at least one of the values among each
configured trait. For example, given the following converted AMR, a user is
pre-approved if they are any level "L1" or "L2" and they are on the "Cloud" team
and they are located in "Seattle". If the user is level "L1" and located in
"Seattle", but they are on the "Tools" team, they would not be pre-approved. If
the user requires more control over the condition matching, they will need to
edit the access monitoring rule yaml directly.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: dev-pre-approved
spec:
  subjects:
    - access_request
  condition: |-
    contains_all(set("cloud-dev", "cloud-stage"), access_request.spec.roles) &&
    contains_any(user.traits["level"], set("L1", "L2")) &&
    contains_any(user.traits["team"], set("Cloud")) &&
    contains_any(user.traits["location"], set("Seattle"))
  desired_state: reviewed
  notification:
    name: slack
    recipients: ["#dev-cloud"]
  automatic_review:
    integration: builtin
    decision: APPROVED
```

The `Access Monitoring Rules` overview page will be modified to display both
notification rules, as well as automatic review rules. This page will allow
user to see a quick overview of the automatic reviews currently enabled. The
overview simply displays the access monitoring rule name, the type of rule,
and the plugin/integration name. Users will need to click on the **View** button
to see the actual conditions for automatic reviews.

![view-amr](assets/0201-view-amr.png)

## Details
This feature will be supported by a new internal Teleport service. This
automatic review service will function similarly to existing access plugins.
It will be running as part of the Teleport Auth Service by default.

The automatic review service relies on a similar workflow that supports access
request notification routing with access monitoring rules. The service watches
for Access Monitoring Rule (AMR) events and Access Request (AR) events. If an
incoming AR matches an existing AMR condition, then the service will attempt to
automatically review the request.

### Access Monitoring Rule
There are a number of changes required for the `access_monitoring_rule` resource.
- `spec.desired_state` field is added to specify the desired state of the
resource. The only accepted value for now will be `reviewed` indicating that the
access request should be automatically reviewed.
- `spec.automatic_review.integration` field specifies the integration
responsible for monitoring the rule. The initial implementation only supports
the `builtin` value. This indicates that Teleport is responsible for monitoring
the rule.
- `spec.automatic_review.decision` field specifies the proposed state of the
access request review. This can be either `APPROVED` or `DENIED`.

The AMR conditions now also support a `user.traits` variable. This variable
maps a trait name to a set of values. This allows users to specify arbitrary
user traits, such as, "level", "team", "location", etc...  These traits can then
be used to identity whether a user is on-call, or if the user is pre-approved
for the access request.

The AMR conditions now also support the `contains_all(list, items)` operator.
This operator returns true if `list` contains an exact match for all elements of
`items`. This operator allows users to configure a more restrictive condition
for automatic reviews.

```yaml
# This AMR would allow users with traits "level"="L1" and "team"="Cloud" and
# "location"="Seattle" to be pre-approved for the "cloud-dev" role.
kind: access_monitoring_rule
version: v1
metadata:
  name: cloud-dev-pre-approved
spec:
  subjects:
    - access_request
  condition: |-
    contains_all(set("cloud-dev"), access_request.spec.roles) &&
    contains_any(user.traits["level"], set("L1")) &&
    contains_any(user.traits["team"], set("Cloud")) &&
    contains_any(user.traits["location"], set("Seattle"))
  desired_state: reviewed
  automatic_review:
    integration: builtin
    decision: APPROVED
```

### Internal automatic review service
The automatic review service implements the same functionality as the other
access request plugins. When a new AR is observed, the automatic review
service will check if the AR matches any AMR conditions and then attempt to
automatically review the AR. Before checking if the AR matches any AMRs, the
automatic review service makes a request to Teleport, requesting additional
information about the AR user. This info should contain the user traits.

The automatic review flow will look like this:
```mermaid
sequenceDiagram
    participant requester
    participant teleport
    participant automatic_review_service

    requester->>teleport: tsh request create --roles='cloud-dev'
    automatic_review_service->>teleport: watch(AMR, AR)
    automatic_review_service->>teleport: requestUser(requester)
    teleport->>automatic_review_service: responseUser(requester)
    automatic_review_service->>automatic_review_service: isConditionMatched(AMR, AR, requester.annotations)
    automatic_review_service->>teleport: approve AR
    requester->>teleport: tsh login --request-id=<request-id>
```
1. The automatic review service is initialized and watches for ARs from Teleport.
2. When a user creates an AR, and after the automatic review service observes
the event, the automatic review service requests additional information about
the user.
3. The automatic review service then checks to see if the AR matches any
existing AMRs. The automatic review service provides the additional user
traits received from Teleport before the AMR condition is evaluated.
4. If the AR matches the AMR, the plugin submits a review for the AR.

## Security & Auditability
Automatic reviews are already a supported feature, although it is currently
only supported when integrated with an external incident management system. The
same security concerns apply with built-in automatic reviews, as they apply to
automatic reviews with an external plugin.

Automatic reviews are submitted using the system user `@teleport-access-approval-bot`.
Audit log events `access_request.review` are created whenever an access request
is reviewed, including automatically reviewed requests. The event contains the
same information as regular access request reviews.
```json
{
  "cluster_name": "example.teleport.sh",
  "code": "T5002I",
  "ei": 0,
  "event": "access_request.review",
  "expires": "2025-02-08T04:04:28.653838697Z",
  "id": "0193083a-77c8-73e1-9f6b-8e337215c5d1",
  "max_duration": "2025-02-08T04:04:28.653838697Z",
  "proposed_state": "APPROVED",
  "reason": "Access request has been automatically approved by 'teleport' plugin because user 'user@goteleport.com' satisfies the 'cloud-dev-pre-approved' access monitoring rule condition.",
  "reviewer": "@teleport-access-approval-bot",
  "state": "APPROVED",
  "time": "2025-02-07T20:04:31.196Z",
  "uid": "a5c9adb1-2a12-47d4-a626-f81a21e22f69"
}
```

## Observability
Anonymized metrics will be collected for access requests. These metrics will
allow us track automatic review usage, and with which plugin it is being used.
- `tp.access_request.create`: Specifies an access request create event.
  - `tp.cluster_name`: Specifies the anonymized cluster name.
  - `tp.user_name`: Specifies the anonymized requesting user name.
  - `tp.access_request.resource_kinds`: Specifies the list of requested resource kinds.

- `tp.access_request.review`: Specifies an access request review event.
  - `tp.cluster_name`: Specifies the anonymized cluster name.
  - `tp.user_name`: Specifies the anonymized reviewer user name.
  - `tp.access_request.resource_kinds`: Specifies the list of requested resource kinds.
  - `tp.access_request.is_bot_reviewed`: Is true if request was reviewed by a bot user.
  - `tp.access_request.proposed_state`: Specifies the proposed state of the review.
    Either `approved` or `denied`.

## Out of Scope

### Predicate Expression Validation
This feature relies on Teleport's Predicate Language to define the conditions
for Access Monitoring Rules. This makes it easy for users to misconfigure rules.
This can result in conditions that are either too restrictive and never match,
or too permissive and allow unintended access.

To mitigate this risk, users should be protected against misconfigured rule
conditions. This could be addressed through static validation of rule conditions,
or by providing a user-friendly interface to preview and test rules before they
are created. One potential approach is leveraging Teleport Access Graph to
visualize and review automatic review rules.

## Implementation Plan
1. Extend the Access Monitoring Rule to support the `automatic_review` field
and the `user.traits` variable.
2. Implement the automatic review service.
3. Deploy the automatic review service as part of Teleport initialization.
4. Update WebUI to allow users to create and view automatic reviews.
5. Update the Terraform resource schema to allow configuration of automatic
reviews with the Terraform provider.
6. Enable metrics for access requests.
7. Release guide on how to configure automatic reviews using Access
Monitoring Rules.
