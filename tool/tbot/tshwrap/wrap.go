/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tshwrap

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	// TSHVarName is the name of the environment variable that can override the
	// tsh path that would otherwise be located on the $PATH.
	TSHVarName = "TSH"

	// TSHMinVersion is the minimum version of tsh that supports Machine ID
	// proxies.
	TSHMinVersion = "9.3.0"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

// TSHRunner is a method for executing tsh.
type TSHRunner interface {
	// Capture runs tsh and captures its standard output.
	Capture(args ...string) ([]byte, error)

	// exec runs tsh with the given additional environment variables and args.
	// It should inherit the current process's input and output streams.
	Exec(env map[string]string, args ...string) error
}

type execTSHRunner struct {
	path string
}

func (r *execTSHRunner) Capture(args ...string) ([]byte, error) {
	out, err := exec.Command(r.path, args...).Output()
	if err != nil {
		return nil, trace.Wrap(err, "error executing tsh")
	}

	return out, nil
}

func (r *execTSHRunner) Exec(env map[string]string, args ...string) error {
	// The subprocess should inherit the environment plus our vars.
	environ := os.Environ()
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}

	log.Debugf("executing %s with env=%+v and args=%+v", r.path, env, args)

	child := exec.Command(r.path, args...)
	child.Env = environ
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	return trace.Wrap(child.Run(), "unable to execute tsh")
}

