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
	"github.com/gravitational/teleport/lib/migrate/hoststep"
)

// migrateFlags holds every `teleport migrate` flag. The migration itself
// (clusters, state directory, mappings) is described only by --config.
type migrateFlags struct {
	configPath, format, subset, hostStep, resource string
	check, confirm                                 bool
}

// withMigrator loads the --config file, opens the migrator and runs fn with
// it, then closes it and warns on stderr if that fails. fn's context is
// canceled by an interrupt, which lets a run stop with its state file intact.
func withMigrator(ctx context.Context, f *migrateFlags, fn func(context.Context, *migrate.Migrator) error) error {
	// Before dialing either cluster, so a typo fails the same way every time.
	if f.resource != "" {
		if err := migrate.CheckResource(f.resource); err != nil {
			return trace.Wrap(err)
		}
	}
	if f.configPath == "" {
		return trace.BadParameter("--config is required")
	}
	cfg, err := migrate.LoadConfig(f.configPath)
	if err != nil {
		return trace.Wrap(err)
	}
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
	return trace.Wrap(fn(ctx, m))
}

// onMigratePlan runs `teleport migrate plan`. A host that cannot migrate is
// reported in the output, not through the exit code.
func onMigratePlan(ctx context.Context, f *migrateFlags) error {
	return withMigrator(ctx, f, func(ctx context.Context, m *migrate.Migrator) error {
		summary, err := m.Plan(ctx, migrate.PlanOptions{Check: f.check, Resource: f.resource})
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(m.Render(os.Stdout, f.format, f.subset, summary))
	})
}

// onMigrateApply runs `teleport migrate apply`. Hosts that failed are listed
// in the report and make the command exit non-zero; plan, by contrast, exits 0.
func onMigrateApply(ctx context.Context, f *migrateFlags) error {
	return withMigrator(ctx, f, func(ctx context.Context, m *migrate.Migrator) error {
		return reportOutcome(m.Apply(ctx, migrate.ApplyOptions{Resource: f.resource}))
	})
}

// onMigrateRollback runs `teleport migrate rollback`. Hosts that failed are
// listed in the report and make the command exit non-zero.
func onMigrateRollback(ctx context.Context, f *migrateFlags) error {
	return withMigrator(ctx, f, func(ctx context.Context, m *migrate.Migrator) error {
		return reportOutcome(m.Rollback(ctx, migrate.RollbackOptions{Resource: f.resource}))
	})
}

// onMigrateRetire runs `teleport migrate retire`. It refuses without
// --confirm before it dials or locks anything. Hosts that failed are listed in
// the report and make the command exit non-zero.
func onMigrateRetire(ctx context.Context, f *migrateFlags) error {
	if !f.confirm {
		return trace.BadParameter("retire removes the source agent from the host and the source cluster; pass --confirm")
	}
	return withMigrator(ctx, f, func(ctx context.Context, m *migrate.Migrator) error {
		return reportOutcome(m.Retire(ctx, migrate.RetireOptions{Resource: f.resource}))
	})
}

// onMigratePromote runs `teleport migrate promote`. Hosts that failed are
// listed in the report and make the command exit non-zero.
func onMigratePromote(ctx context.Context, f *migrateFlags) error {
	return withMigrator(ctx, f, func(ctx context.Context, m *migrate.Migrator) error {
		return reportOutcome(m.Promote(ctx, migrate.PromoteOptions{Resource: f.resource}))
	})
}

// reportOutcome prints a non-nil report, including the partial one that comes
// back with an error, and returns err, or an error counting the failed hosts
// when err is nil and the report lists any.
func reportOutcome(report *migrate.Report, err error) error {
	if report != nil {
		migrate.RenderReport(os.Stdout, report)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if report != nil && len(report.Failures) > 0 {
		return trace.Errorf("hosts failed: %d; see the report above", len(report.Failures))
	}
	return nil
}

// onMigrateHost runs `teleport migrate host <step>`: one migration step on
// this host, reading its request on stdin and writing its result to stdout.
func onMigrateHost(ctx context.Context, f *migrateFlags) error {
	return trace.Wrap(hoststep.RunHostStep(ctx, f.hostStep, os.Stdin, os.Stdout))
}
