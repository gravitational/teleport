/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package tctl

import (
	"context"
	"os/exec"
	"regexp"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/logger"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var regexpStatusCAPin = regexp.MustCompile(`CA pin +(sha256:[a-zA-Z0-9]+)`)

// Tctl is a runner of tctl command.
type Tctl struct {
	Path       string
	ConfigPath string
	AuthServer string
}

// CheckExecutable checks if `tctl` executable exists in the system.
func (tctl Tctl) CheckExecutable() error {
	_, err := exec.LookPath(tctl.cmd())
	return trace.Wrap(err, "tctl executable is not found")
}

// Sign generates Teleport client credentials at a given path.
func (tctl Tctl) Sign(ctx context.Context, username, format, outPath string) error {
	log := logger.Get(ctx)
	args := append(tctl.baseArgs(),
		"auth",
		"sign",
		"--user",
		username,
		"--format",
		format,
		"--overwrite",
		"--out",
		outPath,
	)
	cmd := exec.CommandContext(ctx, tctl.cmd(), args...)
	log.DebugContext(ctx, "Running tctl auth sign", "command", logutils.StringerAttr(cmd))
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.DebugContext(ctx, "tctl auth sign failed",
			"error", err,
			"args", args,
			"command_output", string(output),
		)
		return trace.Wrap(err, "tctl auth sign failed")
	}
	return nil
}

// Create creates or updates a set of Teleport resources.
func (tctl Tctl) Create(ctx context.Context, resources []types.Resource) error {
	log := logger.Get(ctx)
	args := append(tctl.baseArgs(), "create")
	cmd := exec.CommandContext(ctx, tctl.cmd(), args...)
	log.DebugContext(ctx, "Running tctl create", "command", logutils.StringerAttr(cmd))
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return trace.Wrap(err, "failed to get stdin pipe")
	}
	go func() {
		defer func() {
			if err := stdinPipe.Close(); err != nil {
				log.ErrorContext(ctx, "Failed to close stdin pipe", "error", err)
			}
		}()
		if err := writeResourcesYAML(stdinPipe, resources); err != nil {
			log.ErrorContext(ctx, "Failed to serialize resources stdin", "error", err)
		}
	}()
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.DebugContext(ctx, "tctl create failed",
			"error", err,
			"command_output", string(output),
		)
		return trace.Wrap(err, "tctl create failed")
	}
	return nil
}

// GetAll loads a bunch of Teleport resources by a given query.
func (tctl Tctl) GetAll(ctx context.Context, query string) ([]types.Resource, error) {
	log := logger.Get(ctx)
	args := append(tctl.baseArgs(), "get", query)
	cmd := exec.CommandContext(ctx, tctl.cmd(), args...)

	log.DebugContext(ctx, "Running tctl get", "command", logutils.StringerAttr(cmd))
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get stdout")
	}
	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err, "failed to start tctl")
	}
	resources, err := readResourcesYAMLOrJSON(stdoutPipe)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resources, nil
}

// Get loads a singular resource by its kind and name identifiers.
func (tctl Tctl) Get(ctx context.Context, kind, name string) (types.Resource, error) {
	query := kind + "/" + name
	resources, err := tctl.GetAll(ctx, query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(resources) == 0 {
		return nil, trace.NotFound("resource %q is not found", query)
	}
	return resources[0], nil
}

// GetCAPin sets the auth service CA Pin using output from tctl.
func (tctl Tctl) GetCAPin(ctx context.Context) (string, error) {
	log := logger.Get(ctx)

	args := append(tctl.baseArgs(), "status")
	cmd := exec.CommandContext(ctx, tctl.cmd(), args...)

	log.DebugContext(ctx, "Running tctl status", "command", logutils.StringerAttr(cmd))
	output, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err, "failed to get auth status")
	}

	submatch := regexpStatusCAPin.FindStringSubmatch(string(output))
	if len(submatch) < 2 || submatch[1] == "" {
		return "", trace.Errorf("failed to find CA Pin in auth status")
	}
	return submatch[1], nil
}

func (tctl Tctl) cmd() string {
	if tctl.Path != "" {
		return tctl.Path
	}
	return "tctl"
}

func (tctl Tctl) baseArgs() (args []string) {
	if tctl.ConfigPath != "" {
		args = append(args, "--config", tctl.ConfigPath)
	}
	if tctl.AuthServer != "" {
		args = append(args, "--auth-server", tctl.AuthServer)
	}
	return
}
