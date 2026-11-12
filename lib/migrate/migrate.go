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
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// concurrency bounds how many hosts run a step at once.
const concurrency = 32

// planHostTimeout bounds one host's checks while planning. The first preflight
// on a host downloads teleport, so it allows for a slow link.
const planHostTimeout = 5 * time.Minute

// targetWait bounds each wait for the scoped agent: for it to write its
// host_uuid, and for the target to list it.
const targetWait = 5 * time.Minute

// pollInterval is how often a wait on the target checks again.
const pollInterval = 5 * time.Second

// cleanupTimeout bounds the cluster cleanup a rollback or retire runs after
// its host step, detached from the host's timeout.
const cleanupTimeout = 30 * time.Second

// Options tune a Migrator.
type Options struct {
	// Optional: nil discards the progress lines. Otherwise, it receives one
	// line per notable event.
	Progress io.Writer
}

// CheckAndSetDefaults fills the unset options with their defaults.
func (o *Options) CheckAndSetDefaults() error {
	o.Progress = cmp.Or(o.Progress, io.Discard)
	return nil
}

// Migrator runs the migration commands against two clusters and the state
// file. It is not safe for concurrent use: one command runs at a time.
type Migrator struct {
	config   *Config
	progress io.Writer
	// progressMu serializes writes to progress.
	progressMu sync.Mutex
	stateFile  *stateFile
	source     clusterAPI
	target     targetClusterAPI
	// sourceHosts runs commands on the source cluster's hosts.
	sourceHosts hostRunner
	// targetHosts runs commands on the target cluster's hosts.
	targetHosts hostRunner
	closers     []io.Closer
	// binary is how hosts get this build of teleport.
	binary oneoff.OneOffScriptParams
	// cdnBaseURL is TELEPORT_CDN_BASE_URL as the operator set it, which the
	// scoped install downloads from; "" when unset.
	cdnBaseURL string
}

