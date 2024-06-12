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

package integration

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

var regexpSSHStarting = regexp.MustCompile(`SSH Service is starting.*listen_address:[^ ]+:(\d+)`)

type SSHService struct {
	mu           sync.Mutex
	teleportPath string
	configPath   string
	sshAddr      Addr
	isReady      bool
	readyCh      chan struct{}
	doneCh       chan struct{}
	terminate    context.CancelFunc
	setErr       func(error)
	setReady     func(bool)
	error        error
	stdout       strings.Builder
	stderr       bytes.Buffer
}

func newSSHService(teleportPath, configPath string) *SSHService {
	var ssh SSHService
	var setErrOnce, setReadyOnce sync.Once
	readyCh := make(chan struct{})
	ssh = SSHService{
		teleportPath: teleportPath,
		configPath:   configPath,
		readyCh:      readyCh,
		doneCh:       make(chan struct{}),
		terminate:    func() {}, // dummy noop that will be overridden by Run(),
		setErr: func(err error) {
			setErrOnce.Do(func() {
				ssh.mu.Lock()
				defer ssh.mu.Unlock()
				ssh.error = err
			})
		},
		setReady: func(isReady bool) {
			setReadyOnce.Do(func() {
				ssh.mu.Lock()
				ssh.isReady = isReady
				ssh.mu.Unlock()
				close(readyCh)
			})
		},
	}
	return &ssh
}

// Run spawns an ssh service instance.
func (ssh *SSHService) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log := logger.Get(ctx)

	cmd := exec.CommandContext(ctx, ssh.teleportPath, "start", "--debug", "--config", ssh.configPath)
	log.Debugf("Running SSH service: %s", cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stdout")
		ssh.setErr(err)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stderr")
		ssh.setErr(err)
		return err
	}

	if err := cmd.Start(); err != nil {
		err = trace.Wrap(err, "failed to start teleport")
		ssh.setErr(err)
		return err
	}

	ctx, log = logger.WithField(ctx, "pid", cmd.Process.Pid)
	log.Debug("SSH service process has been started")

	ssh.mu.Lock()
	var terminateOnce sync.Once
	ssh.terminate = func() {
		terminateOnce.Do(func() {
			log.Debug("Terminating SSH service process")
			// Signal the process to gracefully terminate by sending SIGQUIT.
			if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
				log.Warn(err)
			}
			// If we're not done in 5 minutes, just kill the process by canceling its context.
			go func() {
				select {
				case <-ssh.doneCh:
				case <-time.After(serviceShutdownTimeout):
					log.Debug("Killing SSH service process")
				}
				// cancel() results in sending SIGKILL to a process if it's still alive.
				cancel()
			}()
		})
	}
	ssh.mu.Unlock()

	var ioWork sync.WaitGroup
	ioWork.Add(2)

	// Parse stdout of a Teleport process.
	go func() {
		defer ioWork.Done()

		stdout := bufio.NewReader(stdoutPipe)
		for {
			line, err := stdout.ReadString('\n')
			if errors.Is(err, io.EOF) {
				return
			}
			if err := trace.Wrap(err); err != nil {
				log.WithError(err).Error("failed to read process stdout")
				return
			}

			ssh.saveStdout(line)

			if ssh.IsReady() {
				continue
			}

			ssh.parseLine(ctx, line)
			if strings.Contains(line, "The new service has started successfully.") {
				log.Debugf("Found addr of SSH service process: %v", ssh.sshAddr)
				ssh.setReady(true)
			}
		}
	}()

	// Save stderr to a buffer.
	go func() {
		defer ioWork.Done()

		stderr := bufio.NewReader(stderrPipe)
		data := make([]byte, stderr.Size())
		for {
			n, err := stderr.Read(data)
			ssh.saveStderr(data[:n])
			if errors.Is(err, io.EOF) {
				return
			}
			if err := trace.Wrap(err); err != nil {
				log.WithError(err).Error("failed to read process stderr")
				return
			}
		}
	}()

	// Wait for process completeness after processing both outputs.
	go func() {
		ioWork.Wait()
		err := trace.Wrap(cmd.Wait())
		ssh.setErr(err)
		close(ssh.doneCh)
	}()

	<-ssh.doneCh

	if !ssh.IsReady() {
		log.Error("SSH service is failed to initialize")
		stdoutLines := strings.Split(ssh.Stdout(), "\n")
		for _, line := range stdoutLines {
			log.Debug("SSH service log: ", line)
		}
		log.Debugf("SSH service stderr: %q", ssh.Stderr())

		// If it's still not ready lets signal that it's finally not ready.
		ssh.setReady(false)
		// Set an err just in case if it's not set before.
		ssh.setErr(trace.Errorf("failed to initialize"))
	}

	return trace.Wrap(ssh.Err())
}

// Addr returns SSH external address.
func (ssh *SSHService) Addr() Addr {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	return ssh.sshAddr
}

// Err returns ssh service error. It's nil If process is not done yet.
func (ssh *SSHService) Err() error {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	return ssh.error
}

// Shutdown terminates the ssh service process and waits for its completion.
func (ssh *SSHService) Shutdown(ctx context.Context) error {
	ssh.doTerminate()
	select {
	case <-ssh.doneCh:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// Stdout returns a collected ssh service process stdout.
func (ssh *SSHService) Stdout() string {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	return ssh.stdout.String()
}

// Stderr returns a collected ssh service process stderr.
func (ssh *SSHService) Stderr() string {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	return ssh.stderr.String()
}

// WaitReady waits for ssh service initialization.
func (ssh *SSHService) WaitReady(ctx context.Context) (bool, error) {
	select {
	case <-ssh.readyCh:
		return ssh.IsReady(), nil
	case <-ctx.Done():
		return false, trace.Wrap(ctx.Err(), "ssh service is not ready")
	}
}

// IsReady indicates if ssh service is initialized properly.
func (ssh *SSHService) IsReady() bool {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	return ssh.isReady
}

func (ssh *SSHService) doTerminate() {
	ssh.mu.Lock()
	terminate := ssh.terminate
	ssh.mu.Unlock()
	terminate()
}

func (ssh *SSHService) parseLine(ctx context.Context, line string) {
	if submatch := regexpSSHStarting.FindStringSubmatch(line); submatch != nil {
		ssh.mu.Lock()
		defer ssh.mu.Unlock()
		ssh.sshAddr = Addr{Host: "127.0.0.1", Port: submatch[1]}
		return
	}
}

func (ssh *SSHService) saveStdout(line string) {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	ssh.stdout.WriteString(line)
}

func (ssh *SSHService) saveStderr(chunk []byte) {
	ssh.mu.Lock()
	defer ssh.mu.Unlock()
	ssh.stderr.Write(chunk)
}
