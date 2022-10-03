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

package config

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var sshConfigTemplate = template.Must(template.New("ssh-config").Parse(
	`# Begin generated Teleport configuration for {{ .ProxyHost }} by {{ .AppName }}
{{$dot := . }}
{{- range $clusterName := .ClusterNames }}
# Common flags for all {{ $clusterName }} hosts
Host *.{{ $clusterName }} {{ $dot.ProxyHost }}
    UserKnownHostsFile "{{ $dot.KnownHostsPath }}"
    IdentityFile "{{ $dot.IdentityFilePath }}"
    CertificateFile "{{ $dot.CertificateFilePath }}"{{- if $dot.NewerHostKeyAlgorithmsSupported }}
    HostKeyAlgorithms rsa-sha2-256-cert-v01@openssh.com,rsa-sha2-512-cert-v01@openssh.com{{- else }}
    HostKeyAlgorithms ssh-rsa-cert-v01@openssh.com{{- end }}

# Flags for all {{ $clusterName }} hosts except the proxy
Host *.{{ $clusterName }} !{{ $dot.ProxyHost }}
    Port 3022
    {{- if eq $dot.AppName "tsh" }}
    ProxyCommand "{{ $dot.ExecutablePath }}" proxy ssh --cluster={{ $clusterName }} --proxy={{ $dot.ProxyHost }} %r@%h:%p
{{- end }}{{- if eq $dot.AppName "tbot" }}
    ProxyCommand "{{ $dot.ExecutablePath }}" proxy --destination-dir={{ $dot.DestinationDir }} --proxy={{ $dot.ProxyHost }} ssh --cluster={{ $clusterName }}  %r@%h:%p
{{- end }}
{{- end }}

# End generated Teleport configuration
`))

type SSHConfigParameters struct {
	AppName             SSHConfigApps
	ClusterNames        []string
	KnownHostsPath      string
	IdentityFilePath    string
	CertificateFilePath string
	ProxyHost           string
	ExecutablePath      string
	DestinationDir      string
}

type sshTmplParams struct {
	SSHConfigParameters
	SSHConfigOptions
}

// openSSHVersionRegex is a regex used to parse OpenSSH version strings.
var openSSHVersionRegex = regexp.MustCompile(`^OpenSSH_(?P<major>\d+)\.(?P<minor>\d+)(?:p(?P<patch>\d+))?`)

// openSSHMinVersionForHostAlgos is the first version that understands all host keys required by us.
// HostKeyAlgorithms will be added to ssh config if the version is above listed here.
var openSSHMinVersionForHostAlgos = semver.New("7.8.0")

type SSHConfigApps string

const (
	TshApp  SSHConfigApps = teleport.ComponentTSH
	TbotApp SSHConfigApps = teleport.ComponentTBot
)

// parseSSHVersion attempts to parse the local SSH version, used to determine
// certain config template parameters for client version compatibility.
func parseSSHVersion(versionString string) (*semver.Version, error) {
	versionTokens := strings.Split(versionString, " ")
	if len(versionTokens) == 0 {
		return nil, trace.BadParameter("invalid version string: %s", versionString)
	}

	versionID := versionTokens[0]
	matches := openSSHVersionRegex.FindStringSubmatch(versionID)
	if matches == nil {
		return nil, trace.BadParameter("cannot parse version string: %q", versionID)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, trace.Wrap(err, "invalid major version number: %s", matches[1])
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, trace.Wrap(err, "invalid minor version number: %s", matches[2])
	}

	patch := 0
	if matches[3] != "" {
		patch, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, trace.Wrap(err, "invalid patch version number: %s", matches[3])
		}
	}

	return &semver.Version{
		Major: int64(major),
		Minor: int64(minor),
		Patch: int64(patch),
	}, nil
}

// GetSystemSSHVersion attempts to query the system SSH for its current version.
func GetSystemSSHVersion() (*semver.Version, error) {
	var out bytes.Buffer

	cmd := exec.Command("ssh", "-V")
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parseSSHVersion(out.String())
}

type SSHConfigOptions struct {
	// NewerHostKeyAlgorithmsSupported when true sets HostKeyAlgorithms OpenSSH configuration option
	// to SHA256/512 compatible algorithms. Otherwise, SHA-1 is being used.
	NewerHostKeyAlgorithmsSupported bool
}

func (c *SSHConfigOptions) String() string {
	sb := &strings.Builder{}
	sb.WriteString("SSHConfigOptions:")

	if c.NewerHostKeyAlgorithmsSupported {
		sb.WriteString("HostKeyAlgorithms will include SHA-1")
	} else {
		sb.WriteString("HostKeyAlgorithms will include SHA-256 and SHA-512")
	}

	return sb.String()
}

func IsNewerHostKeyAlgorithmsSupported(ver *semver.Version) bool {
	return !ver.LessThan(*openSSHMinVersionForHostAlgos)
}

func GetSSHConfigOptions(sshVer *semver.Version) *SSHConfigOptions {
	return &SSHConfigOptions{
		NewerHostKeyAlgorithmsSupported: IsNewerHostKeyAlgorithmsSupported(sshVer),
	}
}

func GetDefaultSSHConfigOptions() *SSHConfigOptions {
	return &SSHConfigOptions{
		NewerHostKeyAlgorithmsSupported: false,
	}
}

type SSHConfig struct {
	getSSHVersion func() (*semver.Version, error)
	log           logrus.FieldLogger
}

func NewSSHConfig(getSSHVersion func() (*semver.Version, error), log logrus.FieldLogger) *SSHConfig {
	return &SSHConfig{getSSHVersion: getSSHVersion, log: log}
}

func (c *SSHConfig) GetSSHConfig(sb *strings.Builder, config *SSHConfigParameters) error {
	var sshConfigOptions *SSHConfigOptions
	version, err := c.getSSHVersion()
	if err != nil {
		c.log.WithError(err).Debugf("Could not determine SSH version, using default SSH config")
		sshConfigOptions = GetDefaultSSHConfigOptions()
	} else {
		c.log.Debugf("Found OpenSSH version %s", version)
		sshConfigOptions = GetSSHConfigOptions(version)
	}

	c.log.Debugf("Using SSH options: %s", sshConfigOptions)

	if err := sshConfigTemplate.Execute(sb, sshTmplParams{
		SSHConfigParameters: *config,
		SSHConfigOptions:    *sshConfigOptions,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
