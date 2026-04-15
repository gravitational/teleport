package x11

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/session/envutils"
	"github.com/gravitational/teleport/session/reexec"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
	"github.com/gravitational/trace"
)

// GetAvailableXSessions return xsessions available in the system with optional filtering
func GetAvailableXSessions(included, excluded *regexp.Regexp) (map[string]string, error) {
	path, exists := os.LookupEnv("TELEPORT_XSESSIONS_PATH")
	if !exists {
		path = "/usr/share/xsessions"
	}
	entries := make(map[string]string)
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, entry := range dirEntries {
		var found bool
		var fileName string
		if fileName, found = strings.CutSuffix(entry.Name(), ".desktop"); !found {
			continue
		}
		if included != nil && !included.MatchString(fileName) {
			continue
		}
		if excluded != nil && excluded.MatchString(fileName) {
			continue
		}
		file, err := os.Open(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		scanner := bufio.NewScanner(file)
		var name string
		var exec string
		for scanner.Scan() {
			if s, found := strings.CutPrefix(scanner.Text(), "Name="); found {
				name = s
			} else if s, found := strings.CutPrefix(scanner.Text(), "Exec="); found {
				exec = s
			}
			if name != "" && exec != "" {
				entries[name] = exec
				break
			}
		}
		file.Close()
	}
	return entries, nil
}

// XSessionConfig is configuration used for starting xsession for specified user.
type XSessionConfig struct {
	Logger *slog.Logger

	// ChildLogConfig contains logger configuration for the child process.
	ChildLogConfig *srv.ChildLogConfig

	// Command is command to execute to start xsession.
	Command string

	// Username is the username associated with the Teleport identity.
	Username string

	// Login is the local *nix account.
	Login string

	// Display is X11 display string (:N) to use for connection to X11 server.
	Display string
	// AuthorityFile is XAuthority file used to secure connection to X11 server.
	AuthorityFile string
}

// StartTeleportExecXSession reexecs the current Teleport binary using
// `teleport exec` and runs the provided start command.
func StartTeleportExecXSession(ctx context.Context, cfg *XSessionConfig) (*reexec.CommandExecutor, error) {
	if cfg.ChildLogConfig == nil {
		return nil, trace.BadParameter("missing parameter ChildLogConfig")
	}

	env := envutils.SafeEnv{}
	env.AddTrusted("DISPLAY", cfg.Display)
	env.AddTrusted("XAUTHORITY", cfg.AuthorityFile)

	cmdmsg := &reexec.ExecCommand{
		Command:         cfg.Command,
		ForceLoginShell: true,
		RequestType:     sshutils.ExecRequest,
		Login:           cfg.Login,
		Username:        cfg.Username,
		Environment:     env,
		LogConfig:       cfg.ChildLogConfig.ExecLogConfig,
	}

	inr, inw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	inw.Close()
	defer inr.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer outw.Close()

	go func() {
		scanner := bufio.NewScanner(outr)
		for scanner.Scan() {
			line := scanner.Text()
			cfg.Logger.Log(ctx, logutils.TraceLevel, line)
		}
		outr.Close()
	}()

	cmd, err := reexec.ConfigureCommand(ctx, cfg.Logger, cfg.ChildLogConfig.Writer, cmdmsg, reexecconstants.ExecSubCommand, inr, outw, outw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}
