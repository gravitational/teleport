# `tsh beams dev` — local development, executed on beams

`tsh beams dev` turns a beam into the **execution host** for a local git
checkout. You edit locally in any IDE; the code, every build/test command, and
every AI agent (`claude`, `codex`) run on the beam. A foreground daemon keeps
the two working trees converged and rotates the beam before its 24-hour expiry
— so to you the environment feels permanent even though every individual beam
is disposable.

The safety model in one line: **execution never happens locally.** An agent on
the beam can `rm -rf /` or `curl | bash` all it wants — it lands inside a
sandbox with a 24h maximum lifetime, domain-restricted egress support, and no
API keys (LLM traffic goes through Teleport's audited LLM app proxy). Your
laptop only ever receives file *content*, and even that is filtered (see
[What never syncs down](#what-never-syncs-down)).

## Building and logging in (dev builds from this worktree)

Build tsh from this checkout (no `make` needed; the `-lobjc` linker warning is
harmless macOS noise):

```bash
cd ~/conductor/workspaces/teleport/tripoli
go build -o build/tsh ./tool/tsh
```

Log in to a beams cluster with the built binary — e.g. with an existing
profile for `empty-sun.beams.sh`:

```bash
~/conductor/workspaces/teleport/tripoli/build/tsh login --proxy=empty-sun.beams.sh:443 --skip-version-check
~/conductor/workspaces/teleport/tripoli/build/tsh beams ls    # verify
```

`--skip-version-check` is needed because this worktree builds a prealpha
version (e.g. v19.0.0-prealpha) while the proxy may run an older release —
it currently only warns, but the flag keeps it quiet and future-proof.

### Scoping the worktree binary so nothing else is affected

Two layers to understand:

- **Which binary runs** — per-shell. The simplest isolation is an alias in
  only the terminal(s) where you're testing:

  ```bash
  alias tsh=~/conductor/workspaces/teleport/tripoli/build/tsh
  ```

  Other terminals, other worktrees, and anything invoking `tsh` by absolute
  path are untouched; each worktree builds its own `build/tsh`. To make it
  automatic when you `cd` into the worktree, install direnv and put
  `PATH_add build` in a `.envrc` at the worktree root.

  The `.beams/bin` shims don't depend on any of this: they embed the absolute
  path of the tsh binary that generated them, so they keep working in shells
  whose PATH only has the stock tsh.

- **Profile state** (`~/.tsh` — which cluster you're logged into) is shared by
  all tsh binaries on the machine; `login` switches it for the stock tsh too.
  To pin the cluster per-invocation instead, bake it into the alias:

  ```bash
  alias tsh='~/conductor/workspaces/teleport/tripoli/build/tsh --proxy=empty-sun.beams.sh'
  ```

## Quickstart

```bash
# prerequisites: logged in (tsh login --proxy=<cluster>), inside a git checkout
cd ~/repos/dummy-repo
tsh beams dev
```

First attach does the following, then stays in the foreground syncing:

1. Creates a beam (or reuses the one already attached to this checkout).
2. Seeds it: `git clone` from origin *on the beam* (your laptop never uploads
   the repo; falls back to uploading a git bundle if the clone needs
   credentials the beam doesn't have), then replays your unpushed local
   commits and copies your uncommitted files.
3. Runs `.beams/setup.sh` from the repo root, if present — this is your
   provisioning script (install deps, toolchains). It runs once per beam, not
   once per day (see [Handoff](#the-24h-problem-handoff)).
4. Installs the beam-side payload (WIP auto-push + self-handoff cron jobs).
5. Writes command shims into `.beams/bin/`.

Leave it running in a spare terminal pane. `Ctrl+C` stops the sync loop; the
workspace stays attached and `tsh beams dev` resumes it.

## Day-to-day flow

```bash
$ cd ~/repos/dummy-repo
$ vim src/server.ts              # local edit → on the beam in ~3s
$ claude                         # via shim: runs ON the beam, full history
$ tsh beams dev run -- npm test  # any command, executed on the beam
$ tsh beams dev shell            # interactive shell, cwd-mapped
$ git diff                       # local git: review what the agent wrote
$ git commit -am "fix retry"     # local commit → replayed onto the beam
$ git push                       # normal push to origin, from your laptop
```

To make the shims transparent, put the workspace's shim dir on your PATH while
inside the repo (direnv users: `PATH_add .beams/bin` in `.envrc`; or add
`export PATH=".beams/bin:$PATH"` to your shell prompt hook of choice). Typing
`claude` then just does the right thing — the habitual path and the safe path
are the same path.

## Keeping agents running while your laptop is off

`tsh beams dev run -- claude` ties Claude to your SSH session — close the
laptop and the session hangup kills it mid-task. To let the agent keep
working on the beam with no laptop attached, detach it from the session:

**tmux, for interactive sessions you want to survive (recommended default).**
The beam image ships a `.tmux.conf`, so this works out of the box:

```bash
tsh beams dev shell
tmux new -s work        # inside the beam
claude                  # run your session inside tmux
# … close your laptop whenever …
# next day:
tsh beams dev shell
tmux attach -t work     # claude is still there, mid-conversation
```

**Headless fire-and-forget, for well-defined tasks:**

```bash
tsh beams dev run -- bash -lc 'setsid nohup claude -p "run the test suite and fix all failures" --permission-mode acceptEdits > ~/task.log 2>&1 < /dev/null &'
# later:
tsh beams dev run -- tail -20 ~/task.log
```

How this composes with the rest of the machinery:

- While the laptop is off nothing mirrors down (the sync engine runs in your
  attach loop), but nothing is lost: the agent's edits accumulate on the
  beam and the dead-man WIP push snapshots them every 5 minutes (where git
  credentials allow). The next `tsh beams dev` pulls everything the agent
  did into your local tree for review.
- The automatic handoff **defers while an agent process is running** and only
  forces inside the final ~30–40 minutes of beam life. If it does force, the
  transcript up to the last completed message carries to the successor —
  resume there with `claude --continue`. The tmux session itself dies with
  the old beam; only the conversation/context survives the rotation.

## Commands

| Command | What it does |
|---|---|
| `tsh beams dev [attach] [dir]` | Attach a checkout to a beam (creating/seeding if needed) and run the sync loop in the foreground. |
| `tsh beams dev run -- <cmd>...` | Run a command on the beam, in the directory mapped from your cwd, with an interactive TTY. |
| `tsh beams dev shell` | Interactive login shell on the beam, cwd-mapped. |
| `tsh beams dev status [dir]` | Show the attachment, beam, and time to expiry. |
| `tsh beams dev detach [dir] [--rm-beam]` | Forget the attachment (and optionally delete the beam). |
| `tsh beams dev handoff [dir]` | Rotate onto a fresh beam **now** — same path the TTL watcher takes automatically; great for demos. |

Attach flags: `--interval` (sync cadence, default 3s), `--auto-handoff`
(default true), `--no-shims`.

## How sync works

Two channels, both incremental:

- **Commit channel (git bundles).** Committed history moves as delta git
  bundles in both directions — a large repo never re-transfers. Agent commits
  made on the beam land on the local tracking ref `refs/remotes/beam/<branch>`
  and are fast-forwarded into your branch only when that's trivially safe;
  divergence is reported, never merged silently.
- **Working-tree channel (tar deltas).** The *uncommitted* dirty set (per
  `git status`) is compared by mtime/size locally and content hash remotely;
  only changed files travel, as small tarballs over SFTP.

**Conflict policy:** if the same file changed on both sides in the same
window, **local wins** and you get a warning. The human's editor beats the
agent.

**Claude session context:** the beam's `~/.claude/projects` transcripts are
mirrored to the laptop every minute and restored to every successor beam at
the same repo path — so after any rotation, `claude --continue` resumes the
same conversation with **full context**, not a summary.

### What never syncs down

Files that auto-execute in local tooling are never synced from the beam to
your laptop, even if the agent writes them: currently `.envrc` and
`.vscode/tasks.json`. You get a one-time warning instead; copy them manually
if they're intentional. `.git/` internals (hooks!) never travel on the file
channel at all — git history moves only via bundle fetch, which cannot carry
hooks. `.beams/bin/` (the shims) is local-only and hidden from git via
`.git/info/exclude`.

## The 24h problem: handoff

Beams have a hard, non-extendable 24h TTL. `tsh beams dev` treats that as a
scheduling problem:

- **Warm handoff (laptop attached — the common case).** At T-100 minutes the
  client triggers the beam's handoff script: the dying beam *itself* creates
  a successor with its own delegated identity (`tsh` is preinstalled and
  authenticated on every beam), tars its repo — including installed
  `node_modules`/build caches — plus Claude sessions and the payload straight
  to the successor beam-to-beam, and bootstraps it. The client then repoints
  the workspace and deletes the old beam. You keep typing; nothing re-installs.
- **Self-handoff (laptop off).** The same script runs from cron on the beam at
  T-90 minutes with no laptop involved, and leaves a successor pointer. Next
  time you run `tsh beams dev`, the client finds the recorded beam gone, scans
  your beams for a workspace marker, and adopts the successor — environment,
  uncommitted work, and Claude context intact. Chains survive multiple
  expiries: each successor reinstalls the cron jobs.
- **Cold resurrection (no successor found).** A fresh beam is created and
  seeded from origin + your local mirror, and `.beams/setup.sh` re-runs. Code
  and locally mirrored Claude context are never lost in any scenario; only
  the installed environment pays the rebuild cost here.

**Beam-count invariant: a workspace never holds more than two beams** — the
current one plus, briefly, its in-flight successor:

- The handoff records the successor in `successor.pending` the moment it is
  created. Retries resume seeding *that* beam instead of creating another;
  after 3 failed seeding attempts it is deleted before a replacement is made.
- The successor is stamped with a `claim` file before any data lands, so even
  a maximally unlucky crash leaves an attributable beam that the next
  `tsh beams dev` reuses (re-running setup) rather than a mystery orphan.
- After a completed self-handoff the drained predecessor deletes itself (once
  no agent process remains); in a warm handoff the client deletes it.
- The attach-time successor scan deletes any stray beams still marked or
  claimed for the workspace beyond the one it adopts.
- Every `tsh`/`git` call in the beam-side scripts is wrapped in `timeout`, so
  a hung call becomes a retryable failure instead of wedging the poller.

Backstopping all of that, a **dead-man's push** runs on the beam every 5
minutes: it snapshots the working tree (via `git stash create`, no side
effects) and pushes it to `refs/heads/beams-wip/<workspace-id>` on origin —so
even agent work done after your laptop went to sleep survives a beam that
dies with no successor.

## The setup script

Put provisioning steps in `.beams/setup.sh` at the repo root (tracked in git):

```bash
#!/bin/bash
set -euo pipefail
sudo apt-get update && sudo apt-get install -y build-essential
corepack enable && pnpm install
```

It runs on fresh beams only — warm/self handoffs carry the environment across
instead of re-running it.

## State on disk

| Where | What |
|---|---|
| `~/.tsh/beams_dev/<id>.json` (laptop) | Workspace record: local dir ⇄ beam binding, branch, origin. |
| `~/.tsh/beams_dev/<id>/` (laptop) | Mirrored Claude transcripts, transfer temp files. |
| `<repo>/.beams/bin/` (laptop) | Command shims (git-ignored via `.git/info/exclude`). |
| `/home/beams/.beams-dev/` (beam) | Payload scripts, `meta.env`, workspace marker, successor pointer, handoff log. |

## Current limitations (MVP)

- The sync loop is polling-based (default 3s) over per-cycle SSH execs; a
  persistent SSH session + filesystem watchers would cut latency and load.
- Divergent branches (you and the agent both committed) require a manual
  rebase/merge from `refs/remotes/beam/<branch>` — by design, but there's no
  helper command yet.
- Self-handoff assumes the beam's delegated identity may call
  `tsh beams add` and that `cron`/`flock` exist on the beam image (there is a
  `setsid` fallback for cron-less images). Verify both on your cluster:
  SSH into a beam and run `tsh status && tsh beams add --no-console`.
- Handoffs briefly count two beams against the 20-concurrent/5-per-minute
  tenant caps.
- `tsh beams dev run` always allocates a TTY; piping data through it is not
  supported yet.
- Renames in the working tree sync as delete+add.
- Sync memory is in-process: deleting an **untracked** file while the sync
  loop is *not* running leaves the beam's copy in place, and it will sync
  back on the next attach (git reports no deletion event for untracked
  files). Delete it again with the loop running, or remove it on the beam
  too (`tsh beams dev run -- rm <path>`).

## Trying it end to end

```bash
cd ~/repos/dummy-repo
tsh beams dev                       # terminal 1: attach + sync loop
# terminal 2:
echo test >> notes.md               # appears on the beam within ~3s
tsh beams dev run -- git status     # see it from the beam's side
tsh beams dev run -- claude         # agent session on the beam
tsh beams dev status                # expiry countdown
tsh beams dev detach --rm-beam      # tear everything down
```
