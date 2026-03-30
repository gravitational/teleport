package x11

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/envutils"
	"github.com/gravitational/trace"
)

func GetAvailableXSessions() (map[string]string, error) {
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
		if !strings.HasSuffix(entry.Name(), ".desktop") {
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

type XSessionConfig struct {
	Logger     *slog.Logger
	Command    string
	Username   string
	Login      string
	LogConfig  *srv.ChildLogConfig
	Display    string
	RemoteAddr utils.NetAddr
}

// StartTeleportExecXSession reexecs the current Teleport binary using
// `teleport exec` and runs the provided start command.
//
// It wires the same control-pipe protocol used by SSH reexecs in
// lib/srv/reexec.go.
func StartTeleportExecXSession(ctx context.Context, cfg *XSessionConfig) (*exec.Cmd, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmdr, cmdw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer cmdw.Close()
	defer cmdr.Close()

	contr, contw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	contw.Close()
	defer contr.Close()

	readyr, readyw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer readyr.Close()
	defer readyw.Close()

	var logw *os.File

	// If the log writer is a file, we can pass it directly to the child
	// process to write to. Otherwise, we need to create a pipe to the child
	// process and stream the logs to the log writer.
	logCfg := cfg.LogConfig

	if fileWriter, ok := logCfg.Writer.(*os.File); ok {
		logw = fileWriter
	} else {
		// Create a pipe so we can pass the writing side as an *os.File to the child process.
		// Then we can copy from the reading side to the log writer (e.g. syslog, log file w/ concurrency protection).
		r, w, err := os.Pipe()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer w.Close()
		logw = w

		// Copy logs from the child process to the parent process over
		// the pipe until it is closed by the child context.
		go func() {
			if _, err := io.Copy(logCfg.Writer, r); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				cfg.Logger.ErrorContext(ctx, "Failed to copy logs over pipe", "error", err)
			}
		}()
	}

	killr, killw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer killr.Close()

	env := envutils.SafeEnv{}
	env.AddTrusted("DISPLAY", cfg.Display)

	cmdmsg := &srv.ExecCommand{
		Command:         cfg.Command,
		ForceLoginShell: true,
		RequestType:     sshutils.ExecRequest,
		Login:           cfg.Login,
		Username:        cfg.Username,
		Environment:     env,
		UaccMetadata: srv.UaccMetadata{
			RemoteAddr: cfg.RemoteAddr,
		},
		LogConfig: srv.ExecLogConfig{
			Level:        logCfg.Level,
			Format:       logCfg.Format,
			ExtraFields:  logCfg.ExtraFields,
			EnableColors: logCfg.EnableColors,
			Padding:      logCfg.Padding,
		},
	}

	cmd := exec.CommandContext(ctx, executable, teleport.ExecSubCommand)
	cmd.ExtraFiles = []*os.File{
		cmdr,
		logw,
		contr,
		readyw,
		killr,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Cancel = killw.Close
	if err := cmd.Start(); err != nil {
		killw.Close()
		return nil, trace.Wrap(err)
	}

	// Close parent-side copies of child FDs after process start.
	cmdr.Close()
	contr.Close()
	readyw.Close()
	killr.Close()

	if err := json.NewEncoder(cmdw).Encode(cmdmsg); err != nil {
		killw.Close()
		return nil, trace.Wrap(err)
	}

	if err := waitForChildReadySignal(readyr, 10*time.Second); err != nil {
		_ = cmd.Cancel()
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

func waitForChildReadySignal(f *os.File, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		_, err := f.Read(make([]byte, 1))
		if err == io.EOF {
			waitCh <- nil
			return
		}
		waitCh <- err
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-waitCh:
		return trace.Wrap(err)
	case <-timer.C:
		return trace.LimitExceeded("timed out waiting for teleport reexec readiness signal")
	}
}
