// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/renameio/v2/maybe"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Host states.
const (
	stateUnstarted = "unstarted"
	stateBlocked   = "blocked"
	stateMigrated  = "migrated"
)

// states returns every host state in display order. It is a function, not a
// package-level slice, so no caller can reorder or extend the set.
func states() []string {
	return []string{stateUnstarted, stateBlocked, stateMigrated}
}

// host is one source host the migration tracks.
type host struct {
	// ID is the source host's UUID.
	ID string `json:"id"`
	// Hostname is what plan prints.
	Hostname string `json:"hostname"`
	// Labels are the source node's own labels, which chose the mapping.
	Labels map[string]string `json:"labels"`
	// TargetScope is the scope the mapping sends the host to.
	TargetScope string `json:"target_scope"`
	// State is one of states().
	State string `json:"state"`
	// Reason says why the host sits in State, for the states that need one.
	Reason string `json:"reason,omitempty"`
	// TargetID is the host UUID of the scoped agent on the target that this
	// host is paired with.
	TargetID string `json:"target_id,omitempty"`
	// MatchedBy names the evidence that pairs the host with TargetID.
	MatchedBy string `json:"matched_by,omitempty"`
	// UpdatedAt is when State, Reason or the pairing last changed.
	UpdatedAt time.Time `json:"updated_at"`
}

// How a host was paired with its target agent; see host.MatchedBy.
const (
	// matchedByLabel pairs a host by the migration labels on the target agent.
	matchedByLabel = "label"
)

// Correlation labels: the labels a migrated scoped agent carries, so plan can
// pair a target node with its source host.
const (
	// sourceClusterLabel names the source cluster the agent migrated from.
	sourceClusterLabel = types.TeleportInternalLabelPrefix + "migrate-source-cluster"
	// sourceHostLabel holds the UUID of the source host the agent migrated from.
	sourceHostLabel = types.TeleportInternalLabelPrefix + "migrate-source-host-uuid"
)

// targetAgent identifies a target-side agent.
type targetAgent struct {
	// ID is the agent's host UUID.
	ID string `json:"id"`
	// Hostname is the agent's hostname.
	Hostname string `json:"hostname"`
	// Scope is the agent's scope.
	Scope string `json:"scope"`
}

// migrationState is the state file's content.
type migrationState struct {
	// Version is the file's schema version; see stateVersion.
	Version int `json:"version"`
	// Source is the source proxy's address as configured, a check that the
	// file describes the migration the config asks for.
	Source string `json:"source"`
	// Target is the target proxy's address as configured, for the same check.
	Target string `json:"target"`
	// Migrations is the mapping set the file was written for, sorted.
	Migrations []HostMapping `json:"migrations"`
	// Hosts is keyed by host.ID.
	Hosts map[string]*host `json:"hosts"`
	// UnmatchedTargets are agents inside a migration scope that no host
	// claims.
	UnmatchedTargets []targetAgent `json:"unmatched_targets"`
	// UpdatedAt is when the file was last saved.
	UpdatedAt time.Time `json:"updated_at"`
}

// stateVersion is the state file's schema version. Any other readable version
// is refused, not rewritten or moved aside: the file is the only record of the
// migration's progress.
const stateVersion = 1

// newState returns an empty state for cfg.
func newState(cfg *Config) *migrationState {
	return &migrationState{
		Version:    stateVersion,
		Source:     cfg.Source.ProxyServer,
		Target:     cfg.Target.ProxyServer,
		Migrations: sortedMappings(cfg.Migrations),
		Hosts:      make(map[string]*host),
	}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// maxSlugLen caps each proxy's part of the state file name, so two 253-byte
// hostnames still leave the name and its .lock sibling under the 255-byte
// file name limit.
const maxSlugLen = 64

// proxySlug returns the host part of a proxy address, without its port,
// lower-cased, with every run of characters outside [a-z0-9] replaced by one
// dash, cut to maxSlugLen bytes and trimmed of dashes at both ends.
func proxySlug(proxy string) string {
	s := nonAlnum.ReplaceAllString(strings.ToLower(utils.TryHost(proxy)), "-")
	return strings.Trim(s[:min(len(s), maxSlugLen)], "-")
}

// sortedMappings returns a sorted copy of in.
func sortedMappings(in []HostMapping) []HostMapping {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b HostMapping) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// stateFileName is <source>-<target>-<hash>.json, where the hash covers the
// proxy addresses as configured, ports and all, and the sorted mapping set.
// Any change to either starts a new file. The slugs in the prefix are lossy,
// they map source.example.com and source-example.com alike and cut long
// hostnames, so it is the hash, not the prefix, that keeps two migrations'
// files apart.
func stateFileName(cfg *Config) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\n%s\n", cfg.Source.ProxyServer, cfg.Target.ProxyServer)
	for _, mapping := range sortedMappings(cfg.Migrations) {
		fmt.Fprintf(hash, "%s=%s\n", mapping.SourceLabels, mapping.TargetScope)
	}
	return fmt.Sprintf("%s-%s-%s.json", proxySlug(cfg.Source.ProxyServer), proxySlug(cfg.Target.ProxyServer), hex.EncodeToString(hash.Sum(nil))[:8])
}

// stateFilePath is the file a migration's hosts live in.
func stateFilePath(cfg *Config) string {
	return filepath.Join(cfg.StateDir, stateFileName(cfg))
}

// stateFile is a migration's state file, held under an exclusive lock for
// the process lifetime. It is not safe for concurrent use.
type stateFile struct {
	path   string
	unlock func() error
	// state is the file's contents; save writes it back.
	state *migrationState
}

