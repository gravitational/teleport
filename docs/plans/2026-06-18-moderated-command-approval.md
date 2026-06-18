# Moderated Command Approval Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Gate each command typed in an interactive SSH PTY on an approval decision from either a human moderator (structured client UI) or an autonomous AI moderator (natural-language policy evaluated via an auth-server RPC), failing closed.

**Architecture:** Server-side `TermManager` line-buffers PTY input and, on Enter, pauses input and asks a shared `CommandApprover` for a decision before forwarding the newline to the shell. `humanApprover` fans out over the existing `x-teleport-event` SSH channel to moderator clients; `aiApprover` calls a new `EvaluateCommand` auth RPC where the model secret/provider live. SSH first; the core is session-type-agnostic so Kube can plug in later.

**Tech Stack:** Go, gRPC/protobuf (gogo legacy protos), Teleport `lib/srv` session machinery, `tsh` client event loop, Anthropic API (on auth server only).

**Design doc:** `docs/plans/2026-06-18-moderated-command-approval-design.md`

**Conventions for every task:**
- TDD: write the failing test, run it (confirm the *expected* failure), implement minimally, run until green, commit.
- Run a single Go test with: `go test ./<pkg>/ -run '<TestName>' -v`
- Commit message trailer (every commit):
  ```
  Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
  ```
- Work happens in the worktree: `/Users/tiagosilva/code/teleport/.worktrees/moderated-command-approval`

---

## Phase 1 — Shared `CommandApprover` core (pure Go, no Teleport wiring)

This phase is fully unit-testable in isolation. It defines the interface, the data types, the fail-closed timeout wrapper, and a fake approver for later integration tests.

### Task 1.1: Package + core types

**Files:**
- Create: `lib/srv/approval/approval.go`
- Test: `lib/srv/approval/approval_test.go`

**Step 1 — Write the failing test:**
```go
package approval

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestDecisionDenied(t *testing.T) {
	d := Decision{Approved: false, Approver: "ai-moderator", Reason: "blocked", Mode: ModeAI}
	require.False(t, d.Approved)
	require.Equal(t, "ai-moderator", d.Approver)
	require.Equal(t, ModeAI, d.Mode)
}

func TestCommandRequestKind(t *testing.T) {
	r := CommandRequest{Command: "ls", Kind: types.SSHSessionKind}
	require.Equal(t, types.SSHSessionKind, r.Kind)
}
```

**Step 2 — Run, expect FAIL** (package/types undefined):
`go test ./lib/srv/approval/ -run 'TestDecision|TestCommandRequest' -v`

**Step 3 — Implement:**
```go
// Package approval gates individual commands in moderated sessions on an
// approval decision from a human moderator or an autonomous AI moderator.
package approval

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// Mode identifies which kind of approver produced a decision.
type Mode string

const (
	ModeHuman Mode = "human"
	ModeAI    Mode = "ai"
)

// CommandRequest is a single command awaiting an approval decision.
type CommandRequest struct {
	SessionID   string
	Command     string
	Participant string // who typed it
	Login       string // target OS user
	ServerID    string
	Kind        types.SessionKind
}

// Decision is the outcome of evaluating a CommandRequest.
type Decision struct {
	Approved bool
	Approver string // moderator username or "ai-moderator"
	Reason   string
	Mode     Mode
}

// CommandApprover decides whether a command may run. Implementations MUST be
// fail-closed: any error or timeout yields a denying Decision.
type CommandApprover interface {
	Approve(ctx context.Context, req CommandRequest) Decision
}
```

**Step 4 — Run, expect PASS.**

**Step 5 — Commit:** `feat(approval): add CommandApprover core types`

---

### Task 1.2: Fail-closed timeout wrapper

The single chokepoint that guarantees no implementation fails open.

**Files:**
- Modify: `lib/srv/approval/approval.go`
- Test: `lib/srv/approval/approval_test.go`

**Step 1 — Failing test:**
```go
func TestWithTimeoutDeniesOnSlowApprover(t *testing.T) {
	slow := approverFunc(func(ctx context.Context, _ CommandRequest) Decision {
		<-ctx.Done() // never decides on its own
		return Decision{Approved: true} // would-be approve, must be overridden
	})
	a := WithTimeout(slow, 20*time.Millisecond)
	d := a.Approve(context.Background(), CommandRequest{Command: "rm -rf /"})
	require.False(t, d.Approved, "timeout must deny")
	require.Contains(t, d.Reason, "fail-closed")
}

func TestWithTimeoutPassesThroughFastDecision(t *testing.T) {
	fast := approverFunc(func(_ context.Context, _ CommandRequest) Decision {
		return Decision{Approved: true, Approver: "bob", Mode: ModeHuman}
	})
	a := WithTimeout(fast, time.Second)
	d := a.Approve(context.Background(), CommandRequest{Command: "ls"})
	require.True(t, d.Approved)
	require.Equal(t, "bob", d.Approver)
}

// approverFunc adapts a func to CommandApprover (test helper).
type approverFunc func(context.Context, CommandRequest) Decision

func (f approverFunc) Approve(ctx context.Context, r CommandRequest) Decision { return f(ctx, r) }
```