// NewRunner attempts to locate a `tsh` binary on the local system. The
// standard path lookup behavior can be overridden by setting the `$TSH`
// environment variable to point to a different tsh executable.
func NewRunner() (TSHRunner, error) {
	if val, ok := os.LookupEnv(TSHVarName); ok {
		return &execTSHRunner{
			path: val,
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

	return &execTSHRunner{
		path: path,
	}, nil
}

// GetTSHVersion queries the system tsh for its version.
func GetTSHVersion(r TSHRunner) (*semver.Version, error) {
	rawVersion, err := r.Capture("version", "-f", "json")
	if err != nil {
		return nil, trace.Wrap(err, "error querying tsh version")
	}

	versionInfo := struct {
		Version string `json:"version"`
	}{}
	if err := json.Unmarshal(rawVersion, &versionInfo); err != nil {
		return nil, trace.Wrap(err, "error deserializing tsh version from string: %s", rawVersion)
	}

	sv, err := semver.NewVersion(versionInfo.Version)
	if err != nil {
		return nil, trace.Wrap(err, "error parsing tsh version: %s", versionInfo.Version)
	}

	return sv, nil
}

// CheckTSHSupported checks if the current tsh supports Machine ID.
func CheckTSHSupported(r TSHRunner) error {
	version, err := GetTSHVersion(r)
	if err != nil {
		return trace.Wrap(err, "unable to determine tsh version")
	}

	minVersion := semver.New(TSHMinVersion)
	if version.LessThan(*minVersion) {
		return trace.Errorf(
			"installed tsh version %s does not support Machine ID proxies, "+
				"please upgrade to at least %s",
			version, minVersion,
		)
	}

	log.Debugf("tsh version %s is supported", version)

	return nil
}

// GetDestination attempts to select an unambiguous destination, either from
// CLI or YAML config. It returns an error if the selected destination is
// invalid.
func GetDestination(botConfig *config.BotConfig, cf *config.CLIConf) (*config.DestinationConfig, error) {
	// Note: this only supports filesystem destinations.
	if cf.DestinationDir != "" {
		dest, err := botConfig.GetDestinationByPath(cf.DestinationDir)
		if err != nil {
			return nil, trace.Wrap(err, "unable to find destination %s in the "+
				"configuration; has the configuration file been "+
				"specified with `-c <path>`?", cf.DestinationDir)
		}

		return dest, nil
	}

	if len(botConfig.Destinations) == 0 {
		return nil, trace.BadParameter("either --destination-dir or a config file must be specified")
	} else if len(botConfig.Destinations) > 1 {
		return nil, trace.BadParameter("the config file contains multiple destinations; a --destination-dir must be specified")
	}

	return botConfig.Destinations[0], nil
}

// GetDestinationPath returns a path to a filesystem destination.
func GetDestinationPath(destination *config.DestinationConfig) (string, error) {
	destinationImpl, err := destination.GetDestination()
	if err != nil {
		return "", trace.Wrap(err)
	}

	destinationDir, ok := destinationImpl.(*config.DestinationDirectory)
	if !ok {
		return "", trace.BadParameter("destination %s must be a directory", destinationImpl)
	}

	return destinationDir.Path, nil
}

// GetTLSCATemplate returns the TLS CA template for the given destination. It's
// a required template so this should never fail.
func GetTLSCATemplate(destination *config.DestinationConfig) (*config.TemplateTLSCAs, error) {
	tpl := destination.GetConfigByName(config.TemplateTLSCAsName)
	if tpl == nil {
		return nil, trace.NotFound("no template with name %s found, this is a bug", config.TemplateTLSCAsName)
	}

	tlsCAs, ok := tpl.(*config.TemplateTLSCAs)
	if !ok {
		return nil, trace.BadParameter("invalid TLS CA template")
	}

	return tlsCAs, nil
}

// GetIdentityTemplate returns the identity template for the given destination.
// This is a required template so it _should_ never fail.
func GetIdentityTemplate(destination *config.DestinationConfig) (*config.TemplateIdentity, error) {
	tpl := destination.GetConfigByName(config.TemplateIdentityName)
	if tpl == nil {
		return nil, trace.NotFound("no template with name %s found, this is a bug", config.TemplateIdentityName)
	}

	identity, ok := tpl.(*config.TemplateIdentity)
	if !ok {
		return nil, trace.BadParameter("invalid identity template")
	}

	return identity, nil
}

// mergeEnv applies the given value to each key inside the specified map.
func mergeEnv(m map[string]string, value string, keys ...string) {
	for _, key := range keys {
		m[key] = value
	}
}

// GetEnvForTSH returns a map of environment variables needed to properly wrap
// tsh so that it uses our Machine ID certificates where necessary.
func GetEnvForTSH(destination *config.DestinationConfig) (map[string]string, error) {
	tlsCAs, err := GetTLSCATemplate(destination)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	destPath, err := GetDestinationPath(destination)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The env var interface does allow us to set specific resource names for
	// everything but also has generic fallbacks. We'll use the fallbacks for
	// now but could eventually communicate more info to tsh if desired.
	env := make(map[string]string)
	mergeEnv(env, filepath.Join(destPath, identity.PrivateKeyKey), client.VirtualPathEnvNames(client.VirtualPathKey, nil)...)

	// Database certs are a bit awkward since a few databases (cockroach) have
	// special naming requirements. We can document around these for now and
	// automate later. (I don't think tsh handles this perfectly today anyway).
	mergeEnv(env, filepath.Join(destPath, identity.TLSCertKey), client.VirtualPathEnvNames(client.VirtualPathDatabase, nil)...)

	mergeEnv(env, filepath.Join(destPath, identity.TLSCertKey), client.VirtualPathEnvNames(client.VirtualPathApp, nil)...)

	// We don't want to provide a fallback for CAs since it would be ambiguous,
	// so we'll specify them exactly.
	mergeEnv(env,
		filepath.Join(destPath, tlsCAs.UserCAPath),
		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.UserCA)),
	)
	mergeEnv(env,
		filepath.Join(destPath, tlsCAs.HostCAPath),
		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.HostCA)),
	)
	mergeEnv(env,
		filepath.Join(destPath, tlsCAs.DatabaseCAPath),
		client.VirtualPathEnvName(client.VirtualPathCA, client.VirtualPathCAParams(types.DatabaseCA)),
	)

	// TODO: Kubernetes support. We don't generate kubeconfigs yet, so we have
	// nothing to give tsh for now.

	return env, nil
}

// AnyArgsStartWith determines if any of the string arguments have the given
// prefix. Useful for determining if an arg for tsh got caught in
// RemainingArgs.
func AnyArgsStartWith(prefix string, args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}

	return false
}

// LoadIdentity loads a Teleport identity from an identityfile. Secondary bot
// identities are not loadable, so we'll just read the Teleport identity (which
// is required for tsh to function anyway).
func LoadIdentity(identityPath string) (*tlsca.Identity, error) {
	f, err := os.Open(identityPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	idFile, err := identityfile.Read(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tlsca.ParseCertificatePEM(idFile.Certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsed, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parsed, nil
}
