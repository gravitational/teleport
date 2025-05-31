package diag

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
	"github.com/gravitational/trace"
)

type SSHConfig struct {
	ProfilePath string
}

type SSHDiag struct {
	cfg               *SSHConfig
	userSSHConfigPath string
}

func NewSSHDiag(cfg *SSHConfig) (*SSHDiag, error) {
	userSSHConfigPath, err := defaultUserSSHConfigPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SSHDiag{
		cfg:               cfg,
		userSSHConfigPath: userSSHConfigPath,
	}, nil
}

func (d *SSHDiag) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	vnetSSHConfigured, err := OpenSSHConfigIncludesVNetSSHConfig(ctx, d.cfg.ProfilePath, d.userSSHConfigPath)
	if err != nil {
		return nil, trace.Wrap(err, "checking if the default user OpenSSH config includes VNet's SSH configuration")
	}
	return &diagv1.CheckReport{
		Status: diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK,
		Report: &diagv1.CheckReport_SshConfigurationReport{
			SshConfigurationReport: &diagv1.SSHConfigurationReport{
				VnetSshConfigured: vnetSSHConfigured,
			},
		},
	}, nil
}

func (d *SSHDiag) Commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{
		exec.Command("cat", d.userSSHConfigPath),
	}
}

func (d *SSHDiag) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{Report: &diagv1.CheckReport_SshConfigurationReport{}}
}

// OpenSSHConfigIncludesVNetSSHConfig returns true if the given OpenSSH config
// file probably includes the vnet_ssh_config file under profilePath. That is,
// it returns true if r contains a line like:
//
//	Include <profilePath>/vnet_ssh_config
//
// It always returns false if vnet_ssh_config is not included, but it may return
// false positives.
func OpenSSHConfigIncludesVNetSSHConfig(ctx context.Context, profilePath, userSSHConfigPath string) (bool, error) {
	sshConfigFile, err := os.Open(userSSHConfigPath)
	if err != nil {
		return false, trace.Wrap(trace.ConvertSystemError(err), "opening %s for reading", userSSHConfigPath)
	}
	defer sshConfigFile.Close()
	return fileIncludesVNetSSHConfig(profilePath, sshConfigFile)
}

func defaultUserSSHConfigPath() (string, error) {
	userHomeDir, ok := profile.UserHomeDir()
	if !ok {
		return "", trace.Errorf("unable to find user's home directory")
	}
	return filepath.Join(userHomeDir, ".ssh", "config"), nil
}

// fileIncludesVNetSSHConfig returns true if the given OpenSSH config file
// probably includes the vnet_ssh_config file under profilePath.
//
// It always returns false if vnet_ssh_config is not included, but it may return
// false positives because the SSH config format is a tricky to parse, the full
// path:
// - may be quoted
// - may include escape characters
// - may use unix-style paths "/home/user/.tsh/vnet_ssh_config" on either OS
// - may use windows-style paths "C:\\Users\\User\\.tsh\\vnet_ssh_config"
// - may or may not include spaces or special characters
// - may or may not use ~ to refer to the user's home directory
// So it really just checks if there's an include line that matches the leaf
// directory in profilePath:
// - for Connect's "~/Application\ Support/Teleport\ Connect/tsh" this will be "tsh"
// - for tsh's "~/.tsh" this will be ".tsh"
// followed by a path separator, followed by "vnet_ssh_config".
//
// This way at least it will return false if tsh's vnet_ssh_config is included
// but the current profilePath belongs to Connect, or vice-versa. It also always
// returns false if the file is not included at all.
func fileIncludesVNetSSHConfig(profilePath string, r io.Reader) (bool, error) {
	leafDir := filepath.Base(profilePath)
	// Quote any regex meta characters in leafDir to match it literally.
	leafDir = regexp.QuoteMeta(leafDir)
	// Whitespace is trimmed from each line, here's a breakdown of the regex:
	// ^(?i:include)\s the line must start with include followed by whitespace
	//   ?i makes the match for "include" case-insensitive
	// [^#]+ swallows any characters in the path prefix that don't start a comment
	// (/|\\\\) matches a path separator / or \\
	// leafDir matches the last component of profilePath
	// (/|\\\\) matches a path separator / or \\
	// keypaths.VNetSSHConfig matches vnet_ssh_config
	// \b means a word boundary must follow vnet_ssh_config
	includePattern := `^(?i:include)\s[^#]+(/|\\\\)` + leafDir + `(/|\\\\)` + keypaths.VNetSSHConfig + `\b`
	re, err := regexp.Compile(includePattern)
	if err != nil {
		return false, trace.Wrap(err, "compiling regex to match OpenSSH include lines")
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if re.MatchString(line) {
			return true, nil
		}
	}
	return false, trace.Wrap(trace.ConvertSystemError(scanner.Err()), "reading OpenSSH config file")
}
