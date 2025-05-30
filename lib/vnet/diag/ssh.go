package diag

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
	"github.com/gravitational/trace"
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
	cfg                   *SSHConfig
	userHome              string
	userOpenSSHConfigPath string
	vnetSSHConfigPath     string
	isWindows             bool
}

// NewSSHDiag returns a new [SSHDiag].
func NewSSHDiag(cfg *SSHConfig) (*SSHDiag, error) {
	userHome, ok := profile.UserHomeDir()
	if !ok {
		return nil, trace.Errorf("unable to find user's home directory")
	}
	userOpenSSHConfigPath := filepath.Join(userHome, ".ssh", "config")
	vnetSSHConfigPath := filepath.Join(cfg.ProfilePath, keypaths.VNetSSHConfig)
	return &SSHDiag{
		cfg:                   cfg,
		userHome:              userHome,
		userOpenSSHConfigPath: userOpenSSHConfigPath,
		vnetSSHConfigPath:     vnetSSHConfigPath,
		isWindows:             runtime.GOOS == "windows",
	}, nil
}

// Run runs the diagnostic.
func (d *SSHDiag) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	included, err := d.isVNetSSHConfigIncluded(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "checking if the default user OpenSSH config includes VNet's SSH configuration")
	}
	return &diagv1.CheckReport{
		// This intentionally always returns CHECK_REPORT_STATUS_OK even if
		// ~/.ssh/config does not include the VNet generated SSH config. It is
		// not mandatory to configure SSH and returning an error status would
		// case an alert and notification in Connect.
		Status: diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK,
		Report: &diagv1.CheckReport_SshConfigurationReport{
			SshConfigurationReport: &diagv1.SSHConfigurationReport{
				UserOpensshConfigPath:                  d.userOpenSSHConfigPath,
				VnetSshConfigPath:                      d.vnetSSHConfigPath,
				UserOpensshConfigIncludesVnetSshConfig: included,
			},
		},
	}, nil
}

// Commands returns a command that prints the default user OpenSSH config file
// which may be helpful for debugging.
func (d *SSHDiag) Commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{
		exec.CommandContext(ctx, "cat", d.userOpenSSHConfigPath),
	}
}

// EmptyCheckReport returns an empty SSH configuration report.
func (d *SSHDiag) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{Report: &diagv1.CheckReport_SshConfigurationReport{}}
}

func (d *SSHDiag) isVNetSSHConfigIncluded(ctx context.Context) (bool, error) {
	openSSHConfigFile, err := os.Open(d.userOpenSSHConfigPath)
	if err != nil {
		return false, trace.Wrap(trace.ConvertSystemError(err), "opening %s for reading", d.userOpenSSHConfigPath)
	}
	defer openSSHConfigFile.Close()
	return d.openSSHConfigIncludesVNetSSHConfig(openSSHConfigFile)
}

func (d *SSHDiag) openSSHConfigIncludesVNetSSHConfig(r io.Reader) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if d.openSSHConfigLineIncludesPath(scanner.Text(), d.vnetSSHConfigPath) {
			return true, nil
		}
	}
	return false, trace.Wrap(scanner.Err())
}

// openSSHConfigLineIncludesPath returns true if the given line of an OpenSSH
// configuration file is an include statement for the given path.
func (d *SSHDiag) openSSHConfigLineIncludesPath(line, wantPath string) bool {
	wantPath = d.normalizePath(wantPath)
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
	// at unescaped and unqouted whitespace and if the argument matches wantPath
	// returns true. It does support ~ as an alias for the user's home
	// directory.
	var (
		// b is a running buffer holding the current argument as parsed up to
		// the current point.
		b        strings.Builder
		escape   = false
		inQuotes = false
	)
loop:
	for _, c := range line {
		switch {
		case escape:
			// Always write escaped characters literally.
			b.WriteRune(c)
			escape = false
		case c == '\\':
			// The next character is escaped.
			escape = true
		case c == '"':
			// Entering or exiting a quoted section.
			inQuotes = !inQuotes
		case b.Len() == 0 && c == '~':
			// Support ~ as an alias for the user's home directory.
			b.WriteString(d.userHome)
		case !inQuotes && isSpace(c):
			// Reached the end of this argument, check if it matches wantPath.
			if d.normalizePath(b.String()) == wantPath {
				return true
			}
			b.Reset()
		case !inQuotes && c == '#':
			// Found a comment in the middle of the line, ignore the rest.
			break loop
		default:
			// By default just append the current character to the current
			// argument.
			b.WriteRune(c)
		}
	}
	// Handle an argument that ends at the end of the line.
	return d.normalizePath(b.String()) == wantPath
}

func (d *SSHDiag) normalizePath(path string) string {
	if d.isWindows {
		// Normalize all paths to use unix-style separators since OpenSSH
		// supports / or \\ on Windows.
		path = strings.ReplaceAll(path, `\`, `/`)
		// Windows paths are case-insensitive.
		path = strings.ToLower(path)
	}
	return filepath.Clean(path)
}

func userSSHConfigPath(userHome string) string {
	return filepath.Join(userHome, ".ssh", "config")
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t':
		return true
	}
	return false
}
