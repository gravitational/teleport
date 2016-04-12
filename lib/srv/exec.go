/*
Copyright 2015 Gravitational, Inc.

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

package srv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"

	"golang.org/x/crypto/ssh"
)

var osxUserShellRegexp = regexp.MustCompile("UserShell: (/[^ ]+)\n")

type execResult struct {
	command string
	code    int
}

type execReq struct {
	Command string
}

// execResponse prepares the response to a 'exec' SSH request, i.e. executing
// a command after making an SSH connection and delivering the result back.
type execResponse struct {
	cmdName string
	cmd     *exec.Cmd
	ctx     *ctx

	// TODO(klizhentas) implement capturing as a threadsafe, factored out feature
	// that uses protected writes & reads to the buffer

	// 'out' contains captured command output
	out *bytes.Buffer
}

func parseExecRequest(req *ssh.Request, ctx *ctx) (*execResponse, error) {
	var e execReq
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		return nil, fmt.Errorf("failed to parse exec request, error: %v", err)
	}
	return &execResponse{
		ctx:     ctx,
		cmdName: e.Command,
		out:     &bytes.Buffer{},
	}, nil
}

func (e *execResponse) String() string {
	return fmt.Sprintf("Exec(cmd=%v)", e.cmdName)
}

// getLoginShell determines the login shell for a given username
func getLoginShell(username string) (string, error) {
	// func to determine user shell on OSX:
	forMac := func() (string, error) {
		dir := "Local/Default/Users/" + username
		out, err := exec.Command("dscl", "localhost", "-read", dir, "UserShell").Output()
		if err != nil {
			return "", err
		}
		m := osxUserShellRegexp.FindStringSubmatch(string(out))
		shell := m[1]
		if shell == "" {
			return "", trace.Errorf("dscl output parsing error getting shell for %s", username)
		}
		return shell, nil
	}
	// func to determine user shell on other unixes (linux)
	forUnix := func() (string, error) {
		f, err := os.Open("/etc/passwd")
		if err != nil {
			return "", trace.Wrap(err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Split(strings.TrimSpace(scanner.Text()), ":")
			if parts[0] == username && len(parts) > 5 {
				return parts[6], nil
			}
		}
		if scanner.Err() != nil {
			log.Error(scanner.Err())
		}
		return "", trace.Errorf("cannot determine shell for %s", username)
	}
	if runtime.GOOS == "darwin" {
		return forMac()
	} else {
		return forUnix()
	}
}

// prepareOSCommand configures os.Cmd for executing a given command within an SSH
// session.
//
// If args are empty, it means a simple shell must be launched
// Otherwise, a shell launches with "-c args" as parameters
func prepareOSCommand(ctx *ctx, args ...string) (*exec.Cmd, error) {
	osUserName := ctx.login
	// configure UID & GID of the requested OS user:
	osUser, err := user.Lookup(osUserName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uid, err := strconv.Atoi(osUser.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gid, err := strconv.Atoi(osUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	curUser, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// determine shell for the given OS user:
	shellCommand, err := getLoginShell(osUserName)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// in test mode short-circuit to /bin/sh
	if ctx.isTestStub {
		shellCommand = "/bin/sh"
	}
	if len(args) > 0 {
		orig := args
		args = []string{"-c"}
		args = append(args, orig...)
	}

	// try to determine the host name of the 1st available proxy to set a nicer
	// session URL. fall back to <proxyhost> placeholder
	proxyHost := "<proxyhost>"
	if ctx.srv != nil {
		proxies, err := ctx.srv.authService.GetProxies()
		if err != nil {
			log.Error(err)
		}
		if len(proxies) > 0 {
			proxyHost = proxies[0].Hostname
		}
	}

	log.Infof("created OS command '%s' with params: '%v'", shellCommand, args)
	c := exec.Command(shellCommand, args...)
	c.Env = []string{
		"TERM=xterm",
		"LANG=en_US.UTF-8",
		"HOME=" + osUser.HomeDir,
		"USER=" + osUserName,
		"SHELL=" + c.Path,
		"SSH_TELEPORT_USER=" + ctx.teleportUser,
		fmt.Sprintf("SSH_SESSION_WEBPROXY_ADDR=%s:3080", proxyHost),
	}
	// this configures shell to run in 'login' mode
	c.Args[0] = "-" + filepath.Base(shellCommand)
	c.Dir = osUser.HomeDir
	c.SysProcAttr = &syscall.SysProcAttr{}
	// execute the command under requested user's UID:GID
	if curUser.Uid != osUser.Uid {
		c.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	// apply environment variables passed from the client
	for n, v := range ctx.env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", n, v))
	}
	// apply SSH_xx environment variables
	remoteHost, remotePort, err := net.SplitHostPort(ctx.info.RemoteAddr().String())
	if err != nil {
		log.Warn(err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.info.LocalAddr().String())
		if err != nil {
			log.Warn(err)
		} else {
			c.Env = append(c.Env,
				fmt.Sprintf("SSH_CLIENT=%s %s %s", remoteHost, remotePort, localPort),
				fmt.Sprintf("SSH_CONNECTION=%s %s %s %s", remoteHost, remotePort, localHost, localPort))
		}
	}
	if ctx.session != nil {
		if ctx.session.term != nil {
			c.Env = append(c.Env, fmt.Sprintf("SSH_TTY=%s", ctx.session.term.tty.Name()))
		}
		if ctx.session.id != "" {
			c.Env = append(c.Env, fmt.Sprintf("SSH_SESSION_ID=%s", ctx.session.id))
		}
	}
	return c, nil
}

// start launches the given command returns (nil, nil) if successful. execResult is only used
// to communicate an error while launching
func (e *execResponse) start(sconn *ssh.ServerConn, shell string, ch ssh.Channel) (*execResult, error) {
	var err error
	e.cmd, err = prepareOSCommand(e.ctx, e.cmdName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// capture output to the buffer
	e.cmd.Stdout = io.MultiWriter(e.out, ch)
	e.cmd.Stderr = io.MultiWriter(e.out, ch)

	// TODO(klizhentas) figure out the way to see if stdin is ever needed.
	// e.cmd.Stdin = ch leads to the following problem:
	// e.cmd.Wait() never returns  because stdin never gets closed or never reached
	// see cmd.Stdin comments
	e.cmd.Stdin = nil

	if err := e.cmd.Start(); err != nil {
		e.ctx.Warningf("%v start failure err: %v", e, err)
		return e.collectStatus(e.cmd, trace.ConvertSystemError(err))
	}
	e.ctx.Infof("%v started", e)
	return nil, nil
}

func (e *execResponse) collectStatus(cmd *exec.Cmd, err error) (*execResult, error) {
	status, err := collectStatus(e.cmd, err)
	if err != nil {
		e.ctx.emit(events.NewExec(e.cmdName, e.out, -1, err))
	} else {
		e.ctx.emit(events.NewExec(e.cmdName, e.out, status.code, err))
	}
	return status, err
}

func (e *execResponse) wait() (*execResult, error) {
	if e.cmd.Process == nil {
		e.ctx.Errorf("no process")
	}
	return e.collectStatus(e.cmd, e.cmd.Wait())
}

func collectStatus(cmd *exec.Cmd, err error) (*execResult, error) {
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			return &execResult{code: status.ExitStatus(), command: cmd.Path}, nil
		}
		return nil, err
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return nil, fmt.Errorf("unspoorted exit status: %T(%v)", cmd.ProcessState.Sys(), cmd.ProcessState.Sys())
	}
	return &execResult{code: status.ExitStatus(), command: cmd.Path}, nil
}
