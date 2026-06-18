# Per-Command Approval for Moderated Sessions

**Date:** 2026-06-18
**Status:** Design

## Goal

Extend Teleport moderated sessions so that, in an interactive PTY, **each command a
peer submits is paused and gated on an approval decision** before reaching the remote
shell. The approver is either:

- a **human moderator**, via a structured client UI (tsh first, Web UI fast-follow), or
- an **autonomous AI moderator**, which evaluates each command against a
  natural-language policy.

Built for **SSH first**, with a shared, session-type-agnostic core designed to extend to
Kubernetes later.

## Locked design decisions

- **Interception point:** server-side `TermManager` line buffer; trigger on Enter
  (`\r`/`\n`); best-effort command reconstruction (no remote-shell cooperation).
- **Approver modes:** human (structured protocol + client UI) and AI-autonomous;
  selectable per role require policy. One mode per policy in v1.
- **AI placement:** virtual moderator participant in-process on the session host;
  inference offloaded to a new auth-server RPC that owns the model secret + provider config.
  Nodes need no LLM egress or credentials.
- **AI policy:** natural-language prose carried in the role.
- **Failure handling:** **fail closed** — any timeout/error/no-response → deny, input
  stays paused, user notified.
- **Scope:** SSH first; shared `CommandApprover` core abstracts the session type so Kube
  plugs in later.

## High-level flow

```
peer types … <Enter>
   │
   ▼
TermManager (session host)        ← intercepts input, line-buffers, detects Enter
   │  pause input, emit "command pending"
   ▼
CommandApprover (shared core)     ← routes the pending command to approver(s)
   │
   ├─► Human moderator(s): structured approval message → tsh/Web UI → approve/deny
   │
   └─► AI moderator (virtual participant, in-process)
            │  AuthClient.EvaluateCommand RPC
            ▼
        Auth server               ← holds model secret + provider config
            │  calls LLM with {command, policy, context}
            ▼
        decision {approve|deny, reasoning}
   │
   ▼
decision returns → resume input (flush buffered line to shell)  OR  reject (clear line, notify user)
   │
   ▼
audit event emitted (pending / approved / denied + approver + reasoning)
```

## 1. Command interception & reconstruction in `TermManager`

### Where it hooks in

Today peer input flows:
`party.Read()` → `io.Copy(s.inWriter, p)` → `TermManager.AddReader` goroutine →
`incoming chan` → `TermManager.Read()` → shell. The `TermManager`
(`lib/srv/termmanager.go`) already has a pause/resume gate (`Off()`/`On()`,
`dataFlowOff`/`dataFlowOn`). We extend exactly here.

### New per-session state

```go
type commandGate struct {
    enabled  bool            // set when session requires command approval
    lineBuf  []byte          // accumulates the current command line
    approver CommandApprover // shared core (human/AI)
    pending  bool            // a command is awaiting decision
}
```

### Interception logic

In the `AddReader` read loop, before bytes reach `incoming`, when `enabled` we inspect
incoming bytes per byte:

