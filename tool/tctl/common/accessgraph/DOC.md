# Access Graph commands for `tctl`

A set of `tctl` commands for asking Access Graph questions from the
command line: who has access to what, what changed, what's anomalous,
and which access requests are sitting around unused.

The first time you run one of these, `tctl` fetches an Access Graph
client certificate and caches it in your keyring. After that, it just
works.

Every command supports `--format text|json|yaml` (detections also
supports `csv`).

## Commands

### `tctl access`

Who has access to what.

- `tctl access query <query>` — run a raw Access Graph query.
- `tctl access review resource <name>` — who used this resource?
- `tctl access review acl <name>` — who used this ACL?
- `tctl access review role <name>` — who used this role?
- `tctl access review user <name>` — what does this user have access to?

Add `--unused` to flip any review around (who *didn't* use it),
`--detailed` for a per-resource breakdown, and `--from` / `--to` to
pick a time window.

### `tctl investigate`

What did a user or resource do?

- `tctl investigate user <name>`
- `tctl investigate resource <name>`

Useful flags: `--from`, `--to`, `--limit`, `--event-type`,
`--exclude-event-type`.

### `tctl detections`

Browse security detections.

- `tctl detections ls` — list detections.
- `tctl detections get <id>` — show one in detail.

Filters: `--status`, `--source`, `--type`, `--severity`, `--from`,
`--to`. Use `--detailed` for more columns.

### `tctl access-requests`

Review Teleport access requests.

- `tctl access-requests ls`

Filters: `--kind`, `--state`, `--user`, `--approver`, `--limit`,
`--unused` (approved but never used), `--from`, `--to`.

### `tctl access-changes`

See how access paths to important resources are changing.

- `tctl access-changes ls`
- `tctl access-changes get <change-id>`

Filters: `--search`, `--kind`, `--type`, `--source`, `--limit`.

## Notes

1. `tctl access review` currently walks the access graph locally
   (`graph.go`) because `access_path` query results don't include the
   per-path data these commands need. This is temporary — a new Access
   Graph endpoint is being prototyped to replace it. The prototype
   isn't ready yet: JSON deserialization is too slow on large
   datasets.

2. The detections commands are still being finished. While testing I
   needed a way to surface "IAC not configured" errors; doing it on
   each endpoint call was too slow, so the check now happens when the
   client certificate is issued.

3. As individual features land in their final form, this branch will
   be rebased on top of the opened PRs so that what's shown here always
   reflects the latest state. The first rebase happens tomorrow when
   the detections PRs go up.
