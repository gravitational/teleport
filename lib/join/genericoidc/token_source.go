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

package genericoidc

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gravitational/trace"
)

type envGetter func(key string) string

type commandRunner func(ctx context.Context, command ...string) ([]byte, error)

// IDTokenSource allows a generic OIDC token to be fetched whilst within a job
// execution.
type IDTokenSource struct {
	getEnv     envGetter
	runCommand commandRunner
}

// GetIDTokenFromEnvironment fetches a JWT from the local node's environment
func (its *IDTokenSource) GetIDTokenFromEnvironment(key string) (string, error) {
	tok := its.getEnv(key)
	if tok == "" {
		return "", trace.BadParameter(
			"environment variable %q is missing, ensure it exists and contains a valid JWT for OIDC joining",
			key,
		)
	}

	return strings.TrimSpace(tok), nil
}

// GetIDTokenFromCommand executes a command to fetch a JWT. The command may take
// up to the given timeout to execute, and callers are recommended to provide a
// sensible default timeout (e.g. 1 minute) by default.
func (its *IDTokenSource) GetIDTokenFromCommand(ctx context.Context, timeout time.Duration, command ...string) (string, error) {
	if len(command) == 0 {
		return "", trace.BadParameter("at least one command argument is required")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	bytes, err := its.runCommand(timeoutCtx, command...)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Do some minimal validation. We'd otherwise like to check that this is a
	// parseable JWT, but doing so isn't worth including JWT libraries in client
	// binaries.
	if !utf8.Valid(bytes) {
		return "", trace.BadParameter("generic_oidc: retrieved token with invalid content")
	}

	return strings.TrimSpace(string(bytes)), nil
}

func DefaultCommandRunner(ctx context.Context, command ...string) ([]byte, error) {
	executable := command[0]
	args := command[1:]

	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, trace.Wrap(err, "determining home directory")
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir

	// Inherit stderr - if errors are printed, they should be visible to the
	// user. We could buffer them and wrap them in our own log, but that may
	// make debugging more difficult.
	cmd.Stderr = os.Stderr

	bytes, err := cmd.Output()
	if err != nil {
		return nil, trace.Wrap(err, "generic_oidc: failed to run command to fetch token, see previous output for details")
	}

	return bytes, nil
}

// NewIDTokenSource creates a new generic token source with the given audience
// tag.
func NewIDTokenSource(getEnv envGetter, runCommand commandRunner) *IDTokenSource {
	return &IDTokenSource{
		getEnv:     getEnv,
		runCommand: runCommand,
	}
}
