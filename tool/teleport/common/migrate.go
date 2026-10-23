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

package common

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/migrate"
)

// migrateFlags holds every `teleport migrate` flag. The migration itself
// (clusters, state directory, mappings) is described only by --config.
type migrateFlags struct {
	configPath, format, subset string
}

// onMigratePlan runs `teleport migrate plan`. A host that cannot migrate is
// reported in the output, not through the exit code.
func onMigratePlan(ctx context.Context, f *migrateFlags) error {
	cfg, err := migrate.LoadConfig(f.configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	// Canceling lets an interrupted run stop with its state file intact.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	m, err := migrate.NewMigrator(ctx, cfg, migrate.Options{Progress: os.Stderr})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}()
	summary, err := m.Plan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(m.Render(os.Stdout, f.format, f.subset, summary))
}
