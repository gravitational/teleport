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

package common

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/config"
)

// reconfigureFlags holds the command line flags for `teleport reconfigure`.
type reconfigureFlags struct {
	input                  string
	output                 string
	overwrite              bool
	proxy                  string
	authServer             string
	caPins                 []string
	token                  string
	joinMethod             string
	registrationSecret     string
	registrationSecretPath string
	nodeLabels             string
	dataDir                string
	pidFile                string
	diagAddr               string
	sshListenAddr          string
	kubeListenAddr         string
	metricsListenAddr      string
}

// onReconfigure is the handler for the "reconfigure" CLI command.
func onReconfigure(flags reconfigureFlags) error {
	return trace.Wrap(runReconfigure(flags, os.Stdout))
}

func runReconfigure(flags reconfigureFlags, stdout io.Writer) error {
	if flags.overwrite && flags.output == "" {
		return trace.BadParameter("--overwrite requires --output")
	}
	fc, err := config.ReadFromFile(flags.input)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := config.Reconfigure(fc, config.ReconfigureRequest{
		Proxy:                  flags.proxy,
		AuthServer:             flags.authServer,
		CAPins:                 flags.caPins,
		Token:                  flags.token,
		JoinMethod:             flags.joinMethod,
		RegistrationSecret:     flags.registrationSecret,
		RegistrationSecretPath: flags.registrationSecretPath,
		NodeLabels:             flags.nodeLabels,
		DataDir:                flags.dataDir,
		PIDFile:                flags.pidFile,
		DiagAddr:               flags.diagAddr,
		SSHListenAddr:          flags.sshListenAddr,
		KubeListenAddr:         flags.kubeListenAddr,
		MetricsListenAddr:      flags.metricsListenAddr,
	}); err != nil {
		return trace.Wrap(err)
	}
	out, err := fc.YAMLString()
	if err != nil {
		return trace.Wrap(err)
	}
	// Round-trip the output through the same reader the agent uses at
	// start, so an invalid file is never written.
	if _, err := config.ReadConfig(strings.NewReader(out)); err != nil {
		return trace.Wrap(err, "internal error: generated config failed validation, nothing written")
	}
	if flags.output == "" {
		if _, err := fmt.Fprint(stdout, out); err != nil {
			return trace.ConvertSystemError(err)
		}
		return nil
	}
	return trace.Wrap(writeReconfigureOutput(flags.input, flags.output, flags.overwrite, []byte(out)))
}

// writeReconfigureOutput writes data to output with owner-only permissions.
// It refuses to write over the input config and refuses an existing output
// unless overwrite is set.
func writeReconfigureOutput(input, output string, overwrite bool, data []byte) error {
	inInfo, err := os.Stat(input)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if outInfo, err := os.Stat(output); err == nil && os.SameFile(inInfo, outInfo) {
		return trace.BadParameter("refusing to write over the input config %q", input)
	}

	if overwrite {
		// Temp file + atomic rename, forcing owner-only permissions:
		// renameio.WriteFile would keep an existing file's broader mode.
		pending, err := renameio.NewPendingFile(output, renameio.WithPermissions(teleport.FileMaskOwnerOnly))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		defer pending.Cleanup()
		if _, err := pending.Write(data); err != nil {
			return trace.ConvertSystemError(err)
		}
		return trace.ConvertSystemError(pending.CloseAtomicallyReplace())
	}

	// O_EXCL atomically rejects anything already at the path, symlinks included.
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, teleport.FileMaskOwnerOnly)
	if err != nil {
		if os.IsExist(err) {
			return trace.AlreadyExists("output file %q already exists; use --overwrite to replace it", output)
		}
		return trace.ConvertSystemError(err)
	}
	// On failure, remove the partial file so a retry is not blocked by the
	// already-exists guard.
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(output)
		return trace.ConvertSystemError(err)
	}
	// Close can carry the real write failure.
	if err := f.Close(); err != nil {
		os.Remove(output)
		return trace.ConvertSystemError(err)
	}
	return nil
}
