// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"bytes"
	"cmp"
	"context"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	renameio "github.com/google/renameio/v2/maybe" // Writes aren't guaranteed to be atomic on Windows.
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

const (
	filePerms                      os.FileMode = 0o600
	sshConfigurationUpdateInterval             = 30 * time.Second
)

// writeSSHKeys writes hostCAKey to ${TELEPORT_HOME}/vnet_known_hosts so that
// third-party SSH clients can trust it. It then reads or generates
// ${TELEPORT_HOME}/id_vnet(.pub) which SSH clients should be configured to use
// for connections to VNet SSH. It returns id_vnet.pub so that VNet SSH can
// trust it for incoming connections.
func writeSSHKeys(homePath string, hostCAKey ssh.PublicKey) (ssh.PublicKey, error) {
	profilePath := fullProfilePath(homePath)
	if err := writeKnownHosts(profilePath, hostCAKey); err != nil {
		return nil, trace.Wrap(err)
	}
	userPubKey, err := readUserPubKey(profilePath)
	if trace.IsNotFound(err) {
		userPubKey, err = generateAndWriteUserKey(profilePath)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return userPubKey, nil
}

func fullProfilePath(homePath string) string {
	if homePath == "" {
		if homeDir := os.Getenv(types.HomeEnvVar); homeDir != "" {
			homePath = filepath.Clean(homeDir)
		}
	}
	return profile.FullProfilePath(homePath)
}

func writeKnownHosts(profilePath string, hostCAKey ssh.PublicKey) error {
	// MarshalAuthorizedKey serializes the key for inclusion in an
	// authorized_keys file, we need to add the @cert-authority prefix and the
	// wildcard so this CA is trusted for all hosts. The SSH configuration file
	// should only load this vnet_known_hosts file for hosts matching
	// appropriate subdomains, there is no need to keep that list of domains
	// updated in both the SSH config file and the vnet_known_hosts file.
	authorizedKey := ssh.MarshalAuthorizedKey(hostCAKey)
	authorizedCA := "@cert-authority * " + string(authorizedKey)
	p := keypaths.VNetKnownHostsPath(profilePath)
	err := renameio.WriteFile(p, []byte(authorizedCA), filePerms)
	return trace.Wrap(trace.ConvertSystemError(err), "writing host CA to %s", p)
}

func readUserPubKey(profilePath string) (ssh.PublicKey, error) {
	p := keypaths.VNetClientSSHKeyPubPath(profilePath)
	f, err := os.Open(p)
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "opening %s for reading", p)
	}
	defer f.Close()
	const maxPubKeyFileSize = 10000 // RSA 4096 pub key files are ~750 bytes, ~10x to be safe.
	pubKeyBytes, err := io.ReadAll(io.LimitReader(f, maxPubKeyFileSize))
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "reading user public key from %s", p)
	}
	userPubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	return userPubKey, trace.Wrap(err, "parsing user public key from %s", p)
}

func generateAndWriteUserKey(profilePath string) (ssh.PublicKey, error) {
	userKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, trace.Wrap(err, "generating SSH user key")
	}

	privPemBlock, err := ssh.MarshalPrivateKey(userKey, "")
	if err != nil {
		return nil, trace.Wrap(err, "marshaling SSH user key")
	}
	privKeyBytes := pem.EncodeToMemory(privPemBlock)
	privKeyPath := keypaths.VNetClientSSHKeyPath(profilePath)
	if err := renameio.WriteFile(privKeyPath, privKeyBytes, filePerms); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "writing user private key to %s", privKeyPath)
	}

	userPubKey, err := ssh.NewPublicKey(userKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubKeyPath := keypaths.VNetClientSSHKeyPubPath(profilePath)
	if err := renameio.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(userPubKey), filePerms); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err), "writing user public key to %s", pubKeyPath)
	}
	return userPubKey, nil
}

// sshConfigurator writes an OpenSSH-compatible config file to
// TELEPORT_HOME/vnet_ssh_config, and keeps it up to date with the list of
// clusters that should match.
type sshConfigurator struct {
	cfg         sshConfiguratorConfig
	profilePath string
	clock       clockwork.Clock
}

