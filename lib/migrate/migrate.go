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
	"slices"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Options tune a Migrator.
type Options struct {
	// Progress receives one line per notable event; nil discards them.
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
	target     clusterAPI
	closers    []io.Closer
}

// NewMigrator validates opts, opens the state file and dials the source and
// target clusters. Close releases them. cfg must come from LoadConfig.
func NewMigrator(ctx context.Context, cfg *Config, opts Options) (*Migrator, error) {
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	stateFile, err := openStateFile(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m := &Migrator{
		config:    cfg,
		progress:  opts.Progress,
		stateFile: stateFile,
		closers:   []io.Closer{utils.CloseFunc(stateFile.close)},
	}
	if err := m.dial(ctx); err != nil {
		return nil, trace.NewAggregate(err, m.Close())
	}
	return m, nil
}

// dial connects to the source and then the target cluster, and refuses a pair
// that are the same cluster.
func (m *Migrator) dial(ctx context.Context) error {
	source, err := dialCluster(ctx, m.config.Source)
	if err != nil {
		return trace.Wrap(err, "connecting to the source cluster")
	}
	m.source = source
	m.closers = append(m.closers, source)

	target, err := dialCluster(ctx, m.config.Target)
	if err != nil {
		return trace.Wrap(err, "connecting to the target cluster")
	}
	m.target = target
	m.closers = append(m.closers, target)

	return trace.Wrap(refuseSameCluster(m.source, m.target))
}

// refuseSameCluster refuses a source and target that are the same cluster:
// the source cluster's own nodes would be paired with themselves.
func refuseSameCluster(source, target clusterAPI) error {
	if source.ClusterName() == target.ClusterName() {
		return trace.BadParameter("source and target are both cluster %q; check the proxy_server addresses in the config", source.ClusterName())
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
