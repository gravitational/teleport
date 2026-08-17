/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// The sync engine keeps a local git worktree and its checkout on a beam
// converged over two channels:
//
//   - the commit channel moves committed history as git bundles (delta-only,
//     both directions), so large repos never re-transfer;
//   - the tree channel moves the *uncommitted* working set (modified +
//     untracked files, per `git status`) as tarballs of changed files.
//
// Conflict policy is deliberately simple and asymmetric: a path dirty on both
// sides within the same window is won by the local side (the human), with a
// warning. Sync state (stamps/hashes as of the last converged pass) lives in
// memory only — a restart just causes one full re-scan pass.
type devSyncEngine struct {
	cf       *CLIConf
	ws       *devWorkspace
	runner   *beamRunner
	stateDir string

	// localStamps records each dirty path's (mtime,size) as of the last time
	// the engine considered it synced. A differing stamp means "changed since
	// last sync" in the up direction.
	localStamps map[string]localStamp
	// localShas caches content hashes keyed by stamp to avoid re-hashing
	// unchanged files every cycle.
	localShas map[string]stampedSha
	// remoteShas records each remote dirty path's content hash as of the last
	// observation.
	remoteShas map[string]string

	warnedOnce map[string]bool

	lastClaudeMirror time.Time
}

type localStamp struct {
	mtime int64
	size  int64
}

type stampedSha struct {
	stamp localStamp
	sha   string
}

// downSyncDenylist are files that auto-execute in local tooling (shell/IDE)
// when present. The agent on the beam may legitimately write them, but they
// never sync down automatically — the beam is the untrusted side.
var downSyncDenylist = map[string]bool{
	".envrc":             true,
	".vscode/tasks.json": true,
}

