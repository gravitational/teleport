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

**Keep this query-free.** Adding a positional content query (e.g.
`"privileged commands, secrets"`) ranks results by relevance to it *and* caps at
`--limit`, so the set skews toward whatever the query *sounds like* and its severity
mix stops being representative (you will over-report criticals). The no-query form
above is the unbiased population for the window; scope it with structured flags
(`--username`, `--label`, `--kind`) freely — those filter without biasing, only a
text query biases. (And never pass `""` for the query — it errors; see SCHEMA.md.)

**High severity ≠ malice.** Severity is the summarizer's risk score; demo/lab
clusters and legitimate security-tooling or admin work routinely score high or
critical. Treat it as a triage signal, not a verdict — corroborate before escalating
(the mirror of "low ≠ safe" below).

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
recording** for anything high-stakes: `$TSH play <session-id>` or
`$TCTL recordings download <session-id>` (the summary is a lead, the recording is
the evidence).

**Mind coverage gaps when reconstructing "what they did":**
- `recordings search` only sees *summarized* sessions; pair it with `recordings ls`
  (segment by `.event`) so you don't miss unsummarized, app, db, or desktop activity.
- Segment `ls` rows by the **`interactive`** flag. **Non-interactive exec sessions
  are often not recorded at all** (`tsh play` → "recording not found") and not
  summarized — they appear in `ls` as evidence the action happened, but neither the
  content nor an AI summary is retrievable. State that explicitly rather than
  implying the session was empty or clean.

## Playbook 5 — Access-request / JIT window context

"Show everything that happened under access request <id>."

```bash
$TCTL recordings search --access-request=<id> --format=json --limit=200
```

Group the returned sessions by `username` + time to see the full picture of an
elevation window. **Multi-session attacks (activity split across several JIT
sessions) are a known blind spot** — review the whole set together rather than
trusting per-session summaries in isolation.

## Playbook 6 — Probe summary content without the prose (keyword bisection)

The JSON never contains the prose summary (SCHEMA.md), so when you need to know
*what a session was about* and can't open the web player, use `--search-mode=keyword`
as a content probe: a session is returned **only if** the term appears in its
summary. Scope tightly to the candidate set, then bisect with single terms.

```bash
# Each probe asks "does this term appear in this user/host/window's summaries?"
$TCTL recordings search "exfiltration" --search-mode=keyword \
  --username=<u> --server-hostname=<host> --from=<t0> --to=<t1> --format=json --limit=100
```

- **Confirm the probe works first:** a nonsense term (`"zxqv_nonsense"`) must return
  `[]`. If it returns rows, keyword content-filtering isn't active and the method is
  invalid.
- Start broad (multi-term: `"password secret credential token"`), then bisect with
  single terms (`exfiltration`, `shadow`, `passwd`, `deleted`) to characterize a
  specific session — e.g. narrowing a sev-4 session to "credentials + shadow/passwd +
  exfiltration, and *not* ransomware/persistence".
- Cross-reference the returned `session_id`s against your `ls` / search list to
  attribute a topic to a specific recording.

> **Critical caveat — keyword probing is negation-blind.** A hit means the *term
> appears* in the summary; it CANNOT distinguish "exfiltrated credentials" from
> "**no** exfiltration occurred" — both match `exfiltration`. This characterizes the
> *topics* a summary discusses, not what factually happened. Treat results as leads
> and **confirm with the actual recording** (web player / `tsh play`) before
> reporting a finding.

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