type sshConfiguratorConfig struct {
	clientApplication ClientApplication
	homePath          string
	clock             clockwork.Clock
}

func newSSHConfigurator(cfg sshConfiguratorConfig) *sshConfigurator {
	return &sshConfigurator{
		cfg:         cfg,
		profilePath: fullProfilePath(cfg.homePath),
		clock:       cmp.Or(cfg.clock, clockwork.NewRealClock()),
	}
}

func (c *sshConfigurator) runConfigurationLoop(ctx context.Context) error {
	if err := c.updateSSHConfiguration(ctx); err != nil {
		return trace.Wrap(err, "generating vnet_ssh_config")
	}
	// Delete the configuration file before exiting, if it is imported by the
	// default SSH config file it will just stop taking effect.
	defer func() {
		if err := deleteSSHConfigFile(c.profilePath); err != nil {
			log.WarnContext(ctx, "Failed to delete vnet_ssh_config while shutting down", "error", err)
		}
	}()
	// clock.After is intentionally used in the loop instead of a ticker simply
	// for more reliable testing. In the test I use clock.BlockUntilContext(1)
	// to block until the loop is stuck waiting on the clock. If I used
	// clock.NewTicker instead, the ticker always counts as a waiter, and that
	// strategy doesn't work. In go 1.25 we can use testing/synctest instead.
	for {
		select {
		case <-c.clock.After(sshConfigurationUpdateInterval):
			if err := c.updateSSHConfiguration(ctx); err != nil {
				return trace.Wrap(err, "updating vnet_ssh_config")
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "context canceled, shutting down vnet_ssh_config update loop")
		}
	}
}

func (c *sshConfigurator) updateSSHConfiguration(ctx context.Context) error {
	profileNames, err := c.cfg.clientApplication.ListProfiles()
	if err != nil {
		return trace.Wrap(err, "listing profiles")
	}
	hostMatchers := make([]string, 0, len(profileNames))
	for _, profileName := range profileNames {
		rootClient, err := c.cfg.clientApplication.GetCachedClient(ctx, profileName, "" /*leafClusterName*/)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to get root cluster client from cache, profile may be expired, not configuring VNet SSH for this cluster",
				"profile", profileName, "error", err)
			continue
		}
		hostMatchers = append(hostMatchers, hostMatcher(rootClient.RootClusterName()))
	}
	hostMatchers = utils.Deduplicate(hostMatchers)
	slices.Sort(hostMatchers)
	hostMatchersString := strings.Join(hostMatchers, " ")
	return trace.Wrap(writeSSHConfigFile(c.profilePath, hostMatchersString))
}

func hostMatcher(clusterName string) string {
	return "*." + strings.Trim(clusterName, ".")
}

func deleteSSHConfigFile(profilePath string) error {
	p := keypaths.VNetSSHConfigPath(profilePath)
	if err := os.Remove(p); err != nil {
		err = trace.ConvertSystemError(err)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err, "deleting %s", p)
	}
	return nil
}

func writeSSHConfigFile(profilePath, hostMatchers string) error {
	t := template.Must(template.New("ssh_config").Parse(configFileTemplate))
	var b bytes.Buffer
	if err := t.Execute(&b, configFileTemplateInput{
		Hosts:          hostMatchers,
		PrivateKeyPath: strconv.Quote(keypaths.VNetClientSSHKeyPath(profilePath)),
		KnownHostsPath: strconv.Quote(keypaths.VNetKnownHostsPath(profilePath)),
	}); err != nil {
		return trace.Wrap(err, "generating SSH config file")
	}
	p := keypaths.VNetSSHConfigPath(profilePath)
	err := renameio.WriteFile(p, b.Bytes(), filePerms)
	return trace.Wrap(trace.ConvertSystemError(err), "writing SSH config file to %s", p)
}

const configFileTemplate = `Host {{ .Hosts }}
    IdentityFile {{ .PrivateKeyPath }}
    GlobalKnownHostsFile {{ .KnownHostsPath }}
    UserKnownHostsFile /dev/null
    StrictHostKeyChecking yes
    IdentitiesOnly yes
`

type configFileTemplateInput struct {
	Hosts          string
	PrivateKeyPath string
	KnownHostsPath string
}
