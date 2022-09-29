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

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// openSSHVersionRegex is a regex used to parse OpenSSH version strings.
var openSSHVersionRegex = regexp.MustCompile(`^OpenSSH_(?P<major>\d+)\.(?P<minor>\d+)(?:p(?P<patch>\d+))?`)

// openSSHMinVersionForRSAWorkaround is the OpenSSH version after which the
// RSA deprecation workaround should be added to generated ssh_config.
var openSSHMinVersionForRSAWorkaround = semver.New("8.5.0")

// openSSHMinVersionForHostAlgos is the first version that understands all host keys required by us.
// HostKeyAlgorithms will be added to ssh config if the version is above listed here.
var openSSHMinVersionForHostAlgos = semver.New("7.8.0")

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
	PubkeyAcceptedKeyTypesWorkaroundNeeded bool
	NewerHostKeyAlgorithmsSupported        bool
}

func (c *SSHConfigOptions) String() string {
	sb := &strings.Builder{}
	sb.WriteString("SSHConfigOptions:")

	if c.PubkeyAcceptedKeyTypesWorkaroundNeeded {
		sb.WriteString(" PubkeyAcceptedKeyTypes with SHA-1 will be added")
	} else {
		sb.WriteString(" PubkeyAcceptedKeyTypes will not be added")
	}

	if c.PubkeyAcceptedKeyTypesWorkaroundNeeded {
		sb.WriteString(", HostKeyAlgorithms will include SHA-1")
	} else {
		sb.WriteString(", HostKeyAlgorithms will include SHA-256 and SHA-512")
	}

	return sb.String()
}

func IsPubkeyAcceptedKeyTypesWorkaroundNeeded(ver *semver.Version) bool {
	return ver.LessThan(*openSSHMinVersionForRSAWorkaround)
}

func IsNewerHostKeyAlgorithmsSupported(ver *semver.Version) bool {
	return !ver.LessThan(*openSSHMinVersionForHostAlgos)
}

func GetSSHConfigOptions(sshVer *semver.Version) *SSHConfigOptions {
	return &SSHConfigOptions{
		PubkeyAcceptedKeyTypesWorkaroundNeeded: IsPubkeyAcceptedKeyTypesWorkaroundNeeded(sshVer),
		NewerHostKeyAlgorithmsSupported:        IsNewerHostKeyAlgorithmsSupported(sshVer),
	}
}

func GetDefaultSSHConfigOptions() *SSHConfigOptions {
	return &SSHConfigOptions{
		PubkeyAcceptedKeyTypesWorkaroundNeeded: true,
		NewerHostKeyAlgorithmsSupported:        false,
	}
}
