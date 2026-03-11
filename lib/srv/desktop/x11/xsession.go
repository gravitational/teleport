package x11

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
)

var executablePath = os.Executable
var currentUser = user.Current

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

// StartTeleportExecXSession reexecs the current Teleport binary using
// `teleport exec` and runs the provided start command.
//
// It wires the same control-pipe protocol used by SSH reexecs in
// lib/srv/reexec.go.
func StartTeleportExecXSession(startCommand string) (*exec.Cmd, error) {
	startCommand = strings.TrimSpace(startCommand)
	if startCommand == "" {
		return nil, trace.BadParameter("start command is required")
	}

	usr, err := currentUser()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	executable, err := executablePath()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmdr, cmdw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer cmdw.Close()

	contr, contw, err := os.Pipe()
	if err != nil {
		cmdr.Close()
		return nil, trace.Wrap(err)
	}
	defer contw.Close()

	readyr, readyw, err := os.Pipe()
	if err != nil {
		cmdr.Close()
		contr.Close()
		return nil, trace.Wrap(err)
	}
	defer readyr.Close()

	killr, killw, err := os.Pipe()
	if err != nil {
		cmdr.Close()
		contr.Close()
		readyw.Close()
		return nil, trace.Wrap(err)
	}

	cmdmsg := &srv.ExecCommand{
		Command:     startCommand,
		RequestType: sshutils.ExecRequest,
		Login:       usr.Username,
		Username:    usr.Username,
	}
	if err := json.NewEncoder(cmdw).Encode(cmdmsg); err != nil {
		cmdr.Close()
		contr.Close()
		readyw.Close()
		killr.Close()
		killw.Close()
		return nil, trace.Wrap(err)
	}

	cmd := &exec.Cmd{
		Path: executable,
		Args: []string{executable, teleport.ExecSubCommand},
		ExtraFiles: []*os.File{
			cmdr,
			os.Stderr,
			contr,
			readyw,
			killr,
		},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Start(); err != nil {
		cmdr.Close()
		contr.Close()
		readyw.Close()
		killr.Close()
		killw.Close()
		return nil, trace.Wrap(err)
	}

	// Close parent-side copies of child FDs after process start.
	cmdr.Close()
	contr.Close()
	readyw.Close()
	killr.Close()

	// The terminate writer must remain open while the child runs.
	cmd.Cancel = func() error {
		return killw.Close()
	}

	if err := waitForPipeClose(readyr, 10*time.Second); err != nil {
		_ = cmd.Cancel()
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

func waitForPipeClose(f *os.File, timeout time.Duration) error {
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
