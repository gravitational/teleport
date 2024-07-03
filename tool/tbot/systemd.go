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

package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
)

func setupInstallSystemdCmd(
	rootCmd *kingpin.Application,
) (string, func(ctx context.Context, log *slog.Logger, cf config.CLIConf) error) {
	installCmd := rootCmd.Command("install", "Helper commands for installing Machine ID")
	installSystemdCmd := installCmd.Command("systemd", "Install systemd unit file")
	unitName := installSystemdCmd.Flag("name", "Name for the systemd unit").Default("tbot").String()
	group := installSystemdCmd.Flag("group", "The group that the service should run as").Default("teleport").String()
	user := installSystemdCmd.Flag("user", "The user that the service should run as").Default("teleport").String()
	force := installSystemdCmd.Flag("force", "Overwrite existing systemd unit file if present").Bool()
	systemdDirectory := installSystemdCmd.Flag("systemd-directory", "Directory to install systemd unit file").Default("/etc/systemd/system").String()
	anonymousTelemetry := installSystemdCmd.Flag("anonymous-telemetry", "Enable anonymous telemetry").Bool()

	return installSystemdCmd.FullCommand(), func(ctx context.Context, log *slog.Logger, cf config.CLIConf) error {
		return onInstallSystemdCmd(
			ctx,
			log,
			cf,
			*unitName,
			*force,
			*systemdDirectory,
			*user,
			*group,
			*anonymousTelemetry,
		)
	}
}

var (
	//go:embed systemd.tmpl
	systemdTemplateData string
	systemdTemplate     = template.Must(template.New("").Parse(systemdTemplateData))
)

type systemdTemplateParams struct {
	UnitName           string
	User               string
	Group              string
	AnonymousTelemetry bool
	ConfigPath         string
}

func onInstallSystemdCmd(
	ctx context.Context,
	log *slog.Logger,
	cf config.CLIConf,
	unitName string,
	force bool,
	systemdDirectory string,
	user string,
	group string,
	anonymousTelemetry bool,
) error {
	switch {
	case cf.ConfigPath == "":
		return trace.BadParameter("missing required parameter --config")
	case unitName == "":
		return trace.BadParameter("missing required parameter --name")
	}

	buf := bytes.NewBuffer(nil)
	err := systemdTemplate.Execute(buf, systemdTemplateParams{
		UnitName:           unitName,
		User:               user,
		Group:              group,
		AnonymousTelemetry: anonymousTelemetry,
		ConfigPath:         cf.ConfigPath,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	generated := buf.Bytes()

	// Before writing, check if it exists, and if it does, check if it matches
	path := filepath.Join(systemdDirectory, fmt.Sprintf("%s.service", unitName))
	existingData, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}
	if len(existingData) > 0 {
		log.InfoContext(ctx, "Existing systemd unit file found", "path", path)
		if bytes.Equal(existingData, generated) {
			log.InfoContext(ctx, "No changes to write", "path", path)
			return nil
		}
		log.InfoContext(ctx, "Generated unit is different to existing systemd unit file", "path", path)
		if !force {
			log.ErrorContext(ctx, "An existing systemd unit file was found and its content differs from the generated content. No changes will be made. Use --force to overwrite", "path", path)
			return trace.BadParameter("systemd unit file %s already exists with different content", path)
		}
		log.InfoContext(ctx, "--force has been specified, existing systemd unit file will be overwritten", "path", path)
	}

	if err := os.WriteFile(path, generated, 0644); err != nil {
		return trace.Wrap(err, "writing unit file")
	}

	log.InfoContext(
		ctx,
		fmt.Sprintf("Wrote systemd unit file. Reload systemd with 'systemctl daemon-reload' and then enable and start the service with 'systemctl enable --now %s'", unitName),
		"path", path,
	)
	return nil
}
