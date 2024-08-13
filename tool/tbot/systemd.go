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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type onInstallSystemdCmdFunc func(
	ctx context.Context,
	log *slog.Logger,
	configPath string,
	getExecutablePath func() (string, error),
	stdout io.Writer) error

func setupInstallSystemdCmd(rootCmd *kingpin.Application) (
	string,
	onInstallSystemdCmdFunc,
) {
	installCmd := rootCmd.Command("install", "Helper commands for installing Machine ID.")
	installSystemdCmd := installCmd.Command("systemd", "Generates and installs a systemd unit file for a specified tbot configuration file.")
	unitName := installSystemdCmd.Flag("name", "Name for the systemd unit. Defaults to 'tbot'.").Default("tbot").String()
	group := installSystemdCmd.Flag("group", "The group that the service should run as. Defaults to 'teleport'.").Default("teleport").String()
	user := installSystemdCmd.Flag("user", "The user that the service should run as. Defaults to 'teleport'.").Default("teleport").String()
	force := installSystemdCmd.Flag("force", "Overwrite existing systemd unit file if present.").Bool()
	write := installSystemdCmd.Flag("write", "Write the systemd unit file. If not specified, this command runs in a dry-run mode that outputs the generated content to stdout.").Bool()
	systemdDirectory := installSystemdCmd.Flag("systemd-directory", "Path to the directory that the systemd unit file should be written. Defaults to '/etc/systemd/system'.").Default("/etc/systemd/system").String()
	anonymousTelemetry := installSystemdCmd.Flag("anonymous-telemetry", "Enable anonymous telemetry.").Bool()

	f := onInstallSystemdCmdFunc(func(
		ctx context.Context,
		log *slog.Logger,
		configPath string,
		getExecutablePath func() (string, error),
		stdout io.Writer,
	) error {
		return onInstallSystemdCmd(
			ctx,
			log,
			*unitName,
			*force,
			*write,
			*systemdDirectory,
			*user,
			*group,
			*anonymousTelemetry,
			configPath,
			getExecutablePath,
			stdout,
		)
	})

	return installSystemdCmd.FullCommand(), f
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
	TBotPath           string
}

func onInstallSystemdCmd(
	ctx context.Context,
	log *slog.Logger,
	unitName string,
	force bool,
	write bool,
	systemdDirectory string,
	user string,
	group string,
	anonymousTelemetry bool,
	configPath string,
	getExecutablePath func() (string, error),
	stdout io.Writer,
) error {
	switch {
	case configPath == "":
		return trace.BadParameter("missing required parameter --config")
	case unitName == "":
		return trace.BadParameter("missing required parameter --name")
	}

	tbotPath, err := getExecutablePath()
	if err != nil {
		return trace.Wrap(err, "determining path to current executable")
	}

	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return trace.Wrap(err, "determining absolute path to config")
	}

	buf := bytes.NewBuffer(nil)
	err = systemdTemplate.Execute(buf, systemdTemplateParams{
		UnitName:           unitName,
		User:               user,
		Group:              group,
		AnonymousTelemetry: anonymousTelemetry,
		ConfigPath:         configPath,
		TBotPath:           tbotPath,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	generated := buf.Bytes()
	path := filepath.Join(systemdDirectory, fmt.Sprintf("%s.service", unitName))

	if !write {
		_, _ = fmt.Fprintf(
			stdout,
			"Dry-run mode is active. Use '--write' to enable writes.\nThe following would have been written to '%s':\n\n%s",
			path,
			string(generated),
		)
		return nil
	}

	// Before writing, check if it exists, and if it does, check if it matches
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

	msg := fmt.Sprintf(
		"Wrote systemd unit file. Reload systemd with 'systemctl daemon-reload' and then enable and start the service with 'systemctl enable --now %s'",
		unitName,
	)
	//nolint:sloglint // This is intended to be a human-readable message which will be clearer with string formatting.
	log.InfoContext(
		ctx,
		msg,
		"path", path,
	)
	return nil
}