// NewMigrator validates opts, opens the state file and dials the source
// cluster's API and hosts, then the target cluster's API and hosts. Close
// releases them. cfg must come from LoadConfig. Hosts download teleport from
// TELEPORT_CDN_BASE_URL, or from Teleport's CDN when it is unset; the scoped
// install downloads from it too when it is set.
func NewMigrator(ctx context.Context, cfg *Config, opts Options) (*Migrator, error) {
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cdnBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	binary, err := newHostBinary(cmp.Or(cdnBaseURL, teleportassets.CDNBaseURL()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stateFile, err := openStateFile(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := &Migrator{
		config:     cfg,
		progress:   opts.Progress,
		stateFile:  stateFile,
		binary:     binary,
		cdnBaseURL: cdnBaseURL,
		closers:    []io.Closer{utils.CloseFunc(stateFile.close)},
	}
	if err := m.dial(ctx); err != nil {
		return nil, trace.NewAggregate(err, m.Close())
	}
	return m, nil
}

// dial connects to the source and then the target cluster and their hosts,
// and refuses a pair that are the same cluster.
func (m *Migrator) dial(ctx context.Context) error {
	source, err := dialCluster(ctx, m.config.Source)
	if err != nil {
		return trace.Wrap(err, "connecting to the source cluster")
	}
	m.source = source
	m.closers = append(m.closers, source)

	sourceHosts, err := dialHosts(ctx, m.config.Source)
	if err != nil {
		return trace.Wrap(err, "connecting to the source cluster's hosts")
	}
	m.sourceHosts, m.closers = sourceHosts, append(m.closers, sourceHosts)

	target, err := dialCluster(ctx, m.config.Target)
	if err != nil {
		return trace.Wrap(err, "connecting to the target cluster")
	}
	m.target = target
	m.closers = append(m.closers, target)

	if err := refuseSameCluster(m.source, m.target); err != nil {
		return trace.Wrap(err)
	}

	targetHosts, err := dialHosts(ctx, m.config.Target)
	if err != nil {
		return trace.Wrap(err, "connecting to the target cluster's hosts")
	}
	m.targetHosts, m.closers = targetHosts, append(m.closers, targetHosts)
	return nil
}

// refuseSameCluster refuses a source and target that are the same cluster:
// the source cluster's own nodes would be paired with themselves.
func refuseSameCluster(source, target clusterAPI) error {
	if source.ClusterName() == target.ClusterName() {
		return trace.BadParameter("source and target are both cluster %q; check the proxy_server addresses in the config", source.ClusterName())
	}
	return nil
}

// requireState refuses a state file no plan has written, for the commands
// that act on what plan recorded.
func (m *Migrator) requireState() error {
	if m.stateFile.fresh {
		return trace.NotFound("no state file at %q; run plan first", m.stateFile.path)
	}
	return nil
}

// Close closes the connections and then saves and closes the state file, in
// the reverse of the order NewMigrator opened them.
func (m *Migrator) Close() error {
	var errs []error
	for _, closer := range slices.Backward(m.closers) {
		errs = append(errs, closer.Close())
	}
	return trace.NewAggregate(errs...)
}

// progressf writes one line to the progress writer. It is safe for
// concurrent use.
func (m *Migrator) progressf(format string, args ...any) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	fmt.Fprintf(m.progress, format+"\n", args...)
}

// deleteNode deletes the node id in scope from c. A node already gone is not
// an error. Nor is a denied delete: the node then expires with its heartbeat,
// and a progress line says so.
//
// TODO(randalmodowski): once gravitational/core#1342 merges, Auth deletes the
// node when its agent stops gracefully. Then drop these deletes, or at least
// the note's claim that the node may still show up.
func (m *Migrator) deleteNode(ctx context.Context, c clusterAPI, scope, id string) error {
	err := c.DeleteNode(ctx, scope, id)
	switch {
	case err == nil, trace.IsNotFound(err):
		return nil
	case trace.IsAccessDenied(err):
		m.progressf("note: agent was successfully stopped, but this identity cannot immediately unregister node %s; it might still show up in Teleport and in plan for the next 15 minutes",
			utils.EscapeControl(scopes.QualifiedName{Scope: scope, Name: id}.String()))
		return nil
	}
	return trace.Wrap(err)
}

// Report is what a command that changes hosts did.
type Report struct {
	// Verb is what the command did to the hosts it counts, such as "migrated".
	Verb string
	// Done counts the hosts the command succeeded on.
	Done int
	// Failures are the hosts the command failed on, in the order they
	// reported.
	Failures []Failure
}

// Failure is one host a command failed on.
type Failure struct {
	// ID is the host's UUID.
	ID string
	// Hostname is the host's hostname.
	Hostname string
	// Reason is why the command failed, on one line.
	Reason string
}

// hostsInStates returns the hosts in states, in ID order, or only the one ref
// names, which must be in states. need describes the states for the error.
func (m *Migrator) hostsInStates(ref, need string, states ...string) ([]*host, error) {
	state := m.stateFile.state
	if ref != "" {
		name, err := parseNodeRef(ref)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h, err := state.findHost(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !slices.Contains(states, h.State) {
			return nil, trace.NotFound("%q is not %s; run plan and check its state", ref, need)
		}
		return []*host{h}, nil
	}
	var hosts []*host
	for _, id := range slices.Sorted(maps.Keys(state.Hosts)) {
		if h := state.Hosts[id]; slices.Contains(states, h.State) {
			hosts = append(hosts, h)
		}
	}
	return hosts, nil
}

// act runs fn on hosts, records each result on its host and in the report, and
// returns the report with any error.
func (m *Migrator) act(ctx context.Context, verb string, hosts []*host, timeout time.Duration, fn func(context.Context, *host) (*outcome, error)) (*Report, error) {
	report := &Report{Verb: verb}
	for result := range forEachHost(ctx, hosts, timeout, fn) {
		// After the caller's cancel only results that carry an outcome are
		// kept, such as a promote whose agent had already joined.
		if result.err != nil && ctx.Err() != nil && result.value == nil {
			continue
		}
		h := result.host
		if result.value != nil {
			h.setOutcome(*result.value, time.Now().UTC())
		}
		if result.err != nil {
			reason := joinLines(result.err.Error())
			report.Failures = append(report.Failures, Failure{ID: h.ID, Hostname: h.Hostname, Reason: reason})
			m.progressf("%s: failed: %s", utils.EscapeControl(h.Hostname), utils.EscapeControl(reason))
		} else {
			report.Done++
			m.progressf("%s: %s", utils.EscapeControl(h.Hostname), verb)
		}
	}
	if err := m.stateFile.save(); err != nil {
		return report, trace.Wrap(err)
	}
	return report, trace.Wrap(ctx.Err())
}

// hostResult is the outcome of fn for one host.
type hostResult[T any] struct {
	host  *host
	value T
	err   error
}

// forEachHost runs fn on every host, at most concurrency at a time, each
// under its own timeout. Every host that starts sends its result exactly once,
// and the channel is closed when all are done; the caller must drain it. A
// canceled ctx stops new hosts from starting, except one already waiting for
// a slot, which starts with the canceled ctx; the hosts already running still
// report.
func forEachHost[T any](ctx context.Context, hosts []*host, timeout time.Duration, fn func(context.Context, *host) (T, error)) <-chan hostResult[T] {
	results := make(chan hostResult[T])
	go dispatchHosts(ctx, hosts, timeout, fn, results)
	return results
}

// dispatchHosts runs fn on each host under timeout, at most concurrency at a
// time, and closes results after the last one reports.
func dispatchHosts[T any](ctx context.Context, hosts []*host, timeout time.Duration, fn func(context.Context, *host) (T, error), results chan<- hostResult[T]) {
	defer close(results)
	var group errgroup.Group
	group.SetLimit(concurrency)
	defer group.Wait()
	for _, h := range hosts {
		if ctx.Err() != nil {
			return
		}
		group.Go(func() error {
			hostCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			value, err := fn(hostCtx, h)
			results <- hostResult[T]{host: h, value: value, err: err}
			return nil
		})
	}
}