**Step 2 — Run, expect FAIL** (`WithTimeout` undefined).

**Step 3 — Implement in `approval.go`:**
```go
import "time"

// DefaultTimeout is the fail-closed deadline when a role does not set one.
const DefaultTimeout = 60 * time.Second

// WithTimeout wraps an approver so any decision not returned within d denies
// the command (fail-closed). It also denies if the wrapped approver returns
// after ctx is cancelled.
func WithTimeout(inner CommandApprover, d time.Duration) CommandApprover {
	return approverFunc(func(ctx context.Context, req CommandRequest) Decision {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()

		ch := make(chan Decision, 1)
		go func() { ch <- inner.Approve(ctx, req) }()

		select {
		case dec := <-ch:
			return dec
		case <-ctx.Done():
			return Decision{
				Approved: false,
				Approver: "system",
				Reason:   "approval unavailable (fail-closed): " + ctx.Err().Error(),
			}
		}
	})
}
```
Move `approverFunc` into `approval.go` (non-test) so production wrapping can reuse it; keep the test's local copy out (delete it from the test, it now lives in the package).

**Step 4 — Run, expect PASS.**

**Step 5 — Commit:** `feat(approval): add fail-closed timeout wrapper`

---

## Phase 2 — Command line reconstruction in `TermManager`

Pure byte-stream logic, unit-testable without a session. We isolate reconstruction into its own type so it can be tested independently of `TermManager`'s channels.

### Task 2.1: `lineBuffer` reconstruction type

**Files:**
- Create: `lib/srv/linebuffer.go`
- Test: `lib/srv/linebuffer_test.go`

**Step 1 — Failing test (table-driven, covers the §1 byte rules):**
```go
package srv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineBuffer(t *testing.T) {
	type ev struct {
		in       []byte
		wantCmd  string // non-empty => expect a completed line equal to this
		wantDone bool
	}
	tests := []struct {
		name string
		evs  []ev
	}{
		{"simple line", []ev{{[]byte("ls -la\r"), "ls -la", true}}},
		{"lf newline", []ev{{[]byte("whoami\n"), "whoami", true}}},
		{"backspace edits", []ev{{[]byte("lss\x7f -l\r"), "ls -l", true}}},
		{"ctrl-u clears", []ev{{[]byte("rm -rf /\x15ls\r"), "ls", true}}},
		{"ctrl-c aborts line", []ev{{[]byte("dangerous\x03"), "", false}, {[]byte("ls\r"), "ls", true}}},
		{"blank line ignored", []ev{{[]byte("\r"), "", false}}},
		{"split across writes", []ev{{[]byte("ec"), "", false}, {[]byte("ho hi\r"), "echo hi", true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := &lineBuffer{}
			for _, e := range tt.evs {
				cmd, done := lb.feed(e.in)
				require.Equal(t, e.wantDone, done)
				if e.wantDone {
					require.Equal(t, e.wantCmd, cmd)
				}
			}
		})
	}
}
```

**Step 2 — Run, expect FAIL** (`lineBuffer` undefined):
`go test ./lib/srv/ -run 'TestLineBuffer' -v`