// openStateFile locks and loads the migration's state file, creating the
// directory and a fresh state when absent. A file this build cannot trust is
// refused and left alone; discarding it is the operator's call.
func openStateFile(cfg *Config) (*stateFile, error) {
	if err := os.MkdirAll(cfg.StateDir, teleport.PrivateDirMode); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "creating %q", cfg.StateDir)
	}
	path := stateFilePath(cfg)
	lockPath := path + ".lock"
	unlock, err := utils.FSTryWriteLock(lockPath)
	if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
		// The flock dies with its holder, so a lock that is held always has a
		// live process behind it; deleting the lockfile would only let two
		// commands write the state file at once.
		return nil, trace.Wrap(err, "state file %q is locked by %q; is another migrate command running? Wait for it to finish or stop it (lsof %q shows which process holds it); the lock is released when that process exits", path, lockPath, lockPath)
	}
	if err != nil {
		return nil, trace.Wrap(err, "locking %q", lockPath)
	}
	file := &stateFile{
		path:   path,
		unlock: unlock,
	}
	if err := file.load(cfg); err != nil {
		return nil, trace.NewAggregate(err, unlock())
	}
	return file, nil
}

// load reads the state file. A missing file is a fresh state. A file this
// build cannot trust is refused and left for the operator to move aside: an
// unknown schema version, no version, a parse error or a field it does not
// know, another migration's source, target or mappings, no hosts, a nil
// host or one keyed by an ID other than its own, or a host state it does not
// know.
func (f *stateFile) load(cfg *Config) error {
	fresh := newState(cfg)
	input, err := os.Open(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		f.state = fresh
		return nil
	}
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "opening %q", f.path)
	}
	loaded := &migrationState{}
	decoder := json.NewDecoder(input)
	// A field this build does not know came from a newer one, and a save
	// would drop it.
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(loaded)
	if decodeErr == nil {
		// Anything after the object is damage too, and a save would drop it.
		if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
			decodeErr = trace.BadParameter("unexpected data after the state object")
		}
	}
	_ = input.Close()
	// Checked even when the decode failed: encoding/json fills what it can
	// past a type mismatch, so a file a newer build retyped still reports its
	// version, while a file that is not JSON reports 0 and takes the corrupt
	// path below.
	//
	// Version 0 means no version, since every save stamps one: a hand-edited
	// or foreign file, not an older migrate. Both errors offer to move the
	// file aside: discarding it is the operator's call.
	switch {
	case loaded.Version != 0 && loaded.Version != stateVersion:
		return trace.BadParameter("%q has schema version %d; this build reads %d and will not rewrite a file it cannot fully read, so run the version of teleport migrate that wrote it, or move the file aside to start over", f.path, loaded.Version, stateVersion)
	case loaded.Version == 0 && decodeErr == nil:
		return trace.BadParameter("%q has no schema version; every migrate stamps one, so check whether the file was edited or belongs to something else, or move the file aside to start over", f.path)
	}
	if decodeErr != nil {
		return trace.BadParameter("%q does not parse: %v; check whether the file was edited, truncated or written by a newer version of teleport migrate, or move the file aside to start over", f.path, decodeErr)
	}
	if err := checkLoadedState(loaded, fresh); err != nil {
		return trace.Wrap(err, "loading %q", f.path)
	}
	if loaded.Hosts == nil {
		return trace.BadParameter("%q has no hosts; every migrate writes them, so check whether the file was edited or truncated, or move the file aside to start over", f.path)
	}
	for id, h := range loaded.Hosts {
		switch {
		case h == nil || h.ID != id:
			return trace.BadParameter("%q host %q is empty or carries another ID; check whether the file was edited, or move the file aside to start over", f.path, id)
		// stateVersion pins the file's fields, not their values, so a new
		// state slips past it. This is the guard for that.
		case !slices.Contains(states(), h.State):
			return trace.BadParameter("%q host %q has state %q, which this build does not know; a newer version of teleport migrate may have written it, so upgrade the tool, or move the file aside to start over", f.path, id, h.State)
		}
	}
	f.state = loaded
	return nil
}

// checkLoadedState reports whether a decoded file describes the migration
// expected asks for. Reaching a mismatch means two migrations landed on one
// path, so it refuses rather than resets: the file is the only record of the
// migration's progress. Deliberately narrow for the same reason:
// over-validating would refuse a file worth keeping.
func checkLoadedState(loaded, expected *migrationState) error {
	// A refusal blocks every command, so it has to say how to get unstuck.
	const escape = "check whether it was copied from another migration, or move the file aside to start over"
	if loaded.Source != expected.Source || loaded.Target != expected.Target {
		return trace.BadParameter("the state file migrates %q to %q, not %q to %q; %s", loaded.Source, loaded.Target, expected.Source, expected.Target, escape)
	}
	if len(loaded.Migrations) != len(expected.Migrations) {
		return trace.BadParameter("the state file holds a different mapping set; %s", escape)
	}
	for i, mapping := range sortedMappings(loaded.Migrations) {
		if mapping.String() != expected.Migrations[i].String() {
			return trace.BadParameter("the state file holds a different mapping set; %s", escape)
		}
	}
	return nil
}

// save stamps the state's UpdatedAt and writes the file atomically.
func (f *stateFile) save() error {
	f.state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	if err := maybe.WriteFile(f.path, data, teleport.FileMaskOwnerOnly); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "writing %q", f.path)
	}
	return nil
}

// close saves and releases the lock.
func (f *stateFile) close() error {
	return trace.NewAggregate(f.save(), f.unlock())
}
