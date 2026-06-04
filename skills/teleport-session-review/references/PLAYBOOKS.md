# Session Search Playbooks

Real-world ways teams use `tctl recordings search`. Each playbook maps an intent
to a concrete command and a triage approach. Always `--format=json`; parse per
[SCHEMA.md](SCHEMA.md).

> **Risk triage is client-side.** The `--severity` flag is often a no-op
> server-side, and many sessions (databases especially) carry no severity at all.
> Pull the result set and filter/sort on the parsed `severity` field yourself
> (1=low, 2=medium, 3=high, 4=critical; absent = unscored). "Alert on medium and
> above" therefore means `severity >= 2` in your own code, not `--severity=medium`.

## Personas

- **SOC / detection engineer** — triage by risk, surface medium+ sessions, pivot
  into investigation. Cares about *what the user actually did*, not just access
  metadata.
- **Compliance / GRC reviewer** — periodic (e.g. quarterly) review of *all*
  production access; needs an auditable artifact and proof of action.
- **Threat hunter** — natural-language sweeps across large session volumes for
  specific techniques (TTPs).
- **Incident responder** — scope to a host / user / time window, reconstruct the
  session, export evidence.

## Playbook 1 — Compliance review of production access

"Review all production access sessions from last quarter."

```bash
$TCTL recordings search --from=<quarter-start> --to=<quarter-end> \
  --label=env=prod --format=json --limit=500
```

- Group by `username` (and `resource_name`); sort by `severity` desc.
- Present a review table; flag `severity >= 2` for human attention; mark the rest
  reviewed.
- **Watch the `--limit` cap (default 50, max returned = limit, no "more" hint).**
  For a full-quarter review, raise `--limit`, and if you still hit it, page by
  narrowing the window (month-by-month) or by `--label` / `--resource-name`. Tell
  the user if the set may be partial.
- Link each session's web player for the auditable artifact (regulators want
  proof of action, not just an AI summary) — see Step 5 of SKILL.md.

## Playbook 2 — Risk-based triage / "alert on medium+ risk"

"What are the riskiest sessions this week?"

```bash
$TCTL recordings search --from=<7-days-ago> --format=json --limit=200
```

Then **client-side**: keep `severity >= 2`, sort desc, show highest first. Offer
to pivot into any session (download / `tsh play`). This is the daily SOC loop.

## Playbook 3 — Threat hunting for specific techniques

Use a natural-language query describing the behavior. Default `--search-mode=hybrid`
(best recall). Use `keyword` when hunting an exact token (a binary or file name
like `nmap`, `/etc/shadow`); use `embeddings` for purely conceptual queries.

| Hunt | Example query | Tip |
|------|---------------|-----|
| Privilege escalation | `"privilege escalation, sudo to root, or setuid binary"` | usually high/critical |
| SSH config tampering | `"modified sshd_config or ssh authorized_keys"` | |
| Persistence | `"persistence via cron, systemd service, or base64-encoded payload"` | often critical |
| Data exfiltration | `"bulk data export or unusually large database read"` | add `--kind=db` |
| Secret / PII exposure | `"secrets, API keys, or personal data printed to the terminal"` | |
| Kernel / eBPF | `"loaded an eBPF program or kernel module"` | |
| Reconnaissance | `"network scanning or host enumeration"` | |
| Exact tool/file | `"nmap"`, `"/etc/shadow"` | `--search-mode=keyword` |

Combine with structured filters to scope: `--kind`, `--username`, `--role`,
`--resource-name`, `--label`. Then triage the results by `severity` (Playbook 2).

## Playbook 4 — Incident response pivot

"What did <user> do on <host> between <t0> and <t1>?"

```bash
$TCTL recordings search --username=<user> --resource-name=<host> \
  --from=<t0> --to=<t1> --format=json --limit=200
```

Reconstruct the timeline from the results, then **corroborate with the actual
recording** for anything high-stakes: `tsh play <session-id>` or
`$TCTL recordings download <session-id>` (the summary is a lead, the recording is
the evidence).

## Playbook 5 — Access-request / JIT window context

"Show everything that happened under access request <id>."

```bash
$TCTL recordings search --access-request=<id> --format=json --limit=200
```

Group the returned sessions by `username` + time to see the full picture of an
elevation window. **Multi-session attacks (activity split across several JIT
sessions) are a known blind spot** — review the whole set together rather than
trusting per-session summaries in isolation.

## Limitations to communicate

These come from red-team testing and the Teleport docs — state them when a result
looks "clean" or sparse:

- **Search only sees summarized sessions.** Per the docs, a recording is
  searchable only after a successful summary, and summarization is gated by
  `inference_policy`. Unsummarized sessions never appear — so "no results" can
  mean "not summarized," not "didn't happen." Use `recordings ls` for raw,
  complete coverage.
- **A low or absent severity is not proof of safety.** Summaries can miss evasive
  input: STDIN-hidden entry (`read -s`), control-character obfuscation, and
  typed-then-deleted commands. For high-stakes review, watch the recording.
- **Multi-session / chained-JIT attacks** aren't reliably caught (Playbook 5).
- **Database coverage is Postgres-centric**, and db sessions are frequently
  unscored (no `severity`) — don't rank them out just because severity is absent.
- The JSON omits the prose summary entirely — for the narrative, use the web
  player or the interactive `tctl recordings search` TUI.