func newDevSyncEngine(cf *CLIConf, ws *devWorkspace, runner *beamRunner) (*devSyncEngine, error) {
	stateDir, err := beamsDevWorkspaceStateDir(ws.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &devSyncEngine{
		cf:          cf,
		ws:          ws,
		runner:      runner,
		stateDir:    stateDir,
		localStamps: make(map[string]localStamp),
		localShas:   make(map[string]stampedSha),
		remoteShas:  make(map[string]string),
		warnedOnce:  make(map[string]bool),
	}, nil
}

func (e *devSyncEngine) logf(format string, args ...any) {
	fmt.Fprintf(e.cf.Stdout(), format+"\n", args...)
}

func (e *devSyncEngine) warnOncef(key, format string, args ...any) {
	if e.warnedOnce[key] {
		return
	}
	e.warnedOnce[key] = true
	e.logf("! "+format, args...)
}

// syncExcluded reports paths that never travel on either channel.
func syncExcluded(path string) bool {
	return path == ".git" || strings.HasPrefix(path, ".git/") ||
		path == ".beams/bin" || strings.HasPrefix(path, ".beams/bin/") ||
		isMacJunk(path)
}

// isMacJunk reports macOS metadata artifacts (AppleDouble sidecars, Finder
// droppings) that must never sync in either direction.
func isMacJunk(path string) bool {
	base := path
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.HasPrefix(base, "._") || base == ".DS_Store"
}

func (e *devSyncEngine) remoteTmp(name string) string {
	return fmt.Sprintf("/tmp/beams-dev-%s-%s", e.ws.ID, name)
}

func (e *devSyncEngine) quotedRemoteDir() string {
	return beamsDevShellQuote(e.ws.RemoteDir)
}

// ---------------------------------------------------------------------------
// Seeding

// seed makes a fresh (or newly adopted) beam converge with the local checkout:
// clone, replay local-only commits, push the dirty working set, and restore
// mirrored Claude session transcripts.
func (e *devSyncEngine) seed(ctx context.Context) error {
	exists, err := e.runner.output(ctx, fmt.Sprintf("[ -d %s/.git ] && echo yes || echo no", e.quotedRemoteDir()))
	if err != nil {
		return trace.Wrap(err)
	}

	if strings.TrimSpace(exists) != "yes" {
		if err := e.cloneOnBeam(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := e.pushCommits(ctx, true); err != nil {
		return trace.Wrap(err)
	}
	if _, err := e.pushWorkingTree(ctx); err != nil {
		return trace.Wrap(err)
	}
	if err := e.restoreClaudeSessions(ctx); err != nil {
		// Session restore is best-effort: losing it degrades context, not code.
		e.logf("! could not restore Claude sessions to beam: %v", err)
	}
	return nil
}

// cloneOnBeam prefers cloning from origin (datacenter speed, no laptop
// upload); when that fails (private repo, no credentials on the beam yet) it
// falls back to uploading a full git bundle from the local checkout.
func (e *devSyncEngine) cloneOnBeam(ctx context.Context) error {
	if e.ws.OriginURL != "" {
		e.logf("Cloning %s on beam %q...", e.ws.OriginURL, e.ws.BeamAlias)
		_, err := e.runner.output(ctx, fmt.Sprintf("git clone %s %s",
			beamsDevShellQuote(e.ws.OriginURL), e.quotedRemoteDir()))
		if err == nil {
			return nil
		}
		e.logf("! clone from origin failed (private repo without credentials on the beam?), falling back to uploading a bundle: %v", err)
	}

	e.logf("Uploading full repo bundle to beam %q (first seed only)...", e.ws.BeamAlias)
	bundle := filepath.Join(e.stateDir, "seed.bundle")
	defer os.Remove(bundle)
	if _, err := runLocalGit(ctx, e.ws.LocalDir, "bundle", "create", bundle, "--all"); err != nil {
		return trace.Wrap(err)
	}
	remoteBundle := e.remoteTmp("seed.bundle")
	if err := e.runner.upload(ctx, bundle, remoteBundle); err != nil {
		return trace.Wrap(err)
	}

	fixOrigin := "git remote remove origin"
	if e.ws.OriginURL != "" {
		fixOrigin = "git remote set-url origin " + beamsDevShellQuote(e.ws.OriginURL)
	}
	_, err := e.runner.output(ctx, fmt.Sprintf(
		"git clone %s %s && cd %s && %s; rm -f %s",
		beamsDevShellQuote(remoteBundle), e.quotedRemoteDir(), e.quotedRemoteDir(), fixOrigin, beamsDevShellQuote(remoteBundle)))
	return trace.Wrap(err)
}

// restoreClaudeSessions uploads the locally mirrored Claude transcripts (from
// the previous beam) so `claude --continue` on the new beam resumes with full
// context.
func (e *devSyncEngine) restoreClaudeSessions(ctx context.Context) error {
	mirror := filepath.Join(e.stateDir, "claude")
	if _, err := os.Stat(filepath.Join(mirror, "projects")); err != nil {
		return nil // nothing mirrored yet
	}
	archive := filepath.Join(e.stateDir, "claude-restore.tgz")
	defer os.Remove(archive)
	if _, err := runLocal(ctx, mirror, "tar", "--exclude", "._*", "--exclude", ".DS_Store", "-czf", archive, "projects"); err != nil {
		return trace.Wrap(err)
	}
	remoteArchive := e.remoteTmp("claude-restore.tgz")
	if err := e.runner.upload(ctx, archive, remoteArchive); err != nil {
		return trace.Wrap(err)
	}
	_, err := e.runner.output(ctx, fmt.Sprintf(
		"mkdir -p /home/beams/.claude && tar -xzf %s -C /home/beams/.claude && rm -f %s",
		remoteArchive, remoteArchive))
	if err == nil {
		e.logf("Restored Claude session transcripts to beam %q.", e.ws.BeamAlias)
	}
	return trace.Wrap(err)
}

// ---------------------------------------------------------------------------
// One sync cycle

func (e *devSyncEngine) syncCycle(ctx context.Context) {
	// Each direction is independently best-effort: a transient SSH failure in
	// one channel must not stall the others or kill the loop.
	if err := e.pushCommits(ctx, false); err != nil {
		e.logf("! commit up-sync: %v", err)
	}
	if err := e.pullCommits(ctx); err != nil {
		e.logf("! commit down-sync: %v", err)
	}
	dirty, err := e.localDirtySet(ctx)
	if err != nil {
		e.logf("! reading local git status: %v", err)
		return
	}
	if _, err := e.pushWorkingTreeWith(ctx, dirty); err != nil {
		e.logf("! working-tree up-sync: %v", err)
	}
	if err := e.pullWorkingTree(ctx, dirty); err != nil {
		e.logf("! working-tree down-sync: %v", err)
	}
	if time.Since(e.lastClaudeMirror) > time.Minute {
		if err := e.mirrorClaudeSessions(ctx); err != nil {
			e.warnOncef("claude-mirror", "mirroring Claude sessions: %v", err)
		}
		e.lastClaudeMirror = time.Now()
	}
}

// ---------------------------------------------------------------------------
// Commit channel (git bundles)

// pushCommits replays local-only commits onto the beam. With force set (seed
// and branch switches) the remote branch is checked out fresh; otherwise only
// fast-forwards are applied so agent work in flight is never clobbered.
func (e *devSyncEngine) pushCommits(ctx context.Context, force bool) error {
	branch, err := runLocalGit(ctx, e.ws.LocalDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return trace.Wrap(err)
	}
	if branch == "HEAD" {
		e.warnOncef("detached", "local checkout is on a detached HEAD; commit channel paused")
		return nil
	}
	if branch != e.ws.Branch {
		e.logf("Local branch changed %s -> %s; switching beam checkout.", e.ws.Branch, branch)
		e.ws.Branch = branch
		force = true
		_ = saveDevWorkspace(e.ws)
	}

	localHead, err := runLocalGit(ctx, e.ws.LocalDir, "rev-parse", "HEAD")
	if err != nil {
		return trace.Wrap(err)
	}
	remoteHead, err := e.remoteHead(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if remoteHead == localHead && !force {
		return nil
	}

	// Pick a bundle basis the beam is known to have, to keep bundles minimal.
	var basis string
	if remoteHead != "" && e.localHasCommit(ctx, remoteHead) {
		if !force {
			ancestor, err := e.localIsAncestor(ctx, remoteHead, localHead)
			if err != nil {
				return trace.Wrap(err)
			}
			if !ancestor {
				e.warnOncef("diverged-"+remoteHead, "beam has commits not on the local branch; not pushing (down-channel will surface them)")
				return nil
			}
		}
		basis = remoteHead
	}

	bundle := filepath.Join(e.stateDir, "up.bundle")
	defer os.Remove(bundle)
	rangeSpec := branch
	if basis != "" {
		rangeSpec = basis + ".." + branch
	} else {
		// No shared basis: exclude whatever origin already has so a freshly
		// origin-cloned beam doesn't receive the whole history again.
		if _, err := runLocalGit(ctx, e.ws.LocalDir, "bundle", "create", bundle, branch, "--not", "--remotes"); err == nil {
			return trace.Wrap(e.applyBundleOnBeam(ctx, bundle, branch, force))
		}
	}
	if _, err := runLocalGit(ctx, e.ws.LocalDir, "bundle", "create", bundle, rangeSpec); err != nil {
		if strings.Contains(err.Error(), "empty bundle") {
			// Nothing local-only; still honor force checkouts (branch switch).
			if force {
				_, err := e.runner.output(ctx, fmt.Sprintf(
					"cd %s && (git checkout %s 2>/dev/null || git checkout -b %s --track origin/%s)",
					e.quotedRemoteDir(), beamsDevShellQuote(branch), beamsDevShellQuote(branch), beamsDevShellQuote(branch)))
				return trace.Wrap(err)
			}
			return nil
		}
		return trace.Wrap(err)
	}
	return trace.Wrap(e.applyBundleOnBeam(ctx, bundle, branch, force))
}

func (e *devSyncEngine) applyBundleOnBeam(ctx context.Context, bundle, branch string, force bool) error {
	remoteBundle := e.remoteTmp("up.bundle")
	if err := e.runner.upload(ctx, bundle, remoteBundle); err != nil {
		return trace.Wrap(err)
	}
	apply := "git merge --ff-only beams-dev/incoming"
	if force {
		apply = fmt.Sprintf("git checkout -B %s beams-dev/incoming", beamsDevShellQuote(branch))
	}
	out, err := e.runner.output(ctx, fmt.Sprintf(
		"cd %s && git fetch %s +%s:refs/heads/beams-dev/incoming && %s && git branch -D beams-dev/incoming 2>/dev/null; rm -f %s && git rev-parse HEAD",
		e.quotedRemoteDir(), beamsDevShellQuote(remoteBundle), beamsDevShellQuote(branch), apply, beamsDevShellQuote(remoteBundle)))
	if err != nil {
		return trace.Wrap(err)
	}
	e.logf("↑ commits: beam now at %.8s", strings.TrimSpace(lastLine(out)))
	return nil
}

// pullCommits brings agent-made commits down. They land on a tracking ref
// (refs/remotes/beam/<branch>) and are only fast-forwarded into the local
// branch automatically; anything else is surfaced, never merged silently.
func (e *devSyncEngine) pullCommits(ctx context.Context) error {
	remoteHead, err := e.remoteHead(ctx)
	if err != nil || remoteHead == "" {
		return trace.Wrap(err)
	}
	localHead, err := runLocalGit(ctx, e.ws.LocalDir, "rev-parse", "HEAD")
	if err != nil {
		return trace.Wrap(err)
	}
	if remoteHead == localHead {
		return nil
	}

	if !e.localHasCommit(ctx, remoteHead) {
		// Fetch the missing commits as a bundle based on what the beam knows
		// we have.
		basis := ""
		if out, err := e.runner.output(ctx, fmt.Sprintf(
			"cd %s && git cat-file -e %s 2>/dev/null && echo yes || echo no",
			e.quotedRemoteDir(), beamsDevShellQuote(localHead))); err == nil && strings.TrimSpace(lastLine(out)) == "yes" {
			basis = localHead
		}
		rangeSpec := "HEAD"
		if basis != "" {
			rangeSpec = basis + "..HEAD"
		}
		remoteBundle := e.remoteTmp("down.bundle")
		if _, err := e.runner.output(ctx, fmt.Sprintf(
			"cd %s && git bundle create %s %s",
			e.quotedRemoteDir(), beamsDevShellQuote(remoteBundle), rangeSpec)); err != nil {
			return trace.Wrap(err)
		}
		bundle := filepath.Join(e.stateDir, "down.bundle")
		defer os.Remove(bundle)
		if err := e.runner.download(ctx, remoteBundle, bundle); err != nil {
			return trace.Wrap(err)
		}
		_, _ = e.runner.output(ctx, "rm -f "+beamsDevShellQuote(remoteBundle))
		if _, err := runLocalGit(ctx, e.ws.LocalDir, "fetch", bundle, "HEAD"); err != nil {
			return trace.Wrap(err)
		}
	}

	trackingRef := "refs/remotes/beam/" + e.ws.Branch
	if _, err := runLocalGit(ctx, e.ws.LocalDir, "update-ref", trackingRef, remoteHead); err != nil {
		return trace.Wrap(err)
	}

	ancestor, err := e.localIsAncestor(ctx, localHead, remoteHead)
	if err != nil {
		return trace.Wrap(err)
	}
	if ancestor {
		if _, err := runLocalGit(ctx, e.ws.LocalDir, "merge", "--ff-only", remoteHead); err != nil {
			e.warnOncef("ff-"+remoteHead, "beam committed %.8s but local fast-forward failed (%v); inspect ref %s", remoteHead, err, trackingRef)
			return nil
		}
		e.logf("↓ commits: local fast-forwarded to %.8s", remoteHead)
		return nil
	}
	e.warnOncef("diverged-local-"+remoteHead,
		"beam branch diverged from local (beam %.8s); beam commits are on %s — rebase or merge manually", remoteHead, trackingRef)
	return nil
}

func (e *devSyncEngine) remoteHead(ctx context.Context) (string, error) {
	out, err := e.runner.output(ctx, fmt.Sprintf("cd %s && git rev-parse HEAD", e.quotedRemoteDir()))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimSpace(lastLine(out)), nil
}

func (e *devSyncEngine) localHasCommit(ctx context.Context, sha string) bool {
	_, err := runLocalGit(ctx, e.ws.LocalDir, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

func (e *devSyncEngine) localIsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := runLocalGit(ctx, e.ws.LocalDir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	// merge-base --is-ancestor exits 1 for "no": indistinguishable from other
	// failures via error type, so treat any error as "no" — callers only use
	// this to decide whether a fast-forward is safe.
	return false, nil
}

// ---------------------------------------------------------------------------
// Working-tree channel (tar deltas of the dirty set)

type dirtyEntry struct {
	deleted bool
}

// localDirtySet parses `git status --porcelain=v1 -z` into path -> state.
func (e *devSyncEngine) localDirtySet(ctx context.Context) (map[string]dirtyEntry, error) {
	// -uall lists files inside untracked directories individually; without it
	// a brand-new directory syncs as nothing (the dir entry stats as a dir and
	// is skipped). Raw output: the first entry may legitimately start with a
	// space (unstaged status in column X) that trimming would destroy.
	out, err := runLocalGitRaw(ctx, e.ws.LocalDir, "status", "--porcelain=v1", "-z", "-uall")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dirty := make(map[string]dirtyEntry)
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		status, path := entry[:2], entry[3:]
		if strings.ContainsAny(status, "RC") {
			// Rename/copy entries carry the origin path in the next field;
			// treat as delete(origin)+add(target).
			if i+1 < len(fields) {
				i++
				dirty[fields[i]] = dirtyEntry{deleted: true}
			}
		}
		if syncExcluded(path) {
			continue
		}
		deleted := strings.Contains(status, "D")
		dirty[path] = dirtyEntry{deleted: deleted}
	}
	return dirty, nil
}

func (e *devSyncEngine) pushWorkingTree(ctx context.Context) (int, error) {
	dirty, err := e.localDirtySet(ctx)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return e.pushWorkingTreeWith(ctx, dirty)
}

func (e *devSyncEngine) pushWorkingTreeWith(ctx context.Context, dirty map[string]dirtyEntry) (int, error) {
	var changed, removed []string

	for path, entry := range dirty {
		if entry.deleted {
			if _, tracked := e.localStamps[path]; tracked || len(e.localStamps) == 0 {
				removed = append(removed, path)
			}
			delete(e.localStamps, path)
			continue
		}
		stamp, ok := e.statLocal(path)
		if !ok {
			continue
		}
		if prev, seen := e.localStamps[path]; !seen || prev != stamp {
			changed = append(changed, path)
		}
	}

	// Paths that left the dirty set were committed or reverted; push their
	// current (HEAD) content ONCE so a beam-side stale dirty copy converges.
	// They must not get a fresh stamp afterwards — a stamped path absent from
	// the dirty set reads as "departed" again next cycle, re-pushing forever.
	var departed []string
	for path := range e.localStamps {
		if _, still := dirty[path]; still {
			continue
		}
		delete(e.localStamps, path)
		delete(e.remoteShas, path)
		if _, ok := e.statLocal(path); ok {
			departed = append(departed, path)
		} else {
			// Departed AND gone from disk: a locally deleted UNTRACKED file.
			// git status never reports these (only tracked deletions get a
			// ` D` entry), so this departure is the only deletion signal —
			// propagate it, or the beam's copy resurrects via the
			// down-channel.
			removed = append(removed, path)
		}
	}

	if len(changed) == 0 && len(departed) == 0 && len(removed) == 0 {
		return 0, nil
	}

	if len(removed) > 0 {
		quoted := make([]string, 0, len(removed))
		for _, p := range removed {
			quoted = append(quoted, beamsDevShellQuote(p))
			delete(e.remoteShas, p)
		}
		if _, err := e.runner.output(ctx, fmt.Sprintf("cd %s && rm -rf -- %s",
			e.quotedRemoteDir(), strings.Join(quoted, " "))); err != nil {
			return 0, trace.Wrap(err)
		}
	}

	transfer := append(append([]string{}, changed...), departed...)
	if len(transfer) > 0 {
		if err := e.transferUp(ctx, transfer); err != nil {
			return 0, trace.Wrap(err)
		}
		// Only still-dirty paths get a stamp; departed paths are clean now
		// and tracked by the commit channel. Both record the pushed content
		// hash so the down-channel knows the beam copy matches.
		for _, path := range transfer {
			stamp, ok := e.statLocal(path)
			if !ok {
				continue
			}
			if sha, err := e.localSha(path, stamp); err == nil {
				e.remoteShas[path] = sha
			}
		}
		for _, path := range changed {
			if stamp, ok := e.statLocal(path); ok {
				e.localStamps[path] = stamp
			}
		}
	}
	e.logf("↑ %d file(s)%s, %d deletion(s)%s",
		len(transfer), summarizePaths(transfer), len(removed), summarizePaths(removed))
	return len(transfer) + len(removed), nil
}

// summarizePaths renders up to three example paths for a sync log line.
func summarizePaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	shown := paths
	suffix := ""
	if len(shown) > 3 {
		shown = shown[:3]
		suffix = fmt.Sprintf(", +%d more", len(paths)-3)
	}
	return " [" + strings.Join(shown, ", ") + suffix + "]"
}

func (e *devSyncEngine) transferUp(ctx context.Context, paths []string) error {
	list := filepath.Join(e.stateDir, "up.list")
	if err := os.WriteFile(list, []byte(strings.Join(paths, "\n")+"\n"), 0o600); err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(list)
	archive := filepath.Join(e.stateDir, "up.tgz")
	defer os.Remove(archive)
	if _, err := runLocal(ctx, e.ws.LocalDir, "tar", "-czf", archive, "-C", e.ws.LocalDir, "-T", list); err != nil {
		return trace.Wrap(err)
	}
	remoteArchive := e.remoteTmp("up.tgz")
	if err := e.runner.upload(ctx, archive, remoteArchive); err != nil {
		return trace.Wrap(err)
	}
	_, err := e.runner.output(ctx, fmt.Sprintf("mkdir -p %s && tar -xzf %s -C %s && rm -f %s",
		e.quotedRemoteDir(), remoteArchive, e.quotedRemoteDir(), remoteArchive))
	return trace.Wrap(err)
}

// remoteDirtyListing is a single remote pass emitting `sha256 path` lines for
// every dirty/untracked file, then a deletion marker section.
const remoteDirtyListingCmd = `cd %s && { git diff --name-only HEAD -- 2>/dev/null; git ls-files --others --exclude-standard; } | sort -u | while IFS= read -r f; do [ -f "$f" ] && printf '%%s %%s\n' "$(sha256sum -- "$f" | cut -d' ' -f1)" "$f"; done; echo '===DELETED==='; git ls-files --deleted`

func (e *devSyncEngine) pullWorkingTree(ctx context.Context, localDirty map[string]dirtyEntry) error {
	out, err := e.runner.output(ctx, fmt.Sprintf(remoteDirtyListingCmd, e.quotedRemoteDir()))
	if err != nil {
		return trace.Wrap(err)
	}

	remote := make(map[string]string)
	var remoteDeleted []string
	inDeleted := false
	for line := range strings.Lines(out) {
		line = strings.TrimRight(line, "\n")
		if line == "===DELETED===" {
			inDeleted = true
			continue
		}
		if line == "" {
			continue
		}
		if inDeleted {
			remoteDeleted = append(remoteDeleted, line)
			continue
		}
		sha, path, ok := strings.Cut(line, " ")
		if !ok || len(sha) != 64 {
			continue
		}
		remote[path] = sha
	}

	// Remove macOS metadata junk that earlier uploads may have materialized on
	// the beam (pre-COPYFILE_DISABLE tarballs); it must not keep churning the
	// dirty listing.
	var junk []string
	for path := range remote {
		if isMacJunk(path) {
			junk = append(junk, beamsDevShellQuote(path))
		}
	}
	if len(junk) > 0 {
		if _, err := e.runner.output(ctx, fmt.Sprintf("cd %s && rm -f -- %s",
			e.quotedRemoteDir(), strings.Join(junk, " "))); err == nil {
			e.logf("cleaned %d macOS metadata file(s) off the beam", len(junk))
		}
	}

	var toDownload []string
	for path, rsha := range remote {
		if syncExcluded(path) {
			continue
		}
		if downSyncDenylist[path] {
			e.warnOncef("deny-"+path, "beam wrote %s, which can auto-execute locally; NOT syncing it down (copy manually if intended)", path)
			continue
		}
		if e.remoteShas[path] == rsha {
			continue // already converged with this remote content
		}
		if _, dirtyLocal := localDirty[path]; dirtyLocal {
			// Both sides changed the file: local wins; the up-channel will
			// overwrite the beam's copy on the next stamp change.
			e.warnOncef("conflict-"+path+"-"+rsha, "conflict on %s (changed locally and on beam); keeping local version", path)
			continue
		}
		stamp, ok := e.statLocal(path)
		if ok {
			if lsha, err := e.localSha(path, stamp); err == nil && lsha == rsha {
				e.remoteShas[path] = rsha
				continue
			}
		}
		toDownload = append(toDownload, path)
	}

	deletedLocally := 0
	for _, path := range remoteDeleted {
		if syncExcluded(path) {
			continue
		}
		if _, dirtyLocal := localDirty[path]; dirtyLocal {
			e.warnOncef("conflict-del-"+path, "beam deleted %s but it is modified locally; keeping local version", path)
			continue
		}
		if _, ok := e.statLocal(path); !ok {
			continue
		}
		if err := os.Remove(filepath.Join(e.ws.LocalDir, path)); err == nil {
			deletedLocally++
			delete(e.localStamps, path)
			delete(e.remoteShas, path)
		}
	}

	if len(toDownload) > 0 {
		if err := e.transferDown(ctx, toDownload); err != nil {
			return trace.Wrap(err)
		}
		for _, path := range toDownload {
			e.remoteShas[path] = remote[path]
			// Record the post-download stamp so the up-channel doesn't echo
			// the file straight back to the beam.
			if stamp, ok := e.statLocal(path); ok {
				e.localStamps[path] = stamp
			}
		}
	}
	if len(toDownload) > 0 || deletedLocally > 0 {
		e.logf("↓ %d file(s)%s, %d deletion(s)", len(toDownload), summarizePaths(toDownload), deletedLocally)
	}
	return nil
}

func (e *devSyncEngine) transferDown(ctx context.Context, paths []string) error {
	quoted := make([]string, 0, len(paths))
	for _, p := range paths {
		quoted = append(quoted, beamsDevShellQuote(p))
	}
	remoteArchive := e.remoteTmp("down.tgz")
	if _, err := e.runner.output(ctx, fmt.Sprintf("cd %s && tar -czf %s -- %s",
		e.quotedRemoteDir(), remoteArchive, strings.Join(quoted, " "))); err != nil {
		return trace.Wrap(err)
	}
	archive := filepath.Join(e.stateDir, "down.tgz")
	defer os.Remove(archive)
	if err := e.runner.download(ctx, remoteArchive, archive); err != nil {
		return trace.Wrap(err)
	}
	_, _ = e.runner.output(ctx, "rm -f "+beamsDevShellQuote(remoteArchive))
	_, err := runLocal(ctx, e.ws.LocalDir, "tar", "-xzf", archive, "-C", e.ws.LocalDir)
	return trace.Wrap(err)
}

// mirrorClaudeSessions pulls the beam's Claude transcripts into local state so
// a dead beam never takes agent context with it.
func (e *devSyncEngine) mirrorClaudeSessions(ctx context.Context) error {
	remoteArchive := e.remoteTmp("claude.tgz")
	// Purge macOS AppleDouble junk that pre-fix restores may have uploaded,
	// and exclude it from the archive: extracting `._*` entries on macOS
	// makes bsdtar attempt metadata restores that fail on existing files.
	out, err := e.runner.output(ctx, fmt.Sprintf(
		"if [ -d /home/beams/.claude/projects ]; then find /home/beams/.claude/projects \\( -name '._*' -o -name '.DS_Store' \\) -delete 2>/dev/null; tar --exclude='._*' --exclude='.DS_Store' -czf %s -C /home/beams/.claude projects && echo ok; else echo none; fi",
		remoteArchive))
	if err != nil {
		return trace.Wrap(err)
	}
	if strings.TrimSpace(lastLine(out)) != "ok" {
		return nil
	}
	archive := filepath.Join(e.stateDir, "claude-mirror.tgz")
	if err := e.runner.download(ctx, remoteArchive, archive); err != nil {
		return trace.Wrap(err)
	}
	_, _ = e.runner.output(ctx, "rm -f "+beamsDevShellQuote(remoteArchive))
	mirror := filepath.Join(e.stateDir, "claude")
	if err := os.MkdirAll(mirror, 0o700); err != nil {
		return trace.Wrap(err)
	}
	_, err = runLocal(ctx, mirror, "tar", "-xzf", archive, "-C", mirror)
	return trace.Wrap(err)
}

// ---------------------------------------------------------------------------
// Helpers

func (e *devSyncEngine) statLocal(path string) (localStamp, bool) {
	info, err := os.Lstat(filepath.Join(e.ws.LocalDir, path))
	if err != nil || info.IsDir() {
		return localStamp{}, false
	}
	return localStamp{mtime: info.ModTime().UnixNano(), size: info.Size()}, true
}

func (e *devSyncEngine) localSha(path string, stamp localStamp) (string, error) {
	if cached, ok := e.localShas[path]; ok && cached.stamp == stamp {
		return cached.sha, nil
	}
	f, err := os.Open(filepath.Join(e.ws.LocalDir, path))
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", trace.Wrap(err)
	}
	sha := hex.EncodeToString(h.Sum(nil))
	e.localShas[path] = stampedSha{stamp: stamp, sha: sha}
	return sha, nil
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
