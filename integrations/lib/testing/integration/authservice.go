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

var regexpAuthStarting = regexp.MustCompile(`Auth service.*listen_address:[^ ]+:(\d+)`)

type AuthService struct {
	mu           sync.Mutex
	teleportPath string
	configPath   string
	authAddr     Addr
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

func newAuthService(teleportPath, configPath string) *AuthService {
	var auth AuthService
	var setErrOnce, setReadyOnce sync.Once
	readyCh := make(chan struct{})
	auth = AuthService{
		teleportPath: teleportPath,
		configPath:   configPath,
		readyCh:      readyCh,
		doneCh:       make(chan struct{}),
		terminate:    func() {}, // dummy noop that will be overridden by Run(),
		setErr: func(err error) {
			setErrOnce.Do(func() {
				auth.mu.Lock()
				defer auth.mu.Unlock()
				auth.error = err
			})
		},
		setReady: func(isReady bool) {
			setReadyOnce.Do(func() {
				auth.mu.Lock()
				auth.isReady = isReady
				auth.mu.Unlock()
				close(readyCh)
			})
		},
	}
	return &auth
}

// Run spawns an auth server instance.
func (auth *AuthService) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log := logger.Get(ctx)

	cmd := exec.CommandContext(ctx, auth.teleportPath, "start", "--debug", "--config", auth.configPath)
	log.Debugf("Running Auth service: %s", cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stdout")
		auth.setErr(err)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stderr")
		auth.setErr(err)
		return err
	}

	if err := cmd.Start(); err != nil {
		err = trace.Wrap(err, "failed to start teleport")
		auth.setErr(err)
		return err
	}

	ctx, log = logger.WithField(ctx, "pid", cmd.Process.Pid)
	log.Debug("Auth service process has been started")

	auth.mu.Lock()
	var terminateOnce sync.Once
	auth.terminate = func() {
		terminateOnce.Do(func() {
			log.Debug("Terminating Auth service process")
			// Signal the process to gracefully terminate by sending SIGQUIT.
			if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
				log.Warn(err)
			}
			// If we're not done in 5 minutes, just kill the process by canceling its context.
			go func() {
				select {
				case <-auth.doneCh:
				case <-time.After(serviceShutdownTimeout):
					log.Debug("Killing Auth service process")
				}
				// cancel() results in sending SIGKILL to a process if it's still alive.
				cancel()
			}()
		})
	}
	auth.mu.Unlock()

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

			auth.saveStdout(line)

			if auth.IsReady() {
				continue
			}

			auth.parseLine(ctx, line)
			if addr := auth.AuthAddr(); !addr.IsEmpty() {
				log.Debugf("Found addr of Auth service process: %v", addr)
				auth.setReady(true)
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
			auth.saveStderr(data[:n])
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
		auth.setErr(err)
		close(auth.doneCh)
	}()

	<-auth.doneCh

	if !auth.IsReady() {
		log.Error("Auth server is failed to initialize")
		stdoutLines := strings.Split(auth.Stdout(), "\n")
		for _, line := range stdoutLines {
			log.Debug("AuthService log: ", line)
		}
		log.Debugf("AuthService stderr: %q", auth.Stderr())

		// If it's still not ready lets signal that it's finally not ready.
		auth.setReady(false)
		// Set an err just in case if it's not set before.
		auth.setErr(trace.Errorf("failed to initialize"))
	}

	return trace.Wrap(auth.Err())
}

// AuthAddr returns auth service external address.
func (auth *AuthService) AuthAddr() Addr {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	return auth.authAddr
}

// ConfigPath returns auth service config file path.
func (auth *AuthService) ConfigPath() string {
	return auth.configPath
}

// Err returns auth server error. It's nil If process is not done yet.
func (auth *AuthService) Err() error {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	return auth.error
}

// Shutdown terminates the auth server process and waits for its completion.
func (auth *AuthService) Shutdown(ctx context.Context) error {
	auth.doTerminate()
	select {
	case <-auth.doneCh:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// Stdout returns a collected auth server process stdout.
func (auth *AuthService) Stdout() string {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	return auth.stdout.String()
}

// Stderr returns a collected auth server process stderr.
func (auth *AuthService) Stderr() string {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	return auth.stderr.String()
}

// WaitReady waits for auth server initialization.
func (auth *AuthService) WaitReady(ctx context.Context) (bool, error) {
	select {
	case <-auth.readyCh:
		return auth.IsReady(), nil
	case <-ctx.Done():
		return false, trace.Wrap(ctx.Err(), "auth server is not ready")
	}
}

// IsReady indicates if auth server is initialized properly.
func (auth *AuthService) IsReady() bool {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	return auth.isReady
}

func (auth *AuthService) doTerminate() {
	auth.mu.Lock()
	terminate := auth.terminate
	auth.mu.Unlock()
	terminate()
}

func (auth *AuthService) parseLine(ctx context.Context, line string) {
	submatch := regexpAuthStarting.FindStringSubmatch(line)
	if submatch != nil {
		auth.mu.Lock()
		defer auth.mu.Unlock()
		auth.authAddr = Addr{Host: "127.0.0.1", Port: submatch[1]}
		return
	}
}

func (auth *AuthService) saveStdout(line string) {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	auth.stdout.WriteString(line)
}

func (auth *AuthService) saveStderr(chunk []byte) {
	auth.mu.Lock()
	defer auth.mu.Unlock()
	auth.stderr.Write(chunk)
}
