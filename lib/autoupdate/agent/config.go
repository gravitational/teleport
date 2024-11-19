/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

const (
	updateServiceTemplate = `# teleport-update
[Unit]
Description=Teleport auto-update service

[Service]
Type=oneshot
ExecStart={{.LinkDir}}/bin/teleport-update update
`
	updateTimerTemplate = `# teleport-update
[Unit]
Description=Teleport auto-update timer unit

[Timer]
OnActiveSec=1m
OnUnitActiveSec=5m
RandomizedDelaySec=1m

[Install]
WantedBy=teleport.service
`
)

func Setup(ctx context.Context, log *slog.Logger, linkDir, dataDir string) error {
	err := writeConfigFiles(linkDir, dataDir)
	if err != nil {
		return trace.Errorf("failed to write teleport-update systemd config files: %w", err)
	}
	svc := &SystemdService{
		ServiceName: "teleport-update.timer",
		Log:         log,
	}
	err = svc.Sync(ctx)
	if errors.Is(err, ErrNotSupported) {
		log.WarnContext(ctx, "Not enabling systemd service because systemd is not running.")
		return nil
	}
	if err != nil {
		return trace.Errorf("failed to sync systemd config: %w", err)
	}
	if err := svc.Enable(ctx, true); err != nil {
		return trace.Errorf("failed to enable teleport-update systemd timer: %w", err)
	}
	return nil
}

func writeConfigFiles(linkDir, dataDir string) error {
	// TODO(sclevine): revert on failure

	servicePath := filepath.Join(linkDir, serviceDir, updateServiceName)
	err := writeTemplate(servicePath, updateServiceTemplate, linkDir, dataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	timerPath := filepath.Join(linkDir, serviceDir, updateTimerName)
	err = writeTemplate(timerPath, updateTimerTemplate, linkDir, dataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func writeTemplate(path, t, linkDir, dataDir string) error {
	if err := os.MkdirAll(filepath.Dir(path), systemDirMode); err != nil {
		return trace.Wrap(err)
	}
	opts := []renameio.Option{
		renameio.WithPermissions(configFileMode),
		renameio.WithExistingPermissions(),
	}
	f, err := renameio.NewPendingFile(path, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Cleanup()

	tmpl, err := template.New(filepath.Base(path)).Parse(t)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tmpl.Execute(f, struct {
		LinkDir string
		DataDir string
	}{linkDir, dataDir})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(f.CloseAtomicallyReplace())
}