**Step 3 — Implement `lib/srv/linebuffer.go`:**
```go
package srv

// lineBuffer reconstructs a best-effort command line from raw PTY input bytes.
// It is intentionally simple: it handles plain text, backspace, Ctrl-U (clear
// line) and Ctrl-C (abort line). Shell-side editing (tab-completion, history,
// reverse-search) is NOT mirrored; reconstruction is best-effort by design and
// reconciled against BPF session.command events for the audit record.
type lineBuffer struct {
	buf []byte
}

const (
	byteCtrlC     = 0x03
	byteBackspace = 0x7f
	byteBackspace2 = 0x08
	byteCtrlU     = 0x15
	byteCR        = '\r'
	byteLF        = '\n'
)

// feed consumes input bytes. When a line terminator is seen it returns the
// completed (trimmed-of-terminator) command and done=true. A blank line returns
// ("", false). Bytes after a terminator within the same slice continue a new
// line. Only the FIRST completed line in a slice is returned; see feedAll for
// multi-line handling (heredocs/paste) — v1 gates per first line and lets the
// caller re-feed the remainder.
func (l *lineBuffer) feed(p []byte) (string, bool) {
	for i, b := range p {
		switch b {
		case byteCR, byteLF:
			cmd := string(l.buf)
			l.buf = l.buf[:0]
			// Consume a paired \r\n so the caller does not double-trigger.
			_ = i
			if cmd == "" {
				return "", false
			}
			return cmd, true
		case byteBackspace, byteBackspace2:
			if len(l.buf) > 0 {
				l.buf = l.buf[:len(l.buf)-1]
			}
		case byteCtrlU:
			l.buf = l.buf[:0]
		case byteCtrlC:
			l.buf = l.buf[:0]
		default:
			l.buf = append(l.buf, b)
		}
	}
	return "", false
}
```
> Note: the `\r\n` pairing and "bytes after terminator" edge cases are simplified here. If the split-across-writes or paired-newline tests fail, refine `feed` to return the consumed index so the caller can re-feed the tail; update the test accordingly. Keep the rule set (the 5 control behaviors) stable.

**Step 4 — Run, expect PASS.** Iterate on `feed` until all table rows pass.

**Step 5 — Commit:** `feat(srv): add best-effort PTY line reconstruction`

---

## Phase 3 — Gate wiring in `TermManager`

Now connect reconstruction + approver to the real input path. The gate lives in `TermManager` because that is where the existing pause/resume (`On`/`Off`, `dataFlowOff`) already sits (`lib/srv/termmanager.go:52-83`, `204-224`, `238-290`).

### Task 3.1: Add gate fields + setter

**Files:**
- Modify: `lib/srv/termmanager.go` (struct at `:52-83`)
- Test: `lib/srv/termmanager_test.go`

**Step 1 — Failing test:**
```go
func TestTermManagerCommandGateDisabledByDefault(t *testing.T) {
	tm := NewTermManager()
	require.False(t, tm.commandGateEnabled())
}

func TestTermManagerEnableCommandGate(t *testing.T) {
	tm := NewTermManager()
	tm.SetCommandGate(&fakeGate{})
	require.True(t, tm.commandGateEnabled())
}

type fakeGate struct{ deny bool }

func (f *fakeGate) approve(cmd string) bool { return !f.deny }
```

**Step 2 — Run, expect FAIL.**

**Step 3 — Implement.** Add to the `TermManager` struct:
```go
	// commandGate, when non-nil, gates each completed input line on approval
	// before its terminating newline is forwarded to the shell.
	commandGate commandGate
	lineBuf     lineBuffer
```
Define the gate seam (kept tiny so Phase 4 supplies the real implementation):
```go
// commandGate decides whether a reconstructed command line may proceed.
// approve blocks until a decision; true => forward the newline, false => reject.
type commandGate interface {
	approve(cmd string) bool
}

func (g *TermManager) SetCommandGate(cg commandGate) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.commandGate = cg
}

func (g *TermManager) commandGateEnabled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.commandGate != nil
}
```

**Step 4 — Run, expect PASS.**

**Step 5 — Commit:** `feat(srv): add command-gate seam to TermManager`

---

### Task 3.2: Intercept Enter in the input path

The `AddReader` loop (`lib/srv/termmanager.go:238-290`) pushes bytes to `g.incoming` at ~line 275. We intercept *before* forwarding, when a gate is set.

**Files:**
- Modify: `lib/srv/termmanager.go` (`AddReader` read loop, the `Read`/incoming path)
- Test: `lib/srv/termmanager_test.go`

