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

package diag

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

const (
	maxOpenSSHConfigFileSize = 1 * 1024 * 1024 // 1 MiB
)

// SSHConfig includes everything that [SSHDiag] needs to run.
type SSHConfig struct {
	// ProfilePath is the path to the user profile (TELEPORT_HOME) where VNet's
	// SSH configuration file is stored.
	ProfilePath string
}

// SSHDiag is a diagnostic check that inspects whether the default user OpenSSH
// config file includes VNet's generated SSH config file.
type SSHDiag struct {
	cfg              *SSHConfig
	sshConfigChecker *SSHConfigChecker
}

// NewSSHDiag returns a new [SSHDiag].
func NewSSHDiag(cfg *SSHConfig) (*SSHDiag, error) {
	sshConfigChecker, err := NewSSHConfigChecker(cfg.ProfilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SSHDiag{
		cfg:              cfg,
		sshConfigChecker: sshConfigChecker,
	}, nil
}

// Commands returns no commands for this diagnostic.
func (d *SSHDiag) Commands(ctx context.Context) []*exec.Cmd {
	return nil
}

// EmptyCheckReport returns an empty SSH configuration report.
func (d *SSHDiag) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{Report: &diagv1.CheckReport_SshConfigurationReport{}}
}

// Run runs the diagnostic.
func (d *SSHDiag) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	report, err := d.run(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &diagv1.CheckReport{
		// This intentionally always returns CHECK_REPORT_STATUS_OK even if
		// ~/.ssh/config does not include the VNet generated SSH config. It is
		// not mandatory to configure SSH and returning an error status would
		// cause an alert and notification in Connect.
		Status: diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK,
		Report: &diagv1.CheckReport_SshConfigurationReport{
			SshConfigurationReport: report,
		},
	}, nil
}

func (d *SSHDiag) run(ctx context.Context) (*diagv1.SSHConfigurationReport, error) {
	userOpenSSHConfigContents, included, err := d.sshConfigChecker.OpenSSHConfigIncludesVNetSSHConfig()
	if err != nil {
		if trace.IsNotFound(err) {
			return &diagv1.SSHConfigurationReport{
				UserOpensshConfigPath: d.sshConfigChecker.UserOpenSSHConfigPath,
				VnetSshConfigPath:     d.sshConfigChecker.VNetSSHConfigPath,
			}, nil
		}
		return nil, trace.Wrap(err)
	}
	if !utf8.Valid(userOpenSSHConfigContents) {
		return nil, trace.Errorf("%s is not valid UTF-8", d.sshConfigChecker.UserOpenSSHConfigPath)
	}
	return &diagv1.SSHConfigurationReport{
		UserOpensshConfigPath:                  d.sshConfigChecker.UserOpenSSHConfigPath,
		VnetSshConfigPath:                      d.sshConfigChecker.VNetSSHConfigPath,
		UserOpensshConfigIncludesVnetSshConfig: included,
		UserOpensshConfigExists:                true,
		UserOpensshConfigContents:              string(userOpenSSHConfigContents),
	}, nil
}

// SSHConfigChecker checks the state of the user's SSH configuration.
type SSHConfigChecker struct {
	userHome              string
	UserOpenSSHConfigPath string
	VNetSSHConfigPath     string
	isWindows             bool
}

// NewSSHConfigChecker returns a new SSHConfigChecker.
func NewSSHConfigChecker(profilePath string) (*SSHConfigChecker, error) {
	userHome, ok := profile.UserHomeDir()
	if !ok {
		return nil, trace.Errorf("unable to find user's home directory")
	}
	userOpenSSHConfigPath := filepath.Join(userHome, ".ssh", "config")
	vnetSSHConfigPath := keypaths.VNetSSHConfigPath(profilePath)
	return &SSHConfigChecker{
		userHome:              userHome,
		UserOpenSSHConfigPath: userOpenSSHConfigPath,
		VNetSSHConfigPath:     vnetSSHConfigPath,
		isWindows:             runtime.GOOS == "windows",
	}, nil
}

