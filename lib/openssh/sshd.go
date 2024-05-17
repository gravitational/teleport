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

package openssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
)

var (
	// SSHDConfigPath is the path to write teleport specific SSHD config options
	sshdConfigFile = "sshd.conf"
	// SSHDKeysDir is the path to the Teleport openssh keys
	SSHDKeysDir = "openssh"
)

func sshdConfigInclude(dataDir string) string {
	return fmt.Sprintf("Include %s", filepath.Join(dataDir, sshdConfigFile))
}

const DefaultRestartCommand = "systemctl restart sshd"

const (
	// TeleportKey is the name the OpenSSH private key
	TeleportKey = "ssh_host_teleport_key"
	// TeleportKey is the name the OpenSSH cert
	TeleportCert = TeleportKey + "-cert.pub"
	// TeleportOpenSSHCA is the path to the Teleport OpenSSHCA
	TeleportOpenSSHCA = "teleport_openssh_ca.pub"
)

type sshdBackendOperations interface {
	restart() error
	checkConfig(path string) error
}

// SSHD is used to update the OpenSSH config
type SSHD struct {
	sshd sshdBackendOperations
}

// NewSSHD initializes SSHD
func NewSSHD(restartCmd string, checkCmd string, sshdConfigPath string) SSHD {
	return SSHD{
		sshd: &sshdBackend{
			restartCmd:     restartCmd,
			checkCmd:       checkCmd,
			sshdConfigPath: sshdConfigPath,
		},
	}
}

// SSHDConfigUpdate is the list of options to be set in the Teleport OpenSSH config
type SSHDConfigUpdate struct {
	// SSHDConfigPath is the path to the OpenSSH sshd_config file
	SSHDConfigPath string
	// DataDir is the path to the global Teleport datadir
	DataDir string
}

var sshdConfigTmpl = template.Must(template.New("sshd_config_include").Parse(`# Created by 'teleport join openssh', do not edit
TrustedUserCAKeys {{ .OpenSSHCAPath }}
HostKey {{ .HostKeyPath }}
HostCertificate {{ .HostCertPath }}
`))

func fmtSSHDConfigUpdate(u SSHDConfigUpdate) (string, error) {
	type SSHDConfigUpdateBackend struct {
		SSHDConfigUpdate
		// OpenSSHCAPath is the path to which Teleport OpenSSHCA will be written
		OpenSSHCAPath string
		// HostKeyPath is the path to the Teleport Host Key
		HostKeyPath string
		// HostCertPath is the path to the Teleport OpenSSH cert
		HostCertPath string
	}
	keysDir := filepath.Join(u.DataDir, SSHDKeysDir)
	update := SSHDConfigUpdateBackend{
		SSHDConfigUpdate: u,
		OpenSSHCAPath:    filepath.Join(keysDir, TeleportOpenSSHCA),
		HostKeyPath:      filepath.Join(keysDir, TeleportKey),
		HostCertPath:     filepath.Join(keysDir, TeleportCert),
	}

	buf := &bytes.Buffer{}
	if err := sshdConfigTmpl.Execute(buf, update); err != nil {
		return "", trace.Wrap(err)
	}
	return buf.String(), nil
}

// UpdateConfig updates the sshd_config file if needed and writes the
// teleport specific configuration
func (s *SSHD) UpdateConfig(u SSHDConfigUpdate, restart bool) error {
	configUpdate, err := fmtSSHDConfigUpdate(u)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := writeTempAndRename(filepath.Join(u.DataDir, sshdConfigFile), s.sshd.checkConfig, []byte(configUpdate)); err != nil {
		return trace.Wrap(err)
	}

	sshdConfigInclude := sshdConfigInclude(u.DataDir)

	needsUpdate, err := checkSSHDConfigAlreadyUpdated(u.SSHDConfigPath, sshdConfigInclude)
	if err != nil {
		return trace.Wrap(err)
	}
	if needsUpdate {
		if err := prependToSSHDConfig(u.SSHDConfigPath, sshdConfigInclude); err != nil {
			return trace.Wrap(err)
		}
	}

	if restart {
		if err := s.sshd.restart(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// WriteKeys writes the OpenSSH keys and CA from the Identity and the
// OpenSSH CA to disk for the OpenSSH daemon to use
func WriteKeys(keysdir string, id *state.Identity, cas []types.CertAuthority) error {
	if err := os.MkdirAll(keysdir, 0o755); err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := writeTempAndRename(filepath.Join(keysdir, TeleportKey), nil, id.KeyBytes); err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := writeTempAndRename(filepath.Join(keysdir, TeleportCert), nil, id.CertBytes); err != nil {
		return trace.ConvertSystemError(err)
	}

	var caKeyBytes []byte
	for _, ca := range cas {
		for _, key := range ca.GetTrustedSSHKeyPairs() {
			pubKey := append(bytes.TrimSpace(key.PublicKey), byte('\n'))
			caKeyBytes = append(caKeyBytes, pubKey...)
		}
	}

	if err := writeTempAndRename(filepath.Join(keysdir, TeleportOpenSSHCA), nil, caKeyBytes); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

type sshdBackend struct {
	restartCmd     string
	checkCmd       string
	sshdConfigPath string
}

var _ sshdBackendOperations = &sshdBackend{}

func (b *sshdBackend) checkConfig(path string) error {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("%s %q", b.checkCmd, path))
	if err := cmd.Run(); err != nil {
		output, outErr := cmd.CombinedOutput()
		if err != nil {
			return trace.Wrap(trace.NewAggregate(err, outErr), "invalid sshd config file, failed to get `%s %q` output", b.checkCmd, path)
		}
		return trace.Wrap(err, "invalid sshd config file %q, not writing", string(output))
	}
	return nil
}

func (b *sshdBackend) restart() error {
	if err := b.checkConfig(b.sshdConfigPath); err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.Command("/bin/sh", "-c", b.restartCmd)
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "failed to restart the sshd service")
	}
	return nil
}

// writeTempAndRename creates a temporary file with 0o600 permissions,
// and writes contents to the it, if checkfunc passes without error,
// it'll then rename the file to the path specified with configPath
func writeTempAndRename(configPath string, checkFunc func(string) error, contents []byte) error {
	configTmp, err := os.CreateTemp(filepath.Dir(configPath), "")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	tmpName := configTmp.Name()
	defer configTmp.Close()
	defer os.Remove(tmpName)

	_, err = configTmp.Write(contents)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := configTmp.Close(); err != nil {
		return trace.Wrap(err)
	}

	if checkFunc != nil {
		if err := checkFunc(tmpName); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := os.Rename(tmpName, configPath); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func prependToSSHDConfig(sshdConfigPath, config string) error {
	contents, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	line := append([]byte(config), byte('\n'))
	contents = append(line, contents...)

	if err := writeTempAndRename(sshdConfigPath, nil, contents); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func checkSSHDConfigAlreadyUpdated(sshdConfigPath, fileContains string) (bool, error) {
	contents, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	return !strings.Contains(string(contents), fileContains), nil
}
