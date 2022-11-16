# PostHog data model

All events, event properties and person properties are "namespaced" by using the `tp.` prefix in their name.

## `distinct_id`

The `distinct_id` field as emitted by `prehog` is always `tp.<account_id>` where `account_id` is the account ID as reported by an enterprise license file. For licenses that don't have an account ID set (all recent ones, at time of writing), the license _name_ is taken to be the account ID. Care should be taken upon generating new licenses for the same account to use the previous license name as the new account ID in the license.

If no valid license is available, an error is returned and no event is generated. In the future, to support OSS, we might pick something along the lines of `tp.cluster.<cluster_name>` where `cluster_name` is the anonymized cluster name.

Note: the above implies that the PostHog "Person" is an entire account - i.e. a customer with all of their clusters. On Cloud each cluster belongs to a separate account, so the two concepts are equivalent, but for on-prem we're gonna generate events from all the clusters using the same license file as belonging to the same Person.

## Events

### `tp.user.login`

Successful login to Teleport.

Event properties:
- `tp.user_name`: anonymized Teleport username, must be nonempty so we can track unique users
- `tp.connector_type`: optional, value should be `github`/`saml`/`oidc` (could easily be turned into `yes` or some other non-empty value in the future), set if the successful login used a SSO connector

Person properties (set once):
- `tp.first_login`: timestamp of the event
- `tp.first_sso_login`: timestamp of the event, set if the login used a SSO connector

### `tp.sso.create`

Creation of a SSO auth connector.

Event properties:
- `tp.connector_type`: value must be nonempty at the moment, intended to be `github`/`saml`/`oidc`

### `tp.resource.create`

Creation of a resource.

Event properties:
- `tp.resource_type`: value must be nonempty, intended to be `ssh`/`kube`/`app`/`db`/`desktop`

### `tp.session.start`

Beginning of a new session (ssh login, kubectl exec, app session, db connection, desktop login).

Event properties:
- `tp.user_name`: anonymized Teleport username, must be nonempty
- `tp.session_type`: nonempty, probably the same as `tp.resource_type`

Person properties (set once):
- `tp.first_session`: timestamp of the event
<!-- TODO(espadolini): properties for the first session of each type? -->

## UI events

Events starting with `tp.ui.` originate from user interactions in the UI.

### `tp.ui.banner.click`

User clicks a cluster alert banner link via the UI.

Event properties:

- `tp.alert`: value is the cluster alert name the user interacted with, i.e.: "upgrade-to-paid-plan", "register-teleport-connect"

### `tp.ui.onboard.domainNameTC.submit`

User clicks a cluster alert banner link via the UI.

### `tp.ui.onboard.goToDashboard.click`

User clicks a cluster alert banner link via the UI.

### `tp.ui.onboard.getStarted.click`

User clicks a cluster alert banner link via the UI.

### `tp.ui.onboard.completeGoToDashboard.click`

User clicks a cluster alert banner link via the UI.

### `tp.ui.onboard.addFirstResource.click`

User clicks a cluster alert banner link via the UI.

### `tp.ui.onboard.addFirstResourceLater.click`

User clicks a cluster alert banner link via the UI.

<!-- TODO(espadolini): figure out the sort of distinct_id that we get from marketing -->
