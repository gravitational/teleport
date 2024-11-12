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

package tshwrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// TSHVarName is the name of the environment variable that can override the
	// tsh path that would otherwise be located on the $PATH.
	TSHVarName = "TSH"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

// capture runs a command (presumably tsh) with the given arguments and
// returns it's captured stdout. Stderr is ignored. Errors are returned per
// exec.Command().Output() semantics.
func capture(tshPath string, args ...string) ([]byte, error) {
	out, err := exec.Command(tshPath, args...).Output()
	if err != nil {
		return nil, trace.Wrap(err, "error executing tsh")
	}

	return out, nil
}

// Wrapper is a wrapper to execute `tsh` commands via a subprocess.
type Wrapper struct {
	// path is a path to the tsh executable
	path string

	// capture is the function for capturing a command's output. It may be
	// overridden by tests for mocking purposes, but by default is expected to
	// execute an actual tsh binary on the host system.
	capture func(tshPath string, args ...string) ([]byte, error)
}

// New creates a new tsh wrapper. If a $TSH var is set it uses that path,
// otherwise looks for tsh on the OS path.
func New() (*Wrapper, error) {
	if val, ok := os.LookupEnv(TSHVarName); ok {
		return &Wrapper{
			path:    val,
			capture: capture,
		}, nil
	}

	binary := "tsh"
	if runtime.GOOS == constants.WindowsOS {
		binary = "tsh.exe"
	}

	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Wrapper{
		path:    path,
		capture: capture,
	}, nil
}

// Exec runs tsh with the given environment variables and arguments. The child
// process inherits stdin/stdout/stderr and runs until completion. Errors are
// returned per `exec.Command().Run()` semantics.
func (w *Wrapper) Exec(env map[string]string, args ...string) error {
	// The subprocess should inherit the environment plus our vars. Our env
	// vars will safely overwrite those from the environment, per `exec.Cmd`
	// docs.
	environ := os.Environ()
	for k, v := range env {
		// In case of similar keys, last env var wins.
		environ = append(environ, k+"="+v)
	}

	log.DebugContext(
		context.TODO(),
		"executing binary",
		"path", w.path,
		"env", env,
		"args", args,
	)

	child := exec.Command(w.path, args...)
	child.Env = environ
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	return trace.Wrap(child.Run(), "unable to execute tsh")
}

type destinationHolder interface {
	GetDestination() bot.Destination
}

// GetDestinationDirectory attempts to select an unambiguous destination, either
// from CLI or YAML config. It returns an error if the selected destination is
// invalid. Note that CLI destinations will not be validated.
func GetDestinationDirectory(cliDestinationPath string, botConfig *config.BotConfig) (*config.DestinationDirectory, error) {
	if cliDestinationPath != "" {
		d := &config.DestinationDirectory{
			Path: cliDestinationPath,
		}
		if err := d.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return d, nil
	}

	var destinationHolders []destinationHolder
	for _, svc := range botConfig.Services {
		if v, ok := svc.(destinationHolder); ok {
			destinationHolders = append(destinationHolders, v)
		}
	}

	if len(destinationHolders) == 0 {
		return nil, trace.BadParameter("either --destination-dir or a config file containing an output or service must be specified")
	} else if len(destinationHolders) > 1 {
		return nil, trace.BadParameter("the config file contains multiple outputs and services; a --destination-dir must be specified")
	}
	destination := destinationHolders[0].GetDestination()
	destinationDir, ok := destination.(*config.DestinationDirectory)
	if !ok {
		return nil, trace.BadParameter("destination %s must be a directory", destination)
	}

	return destinationDir, nil
}

// mergeEnv applies the given value to each key inside the specified map.
func mergeEnv(m map[string]string, value string, keys []string) {
	for _, key := range keys {
		m[key] = value
	}
}

// GetEnvForTSH returns a map of environment variables needed to properly wrap
// tsh so that it uses our Machine ID certificates where necessary.
func GetEnvForTSH(destPath string) (map[string]string, error) {
	// The env var interface does allow us to set specific resource names for
	// everything but also has generic fallbacks. We'll use the fallbacks for
	// now but could eventually communicate more info to tsh if desired.
	env := make(map[string]string)
	mergeEnv(env, filepath.Join(destPath, identity.PrivateKeyKey), client.VirtualPathEnvNames(client.VirtualPathKey, nil))

	// Database certs are a bit awkward since a few databases (cockroach) have
	// special naming requirements. We can document around these for now and
	// automate later. (I don't think tsh handles this perfectly today anyway).
	mergeEnv(env, filepath.Join(destPath, identity.TLSCertKey), client.VirtualPathEnvNames(client.VirtualPathDatabase, nil))

	mergeEnv(env, filepath.Join(destPath, identity.TLSCertKey), client.VirtualPathEnvNames(client.VirtualPathAppCert, nil))

	// We don't want to provide a fallback for CAs since it would be ambiguous,
	// so we'll specify them exactly.
	env[client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.UserCA))] = filepath.Join(destPath, config.UserCAPath)
	env[client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.HostCA))] = filepath.Join(destPath, config.HostCAPath)
	env[client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.DatabaseCA))] = filepath.Join(destPath, config.DatabaseCAPath)

	return env, nil
}

// LoadIdentity loads a Teleport identity from an identityfile. Secondary bot
// identities are not loadable, so we'll just read the Teleport identity (which
// is required for tsh to function anyway).
func LoadIdentity(identityPath string) (*tlsca.Identity, error) {
	f, err := os.Open(identityPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	idFile, err := identityfile.Read(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tlsca.ParseCertificatePEM(idFile.Certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsed, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	return parsed, trace.Wrap(err)
}