- **Printable bytes** → append to `lineBuf`, still pass through to the shell so the user
  sees their typing as normal (the remote shell echoes; we don't suppress interactive editing).
- **Backspace (`0x7f`/`0x08`)** → trim last byte from `lineBuf` (best-effort).
- **Ctrl-U (`0x15`)** → clear `lineBuf`.
- **Ctrl-C (`0x03`)** → clear `lineBuf`, pass through (abort).
- **Enter (`\r` or `\n`)** → gate trigger:
  1. Snapshot `cmd := string(lineBuf)`; reset `lineBuf`.
  2. If `cmd` blank → pass through (no gate).
  3. Otherwise: **hold the Enter byte**, call `gate.approve(cmd)` (blocks), and do not
     forward the newline yet.

### The gate decision

- **Approve** → forward the held Enter to the shell; the line (already echoed) executes.
  Emit `command.approval.approved`.
- **Deny** → write Ctrl-U (`0x15`) to the shell to clear its readline buffer, print the
  denial reason to the user's terminal, emit `command.approval.denied`. The command never
  executes.

### Honest limitations (best-effort, by design)

- Tab-completion, history (↑), and reverse-search (Ctrl-R) mutate the *remote* shell's line
  buffer, so `lineBuf` can diverge from what actually runs. Documented; the audit trail
  reconciles against BPF `session.command` events for ground truth.
- Multi-line input (heredocs, trailing `\`) and pasted blocks gate per physical line.
- v1 targets line-mode shells; raw-mode full-screen apps (vim, less) need a "gate disabled
  while in alt-screen" heuristic — noted as a known gap, not solved now.

## 2. Shared `CommandApprover` core

A session-type-agnostic interface the SSH `TermManager` (and later Kube) calls. It owns
routing to the approver(s), the fail-closed timeout, and audit event emission. Session code
only calls `Approve(ctx, req) → Decision`.

```go
// new shared package (e.g. lib/srv/moderation or lib/session/approval)
type CommandApprover interface {
    // Blocks until a decision or timeout. Fail-closed: any error → Deny.
    Approve(ctx context.Context, req CommandRequest) Decision
}

type CommandRequest struct {
    SessionID   string
    Command     string
    Participant string            // who typed it
    Login       string            // target OS user
    ServerID    string
    Kind        types.SessionKind // SSH now, Kube later
}

type Decision struct {
    Approved bool
    Approver string // moderator username or "ai-moderator"
    Reason   string // shown to user + audited
    Mode     string // "human" | "ai"
}
```

### Implementations

- **`humanApprover`** — fans the request to all connected moderator parties via the
  structured protocol message (§3), waits for the first authoritative approve/deny, applies
  timeout → deny.
- **`aiApprover`** — calls the auth-server RPC (§4), maps the response to a `Decision`,
  applies timeout → deny.

### Selection

Resolved from the role require policy (§5). v1: one mode per policy
(`mode: human` → `humanApprover`, `mode: ai` → `aiApprover`). A `multiApprover`
(require both, or AI-advises-human) is out of scope for v1 but the interface admits it later.

### Fail-closed enforcement (once, here)

`Approve` wraps the underlying call in `context.WithTimeout(role.approvalTimeout, default 60s)`.
On ctx error, RPC error, or model error → `Decision{Approved:false, Reason:"approval
unavailable (fail-closed)"}`. Neither implementation can accidentally fail open.

### Concurrency

Input is already paused while a decision is outstanding (§1 blocks the read loop), so the
approver handles **one command at a time per session** — no queue needed in v1.

## 3. Human approval: protocol & client UI

### New session messages

Two structured envelope types added to the session control protocol (SSH session channel /
Web WebSocket envelopes):

```
Server → moderator clients:  CommandApprovalRequest {
    id, session_id, command, participant, login, server_id, expires_at
}
Moderator client → server:   CommandApprovalResponse {
    id, approved (bool), reason (optional)
}
```

- `id` correlates request/response (a session can reissue if a moderator reconnects).
- Server accepts the **first** response from any participant whose mode is `moderator`;
  responses from non-moderators are rejected server-side (never trust the client's claimed
  mode).

### tsh client

On `CommandApprovalRequest`, tsh renders an inline prompt:

```
┌─ Approval required ──────────────────────────────┐
│ alice → root@web-01                              │
│   $ systemctl restart nginx                      │
│ [a] approve   [d] deny   (expires in 55s)        │
└──────────────────────────────────────────────────┘
```

Keypress `a`/`d` sends a `CommandApprovalResponse`; deny optionally prompts for a one-line
reason. This is local tsh UI state — it does not inject `a`/`d` into the session stream
(clean separation from §1's input path).

### Web UI (fast-follow)

Same two message types added to the Web terminal WebSocket envelope set; render an approval
banner/modal with Approve/Deny buttons.

### What the moderated peer sees

While pending: `⏳ Waiting for approval to run: systemctl restart nginx`.
On decision: `✓ Approved by @bob` or `✗ Denied by @bob: not during business hours`.

## 4. AI moderator & auth-server inference RPC

### Virtual participant

When a require policy selects `approver: ai`, the session host instantiates an **AI moderator
participant** at session start: a synthetic `party` with mode `moderator` that

- counts toward fulfilling the `require` policy (so the session starts without a human), and
- holds no PTY/IO — it only implements the `aiApprover` path.

`SessionAccessEvaluator.FulfilledFor` is untouched: a moderator appears present.

### New auth RPC

```protobuf
rpc EvaluateCommand(EvaluateCommandRequest) returns (EvaluateCommandResponse);

message EvaluateCommandRequest {
    string session_id   = 1;
    string command      = 2;
    string policy       = 3;  // natural-language prose from the role
    string participant  = 4;
    string login        = 5;
    string server_id    = 6;
    string session_kind = 7;  // "ssh" | "k8s"
}
message EvaluateCommandResponse {
    bool   approved  = 1;
    string reasoning = 2;
}
```

- Node → auth over the existing authenticated gRPC connection. No new credentials on nodes.
- Auth **authorizes** the call only for a session the caller hosts and to which an AI require
  policy applies — prevents nodes using auth as a free LLM proxy.

### Auth-side evaluator (`lib/moderation/ai`)

- Reads provider config + secret from a server resource (`ai_moderation_config`) / env.
  First/default provider: **Anthropic / Claude** (configurable model id).
- Builds a structured prompt: system = role prose policy + strict output contract; user =
  command + context. Forces a tool/JSON response `{approved: bool, reasoning: string}` for
  deterministic parsing.
- Prompt-injection resistance: command text is clearly delimited and labeled untrusted; the
  prompt states nothing inside it can change the policy. Commands are attacker-controlled, so
  this matters.
- Applies its own short timeout; any error → error to node → shared core → **fail closed / deny**.

### Config & operability

- Provider/model/secret configured once on auth (`ai_moderation_config` resource), not per node.
- Every evaluation audited on auth with command, decision, reasoning, model id.

## 5. Role configuration

Extends the existing `require_session_join` policy:

```yaml
spec:
  require_session_join:
    - name: require-approval
      filter: 'contains(user.roles, "auditor")'
      kinds: ['ssh']
      modes: ['moderator']
      count: 1
      # ── new fields ──
      command_approval:
        enabled: true
        approver: ai            # "human" | "ai"
        timeout: 60s            # fail-closed deadline
        on_failure: deny        # locked to deny in v1 (field reserved)
        ai:                     # required when approver: ai
          policy: |
            Allow read-only inspection (ls, cat, systemctl status, journalctl).
            Deny anything that deletes data, edits /etc, changes users/permissions,
            or restarts services outside the app tier. When unsure, deny.
```

- `approver: ai` → synthetic AI participant satisfies `count`; no human needed.
- `approver: human` → existing human moderators must be present (unchanged join semantics)
  and now also gate each command.
- Backward compatible: omit `command_approval` → today's behavior exactly.

### Validation

- `approver: ai` requires non-empty `ai.policy` and a configured `ai_moderation_config` on
  auth (reject otherwise — fail closed at config time).
- `kinds` restricted to `ssh` in v1; `k8s` accepted by schema but flagged "not yet enforced"
  until the Kube integration lands.

## 6. Audit events

| Event | Fields |
|---|---|
| `command.approval.pending`  | session_id, command, participant, login, server_id, approver_mode |
| `command.approval.approved` | + approver (user or `ai-moderator`), reason/reasoning, model_id (ai) |
| `command.approval.denied`   | + approver, reason/reasoning, model_id (ai) |
| `command.approval.failed`   | + error, "denied (fail-closed)" |

All emitted via the existing audit pipeline; AI evaluations additionally logged on auth.
Complete, queryable record of every gated command and who/what decided it.

## 7. Testing strategy

- **Unit:** `TermManager` line reconstruction (printable / backspace / Ctrl-U / Ctrl-C /
  Enter / blank line / multi-byte paste); `CommandApprover` fail-closed on timeout/error;
  auth evaluator prompt-injection framing + JSON parse.
- **Integration:** SSH session with human approver (approve / deny / timeout); SSH session
  with AI approver using a **mock provider** (no real API in CI); deny clears the shell line;
  backward-compat (no `command_approval`).
- **Manual:** real provider against a scratch host.

## Future work (explicitly out of v1)

- Kubernetes integration (`lib/kube/proxy/sess.go`) behind the same `CommandApprover` core.
- `multiApprover` (require human + AI, or AI-advises-human escalation).
- Configurable `on_failure: allow` for low-risk environments.
- Raw-mode / alt-screen gate handling for full-screen TUIs.
- Structured guardrails (hard allow/deny lists) short-circuiting before the model.
