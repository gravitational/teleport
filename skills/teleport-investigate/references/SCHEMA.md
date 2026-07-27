# Teleport `tctl investigate` Output Schema

`tctl investigate --format json` (or `--format yaml`) returns a single object:

| Field       | Type    | Description                                                                                                                                               |
| ----------- | ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `total`     | number  | Approximate match count across the time window, from the stats endpoint. May drift a few percent from `len(data)` on long windows.                        |
| `truncated` | bool    | `true` when more events matched than were returned under `--limit`. Raise `--limit` (or pass `0`) for the full set. Always `false` under `--facets-only`. |
| `facets`    | Facet[] | Aggregated value counts (see below).                                                                                                                      |
| `data`      | Event[] | Matching events. Empty when `--facets-only` is set.                                                                                                       |

## Facet Object

| Field    | Type             | Description                                                                                                                                                                                                      |
| -------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`   | string           | The **CLI flag name** (e.g. `resource`), not the underlying Athena column.                                                                                                                                       |
| `values` | {value, count}[] | Distinct values and their counts. `count = -1` marks a value that exists in the window but did not match the current filter — only shown with `--show-unmatched`, and useful for discovering filters to broaden. |

## Event Object

Every event in `data` carries the same envelope of common fields, regardless of
event type. These are always present, so you can rely on them when projecting:

| Field              | Type             | Description                                                                                                                               |
| ------------------ | ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `uuid`             | string           | Unique event identifier.                                                                                                                  |
| `time`             | string (RFC3339) | When the event occurred.                                                                                                                  |
| `event_type`       | string           | Type of event, e.g. `user.login`, `db.session.start`, `cert.create`.                                                                      |
| `event_source`     | string           | Source of the event.                                                                                                                      |
| `action`           | string           | Action performed; mirrors `event_type` for Teleport-native events.                                                                        |
| `status`           | string           | Outcome, e.g. `success`, `failure`.                                                                                                       |
| `origin`           | string           | Event origin, e.g. `example.teleport.com`.                                                                                                |
| `identity`         | object           | Actor: `{id, kind, name, token_id, user_agent}`.                                                                                          |
| `target`           | object           | Object acted on: `{id, kind, location, resource}`.                                                                                        |
| `location_details` | object           | Geo from IP: `{ip, city, region, country, latitude, longitude}`.                                                                          |
| `origin_details`   | object           | Optional; present for integrations. Any of `{aws_account_id, aws_service, github_organization, github_repo, okta_org, teleport_cluster}`. |
| `event_data`       | object           | **Variable** per-event-type payload — see below.                                                                                          |

`event_data` holds event-type-specific details and its shape varies by
`event_type`; the common envelope above does not describe its contents. To learn
what an event type carries, pull a few of them with a low `--limit` and inspect
the returned `event_data`:

```sh
$TCTL investigate --event-type user.login --limit 3 --format json \
  | jq '.data[].event_data'
```

Payloads are large, so when you don't need the full event project just the
fields you want with `jq`/`yq` rather than printing whole events — see
[EXPERIENCE.md](EXPERIENCE.md).

## Queryable fields

Structured flags and `--query` accept these names. The left column is what you
type; the right is the underlying column — both forms work in `--query`.

| Alias                 | Canonical column      | Notes                                                                                                                        |
| --------------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `source`              | `event_source`        |                                                                                                                              |
| `identity`            | `identity`            |                                                                                                                              |
| `identity_kind`       | `identity_kind`       | Used by the structured `--user-kind` flag.                                                                                   |
| `identity_id`         | `identity_id`         | Used by the structured `--user` flag.                                                                                        |
| `token`               | `token`               |                                                                                                                              |
| `action`              | `action`              |                                                                                                                              |
| `ip`                  | `ip`                  |                                                                                                                              |
| `city`                | `city`                |                                                                                                                              |
| `country`             | `country`             |                                                                                                                              |
| `region`              | `region`              |                                                                                                                              |
| `resource`            | `target_resource`     |                                                                                                                              |
| `kind`                | `target_kind`         |                                                                                                                              |
| `agent`               | `user_agent`          | Used by the structured `--user-agent` flag. Not populated on every event — see the caveat in [EXPERIENCE.md](EXPERIENCE.md). |
| `type`                | `event_type`          |                                                                                                                              |
| `status`              | `status`              |                                                                                                                              |
| `aws_account_id`      | `aws_account_id`      |                                                                                                                              |
| `aws_service`         | `aws_service`         |                                                                                                                              |
| `github_organization` | `github_organization` |                                                                                                                              |
| `github_repo`         | `github_repo`         |                                                                                                                              |
| `okta_org`            | `okta_org`            |                                                                                                                              |
| `teleport_cluster`    | `teleport_cluster`    |                                                                                                                              |

Unknown field names are rejected with `unknown field "<name>"`.