// OpenSSHConfigIncludesVNetSSHConfig returns the current user OpenSSH
// configuration file contents (~/.ssh/config) and a boolean indicating whether
// it already includes VNet's generated OpenSSH-compatible configuration file.
//
// If ~/.ssh/config does not exist it returns a [trace.NotFoundError]
func (c *SSHConfigChecker) OpenSSHConfigIncludesVNetSSHConfig() ([]byte, bool, error) {
	userOpenSSHConfigFile, err := os.Open(c.UserOpenSSHConfigPath)
	if err != nil {
		return nil, false, trace.Wrap(trace.ConvertSystemError(err), "opening %s for reading", c.UserOpenSSHConfigPath)
	}
	defer userOpenSSHConfigFile.Close()

	userOpenSSHConfigContents, err := io.ReadAll(io.LimitReader(userOpenSSHConfigFile, maxOpenSSHConfigFileSize))
	if err != nil {
		return nil, false, trace.Wrap(trace.ConvertSystemError(err), "reading %s", c.UserOpenSSHConfigPath)
	}
	if len(userOpenSSHConfigContents) == maxOpenSSHConfigFileSize {
		return nil, false, trace.Errorf("%s is too large to (max size %s)",
			c.UserOpenSSHConfigPath, humanize.Bytes(maxOpenSSHConfigFileSize))
	}

	included, err := c.openSSHConfigIncludesVNetSSHConfig(bytes.NewReader(userOpenSSHConfigContents))
	if err != nil {
		return nil, false, trace.Wrap(err, "checking if the default user OpenSSH config includes VNet's SSH configuration")
	}
	return userOpenSSHConfigContents, included, nil
}

func (c *SSHConfigChecker) openSSHConfigIncludesVNetSSHConfig(r io.Reader) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if c.openSSHConfigLineIncludesPath(scanner.Text(), c.VNetSSHConfigPath) {
			return true, nil
		}
	}
	return false, trace.Wrap(scanner.Err())
}

// openSSHConfigLineIncludesPath returns true if the given line of an OpenSSH
// configuration file is an include statement for the given path.
func (c *SSHConfigChecker) openSSHConfigLineIncludesPath(line, wantPath string) bool {
	wantPath = c.normalizePath(wantPath)
	line = strings.TrimSpace(line)

	// Only consider lines that begin with "include" (case-insensitive).
	i := strings.IndexFunc(line, isSpace)
	if i == -1 {
		return false
	}
	if strings.ToLower(line[:i]) != "include" {
		return false
	}
	// Consider the rest of the line after "include".
	line = line[i+1:]

	// Include lines may specify multiple pathnames and each pathname may
	// contain glob wildcards, tokens, environment variables, ~, escaped
	// characters and may or may not be quoted. This function does not support
	// glob wildcards, tokens, or environment variables. It splits each argument
	// at unescaped and unquoted whitespace and if the argument matches wantPath
	// returns true. It does support ~ as an alias for the user's home
	// directory.
	var (
		// pathBuf is a running buffer holding the current argument as parsed up to
		// the current point.
		pathBuf strings.Builder
		// quote holds the opening quote character if one has been found.
		quote = byte(0)
	)
loop:
	for i := 0; i < len(line); i++ {
		b := line[i]
		switch {
		case b == '\\' && i < len(line)-1 && canBeEscaped(line[i+1]):
			// Skip the escape char and write the next char literally.
			i++
			pathBuf.WriteByte(line[i])
		case quote == 0 && (b == '"' || b == '\''):
			// Start of quote
			quote = b
		case quote != 0 && b == quote:
			// End of quote
			quote = 0
		case pathBuf.Len() == 0 && b == '~':
			// Support ~ as an alias for the user's home directory.
			pathBuf.WriteString(c.userHome)
		case quote == 0 && b == '#':
			// Found an unquoted comment in the middle of the line, ignore the rest.
			break loop
		case quote == 0 && isSpace(rune(b)):
			// Reached the end of this argument, check if it matches wantPath.
			if c.normalizePath(pathBuf.String()) == wantPath {
				return true
			}
			pathBuf.Reset()
		default:
			// By default just append the current byte to the path.
			pathBuf.WriteByte(b)
		}
	}
	if quote != 0 {
		// Unmatched quote.
		return false
	}
	// Handle an argument that ends at the end of the line.
	return c.normalizePath(pathBuf.String()) == wantPath
}

func (c *SSHConfigChecker) normalizePath(path string) string {
	if c.isWindows {
		// Normalize all paths to use unix-style separators since OpenSSH
		// supports / or \\ on Windows.
		path = strings.ReplaceAll(path, `\`, `/`)
		// Windows paths are case-insensitive.
		path = strings.ToLower(path)
	}
	return filepath.Clean(path)
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t':
		return true
	}
	return false
}

func canBeEscaped(c byte) bool {
	// https://github.com/openssh/openssh-portable/blob/5f761cdb2331a12318bde24db5ca84ee144a51d1/misc.c#L2089-L2099
	switch c {
	case ' ', '\\', '\'', '"':
		return true
	}
	return false
}