**Step 1 — Failing test** (drive bytes through the manager; assert no un-approved newline reaches the shell-side `Read`):
```go
func TestTermManagerGateApprovesForwardsNewline(t *testing.T) {
	tm := NewTermManager()
	tm.On()
	tm.SetCommandGate(&fakeGate{deny: false})

	pr, pw := io.Pipe()
	tm.AddReader("peer", pr)
	go pw.Write([]byte("ls\r"))

	out := readAllWithin(t, tm, time.Second)
	require.Equal(t, "ls\r", string(out)) // echo + approved terminator
}

func TestTermManagerGateDeniesDropsNewline(t *testing.T) {
	tm := NewTermManager()
	tm.On()
	tm.SetCommandGate(&fakeGate{deny: true})

	pr, pw := io.Pipe()
	tm.AddReader("peer", pr)
	go pw.Write([]byte("rm -rf /\r"))

	// The command text may echo through, but the terminating \r must NOT, and a
	// Ctrl-U (0x15) must be injected to clear the shell line.
	out := readAllWithin(t, tm, time.Second)
	require.NotContains(t, string(out), "\r")
	require.Contains(t, string(out), string([]byte{byteCtrlU}))
}

// SECURITY: a multi-line paste must gate EACH line; a denied line in the middle
// must not execute and must not let later lines' newlines through ungated.
func TestTermManagerGateMultiLinePaste(t *testing.T) {
	// deny only "rm -rf /"
	gate := &fakeGate{denyFn: func(cmd string) bool { return cmd == "rm -rf /" }}
	tm := NewTermManager()
	tm.On()
	tm.SetCommandGate(gate)

	pr, pw := io.Pipe()
	tm.AddReader("peer", pr)
	go pw.Write([]byte("ls\nrm -rf /\nwhoami\n"))

	out := string(readAllWithin(t, tm, time.Second))
	// Exactly two approved newlines (ls, whoami); the denied line gets a Ctrl-U.
	require.Equal(t, 2, strings.Count(out, "\r"))
	require.Equal(t, 1, strings.Count(out, string([]byte{byteCtrlU})))
	require.Equal(t, []string{"ls", "rm -rf /", "whoami"}, gate.seen) // all three gated
}
```
Update `fakeGate` to support a per-command decision and record what it saw:
```go
type fakeGate struct {
	deny   bool
	denyFn func(string) bool
	seen   []string
}

func (f *fakeGate) approve(cmd string) bool {
	f.seen = append(f.seen, cmd)
	if f.denyFn != nil {
		return !f.denyFn(cmd)
	}
	return !f.deny
}
```
(Add a `readAllWithin` helper that reads until the writer's bytes are drained or the deadline passes, and fails the test on timeout.)

**Step 2 — Run, expect FAIL.**

**Step 3 — Implement.** In the `AddReader` goroutine, when `g.commandGate != nil`, route the slice through a new `gateInput` method instead of straight to `incoming`. The gate walks the chunk, splitting at terminators, and **never forwards a newline for a command until it is approved**:
```go
// gateInput reconstructs command lines from raw input and gates each one before
// its terminating newline reaches the shell. Non-terminator bytes (text, and
// in-line editing like backspace/Ctrl-U) are forwarded as-is so the user's
// terminal echoes normally. On each terminator the completed command is gated:
// approve => emit a carriage return (the line runs); deny => emit Ctrl-U (the
// shell's readline buffer is cleared, the line never executes). A bare/blank
// terminator forwards a newline (fresh prompt) without gating.
//
// SECURITY: this is the gate-bypass boundary. A newline that would execute a
// command MUST NOT be forwarded before approve() returns true for that exact
// command. Multi-line input (pasted blocks) is handled by gating each line in
// order via lineBuffer.feedLines — intermediate lines are never skipped.
func (g *TermManager) gateInput(p []byte) []byte {
	cg := g.commandGate
	if cg == nil {
		return p
	}
	var out []byte
	segStart := 0
	for i, b := range p {
		if b != byteCR && b != byteLF {
			continue
		}
		// Forward this segment's non-terminator bytes for echo.
		out = append(out, p[segStart:i]...)
		// Feed segment+terminator through the buffer: 0 lines (blank) or 1 line.
		lines := g.lineBuf.feedLines(p[segStart : i+1])
		if len(lines) == 0 {
			out = append(out, byteCR) // blank line: fresh prompt, nothing to gate
		}
		for _, cmd := range lines {
			if cg.approve(cmd) {
				out = append(out, byteCR)
			} else {
				out = append(out, byteCtrlU)
			}
		}
		segStart = i + 1
	}
	// Trailing partial (still typing): echo it and buffer it for the next chunk.
	if segStart < len(p) {
		out = append(out, p[segStart:]...)
		g.lineBuf.feedLines(p[segStart:])
	}
	return out
}
```
Wire `gateInput` into the read loop so its result is what gets sent to `g.incoming`. Keep the existing Ctrl-C-while-paused terminate behavior (`termmanager.go:264-266`) intact — the gate path only runs when `dataFlowOn`.

> Note on CRLF: feeding `p[segStart:i+1]` per terminator means a `\r\n` pair is processed as two segments — the `\r` yields the real line, the following `\n` yields a blank (0 lines). To avoid emitting a spurious extra newline for the `\n` half, skip a `\n` that immediately follows a `\r` (track the previous byte). Add a test `feedLines`-style for `"ls\r\n"` → exactly one approved `\r` in the output.

> Design note: `approve` is called **synchronously inside the read goroutine**, which naturally back-pressures further input until a decision returns — this is the "one command at a time" guarantee from the design. The real gate (Phase 4) must therefore not deadlock on the same mutex; it calls out over the network, not back into `TermManager` locks.

**Step 4 — Run, expect PASS.**

**Step 5 — Commit:** `feat(srv): gate completed command lines in TermManager`

---

## Phase 4 — Human approver over the `x-teleport-event` channel

Reuse the existing structured client channel (`teleport.SessionEvent = "x-teleport-event"`, `constants.go:831`; broadcast pattern at `lib/srv/sess.go:591-608`; client receive at `lib/client/client.go:680-695`).

### Task 4.1: Approval event payloads

**Files:**
- Create: `lib/srv/approval/events.go`
- Test: `lib/srv/approval/events_test.go`

**Step 1 — Failing test:** round-trip JSON marshal/unmarshal of `CommandApprovalRequest` and `CommandApprovalResponse` (fields: `id, session_id, command, participant, login, server_id, expires_at` / `id, approved, reason`). Assert an unknown/zero `id` response is rejected by a matcher helper.

**Step 2 — Run, expect FAIL.**

**Step 3 — Implement** the two structs with JSON tags and a `newApprovalID()` helper (monotonic counter + session id; do NOT use time/rand per harness constraints — use an incrementing counter seeded from a field).

**Step 4 — PASS. Step 5 — Commit:** `feat(approval): add approval request/response events`

---

### Task 4.2: `humanApprover` implementation

**Files:**
- Create: `lib/srv/approval/human.go`
- Test: `lib/srv/approval/human_test.go`

**Step 1 — Failing test** using a fake broadcaster + injected response:
```go
func TestHumanApproverFirstResponseWins(t *testing.T) {
	bc := &fakeBroadcaster{}
	h := newHumanApprover(bc)
	go func() {
		req := bc.waitForBroadcast()
		h.submit(req.ID, true, "ok", "bob", types.SessionModeratorMode)
	}()
	d := h.Approve(context.Background(), CommandRequest{Command: "ls"})
	require.True(t, d.Approved)
	require.Equal(t, "bob", d.Approver)
}

func TestHumanApproverRejectsNonModeratorResponse(t *testing.T) {
	bc := &fakeBroadcaster{}
	h := newHumanApprover(bc)
	go func() {
		req := bc.waitForBroadcast()
		h.submit(req.ID, true, "", "mallory", types.SessionPeerMode) // not a moderator
		h.submit(req.ID, false, "no", "bob", types.SessionModeratorMode)
	}()
	d := h.Approve(context.Background(), CommandRequest{Command: "rm"})
	require.False(t, d.Approved, "peer response must be ignored; moderator deny wins")
}
```

**Step 2 — Run, expect FAIL.**

**Step 3 — Implement** `humanApprover`:
- `Approve` builds a `CommandApprovalRequest`, calls `broadcaster.BroadcastApprovalRequest(req)`, registers a pending channel keyed by `id`, blocks on it (or `ctx.Done`).
- `submit(id, approved, reason, user, mode)` validates `mode == types.SessionModeratorMode` server-side (defense in depth), then delivers the first valid response to the channel.
- `broadcaster` is an interface (`BroadcastApprovalRequest(CommandApprovalRequest) error`) so the session supplies the real SSH-channel fan-out and tests supply a fake.

**Step 4 — PASS. Step 5 — Commit:** `feat(approval): add human moderator approver`

---

### Task 4.3: Session broadcaster + response routing

**Files:**
- Modify: `lib/srv/sess.go` (broadcast like `:591-608`; add a session-event request handler; build the gate in `newSession`/`addParty`)
- Modify: `lib/srv/termmanager.go` (only if the gate adapter needs a hook)
- Test: `lib/srv/sess_test.go` (integration-style with the in-memory test session harness already used in this file)

**Steps (TDD):**
1. Failing test: a moderated session whose require policy has `command_approval{approver: human}` builds a `humanApprover` and sets it on `s.io` via `SetCommandGate`. Assert `s.io.commandGateEnabled()` after start.
2. Implement: in `newSession` (`sess.go:820`) after evaluating moderation, if the matched `SessionRequirePolicy` carries command-approval config, construct the approver (`approval.WithTimeout(...)`), adapt it to the `commandGate` seam, and call `s.io.SetCommandGate(...)`. The adapter's `approve(cmd)` builds the `CommandRequest` (session id, login, server id, participant) and calls the approver.
3. Implement the broadcaster: `BroadcastApprovalRequest` ranges over `s.parties` and `SendRequest(teleport.SessionEvent, false, payload)` only to parties whose `mode == types.SessionModeratorMode` (mirror `:591-608`).
4. Implement response routing: handle the inbound approval-response (a new SSH request type, e.g. `teleport.SessionApprovalResponse = "x-teleport-approval"`; register it in `constants.go`) by calling `humanApprover.submit(...)` with the **server-trusted** party mode (look up the sending party, never trust client-claimed mode).
5. Run the session tests; PASS.
6. Commit: `feat(srv): wire human command approval into SSH sessions`

> Verification gate: `go build ./lib/srv/...` and `go test ./lib/srv/ -run 'Approval|Gate|Session'`.

---

## Phase 5 — `tsh` moderator UI

Client renders the request and sends a response over the new request type. Hook into the existing client event path (`lib/client/client.go:680-695`, `EventsChannel` at `lib/client/api.go:4921-4937`).

### Task 5.1: Parse + render approval request in tsh

**Files:**
- Modify: `lib/client/client.go` (the `teleport.SessionEvent` case) and/or the session event consumer
- Modify: the `tsh` session UI (locate the moderator key-handling / event display in `lib/client/` session runner)
- Test: `lib/client/...` unit test for parse + a render-to-string function

**Steps (TDD):**
1. Failing test: a `renderApprovalPrompt(CommandApprovalRequest)` returns a string containing the participant, target login, the command, and `[a] approve / [d] deny`.
2. Implement the renderer (pure function — easy to test).
3. Failing test: feeding an approval-request event into the client event handler enqueues a UI prompt and a keypress `a` produces a `CommandApprovalResponse{approved:true}` sent via the response request type.
4. Implement: extend the session event loop to recognize approval requests, capture the next `a`/`d` (local UI state — do NOT inject into the session stream), and `SendRequest` the response. Deny optionally reads a one-line reason.
5. Run; PASS.
6. Commit: `feat(tsh): render and respond to command approval prompts`

> Note: Web UI is an explicit fast-follow (same two payloads over the Web terminal WebSocket envelope set) — not in this plan's scope. Add a `// TODO(approval): Web UI renderer` marker where the envelope types are defined.

---

## Phase 6 — AI moderator (auth RPC + evaluator + virtual participant)

> **Reuse existing inference infrastructure (decided).** Teleport already has
> `InferenceModel` + `InferenceSecret` resources (`api/types/summarizer/`,
> proto `api/proto/teleport/summarizer/v1/`), with provider clients and secret
> resolution implemented in the **enterprise submodule** `e/lib/auth/summarizer/`
> (see `openai/`, `bedrock/`, `prompts/`, `command.go`, `summarizer.go`). The AI
> moderator does NOT invent model/secret config. Instead:
> - The role's `command_approval.ai` block references an `InferenceModel` **by name**
>   (field `model`) plus the prose `policy`.
> - The auth-side evaluator resolves that `InferenceModel` (and its `InferenceSecret`)
>   and dispatches to the existing provider client (OpenAI/Bedrock).
> - **The evaluator implementation lives in the enterprise submodule
>   `e/lib/auth/summarizer/`** (e.g. `command_moderation.go`), reusing the existing
>   provider abstraction and secret resolution. The OSS tree (`lib/...`) holds only
>   the interface/seam the enterprise code implements, so OSS builds without it.
> - `e` is a **separate git submodule** — commits there are independent of the main
>   repo. When editing `e/`, commit inside the submodule and note both SHAs.

### Task 6.1: Proto — `EvaluateCommand` RPC

**Files:**
- Modify: `api/proto/teleport/legacy/client/proto/authservice.proto` (service at `:3096`)
- Regenerate: run the repo's proto generation (`make grpc` — confirm exact target in the Makefile before running)
- Test: compilation + a trivial message round-trip test in `lib/auth/`

**Steps:**
1. Add messages `EvaluateCommandRequest{ session_id, command, policy, model, participant, login, server_id, session_kind }` (where `model` is the `InferenceModel` resource name from the role) and `EvaluateCommandResponse{ approved, reasoning }` and the `rpc EvaluateCommand(...)` line to `AuthService`.
2. Run proto generation; verify generated Go compiles: `go build ./api/... ./lib/auth/...`.
3. Commit: `feat(proto): add EvaluateCommand auth RPC`

> If proto generation tooling is unavailable in the environment, STOP and surface that — do not hand-edit generated `.pb.go`.

---

### Task 6.2: AI evaluator (enterprise, reuses InferenceModel)

**Location: enterprise submodule `e/lib/auth/summarizer/`** — reuse the existing
provider clients (`openai/`, `bedrock/`), prompt helpers (`prompts/`), and
`InferenceModel`/`InferenceSecret` resolution already present there. First read
`e/lib/auth/summarizer/command.go` and `summarizer.go` to learn how an existing
feature resolves an `InferenceModel` by name, loads its `InferenceSecret`, and
dispatches to the provider client — then mirror that pattern.

**Files:**
- OSS seam: define a small interface in OSS (e.g. `lib/auth/moderation` or reuse an
  existing extension hook) that the enterprise evaluator implements, so OSS builds
  without `e/`. Mirror how summarizer is wired OSS↔enterprise.
- Create (enterprise): `e/lib/auth/summarizer/command_moderation.go` + test.

**Steps (TDD):**
1. Failing test: an `Evaluator` that, given an `InferenceModel` name + prose policy +
   command, resolves the model/secret and calls the provider, returning
   `(approved bool, reasoning string, err error)`. Use a fake/mocked provider client
   (existing summarizer tests show the pattern) — NO real API in tests. Provider error
   → returned error (caller fails closed via the OSS `WithTimeout` wrapper).
2. Implement the evaluator reusing the existing model-resolution + provider dispatch.
3. Failing test: the prompt builder clearly delimits and labels the command as
   untrusted input and embeds the prose policy; assert the command text sits inside an
   untrusted-input fence and an instruction states nothing in the command can change
   the policy (prompt-injection framing). Reuse `prompts/` helpers if suitable.
4. Implement the prompt builder + a structured/JSON output contract
   `{approved, reasoning}` so parsing is deterministic. Apply a short per-call timeout.
5. Run enterprise tests; PASS.
6. Commit **inside the `e/` submodule** (separate repo): `feat(summarizer): add AI command moderation evaluator`. Note both the submodule SHA and (later) the main-repo submodule-pointer bump.

> The role references the model by name (`command_approval.ai.model`); validation
> (Task 7.1) requires that name to be non-empty when `approver: ai`. The model/secret
> themselves are managed via the existing `tctl inference_model`/`inference_secret`
> resources — no new config resource is invented.

---

### Task 6.3: gRPC handler + authorization guard

**Files:**
- Modify: `lib/auth/grpcserver.go` (mirror `CreateSessionTracker` at `:5080-5103`)
- Modify: `lib/auth/auth_with_roles.go` (the `ServerWithRoles` method + authz)
- Test: `lib/auth/...` test asserting a caller that does not host a matching AI-policy session is rejected

**Steps (TDD):**
1. Failing test: `EvaluateCommand` from an identity that is not the hosting node (or for a session without an AI approval policy) returns `AccessDenied`.
2. Implement `GRPCServer.EvaluateCommand`: `authenticate(ctx)` → delegate to `ServerWithRoles.EvaluateCommand`.
3. Implement the authz in `auth_with_roles.go`: verify the caller is the node hosting `session_id` and that an AI command-approval policy applies; then call the enterprise evaluator (Task 6.2) via the OSS seam, passing the `InferenceModel` name from the request. Deny otherwise. (Prevents using auth as a free LLM proxy.) If no enterprise evaluator is registered (OSS build), return a clear "AI moderation is enterprise-only" error → node fails closed.
4. Run; PASS.
5. Commit: `feat(auth): add EvaluateCommand handler with hosting-node guard`

---

### Task 6.4: `aiApprover` + virtual participant

**Files:**
- Create: `lib/srv/approval/ai.go`
- Test: `lib/srv/approval/ai_test.go`
- Modify: `lib/srv/sess.go` (construct AI participant; make it satisfy `count`)
- Modify: `lib/auth/moderation/session_access.go` only if needed so a synthetic moderator counts in `FulfilledFor` (`:274`)

**Steps (TDD):**
1. Failing test: `aiApprover.Approve` calls a fake auth client `EvaluateCommand` and maps `{approved:false, reasoning}` → `Decision{Approved:false, Approver:"ai-moderator", Mode:ModeAI, Reason: reasoning}`. A client error → (relies on `WithTimeout`/wrapper) denying decision.
2. Implement `aiApprover` (holds an auth client; `Approve` builds the RPC request from `CommandRequest`).
3. Failing test (session-level): a session with `approver: ai` starts WITHOUT a human moderator (the virtual AI participant satisfies `count`) and sets an AI-backed gate on `s.io`.
4. Implement: when the matched require policy is `approver: ai`, add a synthetic moderator participant at `newSession`, build `aiApprover` → `WithTimeout` → `commandGate` adapter → `SetCommandGate`. Ensure `checkIfStartUnderLock` (`sess.go:2041-2068`) sees the AI participant as a satisfying moderator.
5. Run; PASS.
6. Commit: `feat(srv): add autonomous AI command approver`

---

## Phase 7 — Role config schema + audit events

### Task 7.1: `command_approval` policy fields

**Files:**
- Modify: `api/proto/teleport/legacy/types/types.proto` (`SessionRequirePolicy` at `:4338-4357`)
- Regenerate protos
- Modify: `api/types/role.go` (accessors + `CheckAndSetDefaults` validation)
- Test: `api/types/role_test.go`

**Steps (TDD):**
1. Failing test: a role with `require_session_join[].command_approval{enabled:true, approver:"ai"}` but empty `ai.policy` fails validation; `approver:"ai"` with a policy passes; `approver:"human"` passes without `ai`.
2. Add proto sub-message `CommandApproval{ enabled, approver, timeout, on_failure, ai{ policy, model } }` as field 7 of `SessionRequirePolicy`; regenerate. `ai.model` is the name of an existing `InferenceModel` resource (see Phase 6 note).
3. Implement validation in `CheckAndSetDefaults`: `on_failure` defaults/locks to `deny`; `timeout` default 60s; `kinds` must include only `ssh` in v1 (warn/reject `k8s` as "not yet enforced"); `approver:ai` requires non-empty `ai.policy` AND non-empty `ai.model`.
4. Run; PASS.
5. Commit: `feat(types): add command_approval to SessionRequirePolicy`

---

### Task 7.2: Audit events

**Files:**
- Modify: `lib/events/api.go` (event names, near `:312`), `lib/events/codes.go` (codes, near `:355`)
- Modify: `api/proto/teleport/legacy/types/events/events.proto` + regenerate (new event message), OR reuse a generic structured event if adding a proto event is too heavy — prefer the proper event
- Modify: emission sites in `lib/srv/sess.go` / the gate adapter
- Test: `lib/events/...` and an emission assertion in the session test

**Steps (TDD):**
1. Failing test: completing an approved command emits a `command.approval.approved` event with approver + reason; a denied one emits `command.approval.denied`; a fail-closed timeout emits `command.approval.failed`.
2. Add event names + codes:
   - `CommandApprovalPendingEvent  = "command.approval.pending"`
   - `CommandApprovalApprovedEvent = "command.approval.approved"`
   - `CommandApprovalDeniedEvent   = "command.approval.denied"`
   - `CommandApprovalFailedEvent   = "command.approval.failed"`
   - codes `T44xx` series (pick unused codes; verify against `codes.go`).
3. Define the event struct (proto) with: session/user/server metadata, command, participant, login, approver, approver_mode, reason/reasoning, model_id (AI only).
4. Emit from the gate adapter using the session `emitter` (`sess.go` `s.emitter`). Auth additionally logs AI evaluations on the auth side.
5. Run; PASS.
6. Commit: `feat(events): add command approval audit events`

---

## Phase 8 — End-to-end verification & docs

### Task 8.1: Integration test — full SSH approval loop

**Files:**
- Test: `lib/srv/regular/sshserver_test.go` or the existing SSH integration harness

**Steps:**
1. Human path: start a moderated SSH session with `approver: human`; a moderator approves one command (it runs) and denies another (it does not run, shell line cleared). Assert via recorded output + emitted events.
2. AI path: same with `approver: ai` and a **mock provider** injected on the auth test server; assert no human needed and decisions gate execution.
3. Fail-closed: provider returns error/timeout → command denied, audit `command.approval.failed`.
4. Commit: `test(srv): end-to-end command approval integration`

### Task 8.2: Docs

**Files:**
- Modify: `docs/` moderated-sessions page (locate under `docs/pages/`); add a `command_approval` section + AI moderator setup (auth `ai_moderation_config`).
- Commit: `docs: document moderated command approval`

---

## Build/verify checklist (run before declaring done)

- `go build ./lib/srv/... ./lib/auth/... ./api/... ./lib/client/...`
- `go test ./lib/srv/approval/... ./lib/srv/ ./lib/moderation/ai/... -count=1`
- `go vet ./lib/srv/approval/...`
- Backward-compat: a role WITHOUT `command_approval` behaves exactly as today (explicit test in 7.1 / 8.1).

## Known gaps (documented, out of v1 scope)

- Raw-mode/alt-screen TUIs (vim/less) — gate not adjusted; document the limitation.
- Tab-completion/history divergence between `lineBuffer` and the real shell line — best-effort; BPF `session.command` is ground truth.
- Kubernetes integration — core is ready (`CommandApprover`, events, config), Kube `TermManager` wiring is a follow-up.
- `multiApprover` (human + AI) and `on_failure: allow` — interface admits them; not built.
