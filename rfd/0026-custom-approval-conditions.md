---
authors: Forrest Marshall (forrest@gravitational.com)
state: implemented

---

## RFD 0026 - Custom Approval Conditions

Proposal for extending the Access Workflow API to support a more detailed
approval model, including custom scopes for approvers, and multi-party approval.

### Overview

#### Related Issues

- [#5007](https://github.com/gravitational/teleport/issues/5007) - Dual Authorization
- [#4309](https://github.com/gravitational/teleport/issues/4309) - Scoped Approvals
- [#4937](https://github.com/gravitational/teleport/issues/4937) - Workflow UI


#### Problem

The current approval model is very simplistic.  Any user which holds write permissions for
the `access_request` resource may update the state of any access request from `PENDING`
to `APPROVED` (or `DENIED`).  This is the only means by which a user may approve requests, so
any user that should be allowed to approve requests must currently be granted unilateral control
over all access requests.  This simplistic model works for cases where all access requests
are manged by a single entity (a 'plugin' user), but isn't granular enough to handle complex
controls.

#### Requirements

- Support configuration of access requests s.t. multiple approvals from different
teleport users must be submitted prior to the access request transitioning from
a `PENDING` to an `APPROVED` state.

- Add custom permission scopes for approvers s.t. individual users may have permissions
to only approve certain access requests (e.g. members of team `dev` may be permitted to
approve access requests for `dev`, without necessarily being able to approve access requests
for `admin`).

- Support sufficiently granular configuration to allow scenarios such as "requires 2 dev
approvals or 1 admin approval" or "requires 2 dev approvals, but may be denied by any
non-contractor".

### Proposition

#### Model

Rather than simple write operations that are either always or never allowed, the new model
will revolve around *reviews* and *approval thresholds*.  Users with the correct permissions
will be able to propose a state transition which does not come into effect until a specified
threshold (e.g. reaching a certain number of approvals) is reached.

Say that `carol` has role `intern` and `alice` and `bob` both hold role `dev`.  We want
`carol` to be able to temporarily use role `staging` if both `alice` and `bob` approve of
her doing so.  We can construct a pair of roles that look something like this:

```yaml
kind: role
metadata:
  name: dev
spec:
  allow:
    # grants the ability to review requests for role 'staging'
    review_requests:
      roles: ['staging']
    # ...
---
kind: role
metadata:
  name: intern
spec:
  allow:
    request:
      roles: ['staging']
    # require 2 approvals
    thresholds:
    - approve: 2
```

When `carol` generates her access request, the RBAC layer will automatically determine that
her request is subject to an approval threshold of `2` and encode this as part of the
pending request when it is stored in the backend.

When the first approval comes in, it will be stored, but the state of the access request will
not be updated since the request has not yet met its approval threshold.  Ex:

```
{
  "state": "PENDING",
  "thresholds": [ ... ],
  "reviews": [
    {
      "state": "APPROVED",
      "user": "alice",
      "reason": "You seem trustworthy",
      ...
    }
  ],
  ...
}
```

When the second approval arrives, the auth server can automatically detect that the approval
condition (a threshold of 2) has been met, and transition the request from the `PENDING` to
the `APPROVED` state.

#### Advanced Approval Thresholds

Borrowing from the existing pattern of `where` clauses used by resource rules, we can
extend the idea of approval thresholds to something far more granular than a simple count.

The predicate language used in `where` clauses is cumbersome when dealing with complex
data.  Luckily, we can simplify it significantly by limiting the scope of predicates to
match single reviews.  Take this example which describes different thresholds for
different reviews depending on the traits provided by external identity providers:

```yaml
kind: role
# ...
spec:
  allow:
    request:
      thresholds:
      - name: Administrative control
        filter: 'contains(reviewer.traits["teams"], "admin")'
        approve: 1
        deny: 1
      - name: Developer control
        filter: 'contains(reviewer.traits["teams"],"dev") || contains(reviewer.roles, "dev")'
        approve: 2
        deny: 1
      - name: Let the commonfolk decide
        approve: 4
      # ...
```

Within the above framework, we can model the current default behavior of access requests
(applying the first proposed state-transition immediately) like so:

```yaml
kind: role
# ...
spec:
  allow:
    request:
      thresholds:
      - name: Default
        approve: 1
        deny: 1
```

*NOTE*: Because the roles that a user is allowed to request are defined as a "sum" of
the `allow.request` blocks across all of their statically assigned roles, unifying
custom approval conditions of any kind will be tricky.  More so because some roles with
different approval conditions may have overlap in terms of what roles they deem
requestable.  Basically, each *requested* role is going to need at least one
matching condition to pass from the set of *static* roles that permit it to be
requested.  This state is going to need to be tracked within the access request itself,
and will likely make it impossible to cleanly display to the user exactly what conditions
need to be met in order for their request to be approved.


#### Advanced Approval Permissions

The concept of the `allow.review_requests` block can very naturally be extended to support
more granular definitions of approvable roles.  Ex: 

```yaml
kind: role
# ...
spec:
  allow:
    review_requests:
      roles: ['*-staging']
      claims_to_roles:
        - claim: teams
          value: admin
          roles: ['*-prod']
      where: 'contains(request.system_annotations["teams"],"red")'
    # ...
```

Note that we are using the request's `system_annotations` attribute in order to indirectly
operate on a trait, rather than operating directly on the requesting user's traits.  This
is intended to ensure that traits of requesting users are not inadvertently exposed to
other users.  See the the 'User Data Leakage' discussion below for more info.

In general, what the above configuration allows us to do, is to construct a "matcher" which
allow us to answer the question "can user `X` serve as an approver for request `Y`" based
solely on the initial state of request `Y` and the current auth context of user `X`.
Note that just because the roles of a user say that they can theoretically be an approver,
does not mean that they are capable of triggering any of the custom thresholds defined
in the request.  This is OK, and we will allow them to submit a review anyhow.  The
question of what triggers a state-transition is separate.  In fact, it need not be answerable
by teleport.  It is acceptable for the request to never automatically state-transition,
and instead rely on an external plugin which monitors reviews and forcibly updates
state once whatever conditions that plugin cares about happen to be met.

#### Reviewers

With more users being potentially eligible to approve requests, it may become necessary to
provide hints for who should be reviewing a given request.  While it isn't feasible to
directly notify a user via teleport (non-static users are lazily created on login), that
doesn't stop us from supporting a loose concept of *suggested* reviewers.

When a user generates an access request, we can accept a list of arbitrary strings identifying
suggested reviewers:

```
$ tsh login --request-roles=dictator --request-reviewers=alice@example.com
```

Rather than requiring `alice` and `bob` to be existing teleport users, we can simply store
the strings alongside the request.  Clients can then filter by the username of the currently
logged in user where appropriate.  Ex:

```
$ tsh request ls --suggested
# ...
```

Simultaneously, plugin implementations may use this field in other ways.  A slack plugin,
for example, could DM each slack user whose name appeared in the suggested reviewers
list, providing a link to the request within the Web UI.  Since the request ID is opaque,
we don't actually need to verify any permissions until the link is clicked and the
potential reviewer authenticates with teleport.

If manually listing reviewers ends up being tedious, it would also be feasible to hard-code
suggested reviewers within an `allow.request` block like so:

```yaml
kind: role
# ...
spec:
  allow:
    request:
      suggested_reviewers: ["alice@example.com", "bob@example.com"]
      # ...
```

#### Supporting Extended Options

Ideally, all options available in the old approval style should be available in thresholded
approvals (i.e. an approval threshold of `1` should not be a special case).  Unfortunately,
not all parameters supported by the existing system translate intuitively to a multi-party
system:


##### Annotations

Calculating the `ResolveAnnotations` (annotations supplied upon approval/denial) of a request
is a bit tricky.  If two reviews provide two different annotation mappings, do you pick one?
Simply using the annotations from either the first or last review is problematic because that
makes us very sensitive to ordering (potentially leading to very confusing bugs).

Given that annotations are of the form `map[string][]string`, the annotations of all
reviews could theoretically be "summed" (e.g. `{"hello": ["world"]}` and `{"hello": ["there"]}`
become `{"hello": ["there","world"]}`).  This isn't a perfect solution, as it would prevent
users from treating the order of annotations as meaningful, but that may be for the best.
They were never intended to be meaningfully ordered, and this same kind of summing already
happens elsewhere.


##### Role Overrides

Role overrides are very tricky.  The existing system allows approvers to override the list of
roles granted by an access request (specifically, approvers can subselect, they cannot add
roles which are not present).  How this should map to thresholded approvals is unclear.

Say that `dave` requests roles `["foo","bar","bin"]` with an approval threshold of `2`.
If `bob` approves with override `["foo","bar"]`, and `alice` approves with override `["bar","bin"]`,
what happens? The approval threshold has been reached but, in a sense, only `bar` actually reached
the threshold. It may seem reasonable to only grant `bar`, since `bob`'s approval effectively denied
`bin` and  `alice`'s approval effectively denied `foo`, but that is only in the case where
there are only two people *trying* to approve. 

What if `carol` also submits an approval with no override (equivalent to `["foo","bar","bin"]`)
at the same time that `alice` and `bob` are submitting their approvals?
Since the approval threshold is `2`, we are now racing to decide what the role subselection
will be, as one of the three approvals will not be present at the time when the final role
list is calculated.  Any particular combination of three approvals results in a different outcome:

```
{
  "roles": ["???"],
  "state": "???",
  "thresholds": [ ... ],
  "reviews": [
    {
      "state": "APPROVED",
      "user": "bob",
      "roles": ["foo", "bar"],
      ...
    },
    {
      "state": "APPROVED",
      "user": "alice",
      "roles": ["bar", "bin"],
      ...
    },
    {
      "state": "APPROVED",
      "user": "carol",
      "roles": ["foo","bar","bin"],
      ...
    }
  ],
  ...
}
```

We could treat this case as out of scope.  After all, approvals can already race within the single-approver model.
The single-approver model, however, was explicitly built for the purposes of control by automated
software or by a small team of well-coordinated admins.  Ideally, multi-party approval should
be more resilient to multiple parties.

A partial solution to this conundrum is to tally reviews individually based on the state
they would resolve to (i.e. an approval for a specific set of roles counts only towards a final
request state with that exact set of roles).  Taking this strategy, the access request in the above
example would remain in a `PENDING` state because `alice`, `bob`, and `carol` effectively proposed
three separate possible outcomes.  Each possible outcome only has one supporting review, failing
to meet the threshold of `2`.  This doesn't eliminate the possibility of nondeterminism due to ordering
(after all, 4 people voting for two possible states still results in a race), but it does ensure
that no `APPROVED` state is reached that wasn't exactly supported by the requisite number of
approvals.  Whether this is better than simply disabling the role override feature in the
context of multiple approvals is questionable.
 

#### Partial Permission Overlap

Since users will now be able to approve access to some roles and not others, a new question
arises.  Should an approver's permissions be calculated based on the roles originally requested, or
the roles that the approver subselects to?

Say, for example, that `alice` is allowed to approve `staging`, but not `prod`.   If `bob` generates
a request for `staging` *and* `prod`, can `alice` approve the request if she subselects to just
`staging`? She obviously can't approve for `prod`, but granting `bob` access to `staging` is within her
power.  If so, can she *deny* the request?  It would be strange to have someone with no explicit
permissions allowing them to control access to `prod` be able to indirectly deny it.  On the other
hand, if a given user is allowed to control access to a role, it seems equally strange to have them
be powerless to deny access to the role because of the presence of an unrelated role within the
request.

As discussed above, we *could* choose to treat reviews as relating only to the exact outcome
they describe.  In the above discussion, however, we were treating approvals as being strictly
related to the *exact* set of roles they proposed (i.e. `["foo"]` has no relation to `["foo","bar"]`).
If we take this route with denials, then denying access to `["staging"]` does not deny access
to `["staging","prod"]`.  That feels wrong too.  One possible resolution is to simply accept
inconsistency here and treat denials as being for all permutations which include the specified
roles, while approvals are for the exact permutation specified.


#### User Data Leakage

Special care will need to be taken to ensure that we have a clear, and difficult to misuse,
model of what information can be leaked about request generators or reviewers.

Some "leaks" are obvious and intentional (e.g. if only role `foo` grants the ability to request
role `dev`, then anyone with the ability to *approve* role `dev` is going to be able to get a
pretty good idea of who in the system holds role `foo`).

Some potential leaks are less obvious.  If we were to expose a requestor's traits within the namespace
of a `thresholds[].filter` predicate, then a carelessly written predicate might allow approvers
to brute-force the contents of some of the requestors traits (e.g. if for some reason traits were
being compared to an input which the approver controlled, such as annotations).

Generally, this means that we need to avoid providing the ability for arbitrary strings provided
by one party, to be compared with structured permission data associated with another party.

Furthermore, this rules out providing the ability for a requestor to query teleport and ask "who
can approve this request" unless the requestor holds `read` and `list` permissions for
the `user` resource.

### Notes

- An exception should be added to prevent users from approving their own requests.  Just a good
footgun to avoid.
