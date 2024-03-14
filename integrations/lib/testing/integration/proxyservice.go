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
	"fmt"
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

var regexpWebProxyStarting = regexp.MustCompile(`Starting web proxy service.*listen_address:[^ ]+:(\d+)`)
var regexpSSHProxyStarting = regexp.MustCompile(`Starting SSH proxy service.*listen_address:[^ ]+:(\d+)`)
var regexpReverseTunnelStarting = regexp.MustCompile(`Starting reverse tunnel server.*listen_address:[^ ]+:(\d+)`)

type ProxyService struct {
	mu                sync.Mutex
	teleportPath      string
	configPath        string
	webProxyAddr      Addr
	sshProxyAddr      Addr
	reverseTunnelAddr Addr
	isReady           bool
	readyCh           chan struct{}
	doneCh            chan struct{}
	terminate         context.CancelFunc
	setErr            func(error)
	setReady          func(bool)
	error             error
	stdout            strings.Builder
	stderr            bytes.Buffer
}

func newProxyService(teleportPath, configPath string) *ProxyService {
	var proxy ProxyService
	var setErrOnce, setReadyOnce sync.Once
	readyCh := make(chan struct{})
	proxy = ProxyService{
		teleportPath: teleportPath,
		configPath:   configPath,
		readyCh:      readyCh,
		doneCh:       make(chan struct{}),
		terminate:    func() {}, // dummy noop that will be overridden by Run(),
		setErr: func(err error) {
			setErrOnce.Do(func() {
				proxy.mu.Lock()
				defer proxy.mu.Unlock()
				proxy.error = err
			})
		},
		setReady: func(isReady bool) {
			setReadyOnce.Do(func() {
				proxy.mu.Lock()
				proxy.isReady = isReady
				proxy.mu.Unlock()
				close(readyCh)
			})
		},
	}
	return &proxy
}

// Run spawns an proxy service instance.
func (proxy *ProxyService) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log := logger.Get(ctx)

	cmd := exec.CommandContext(ctx, proxy.teleportPath, "start", "--debug", "--config", proxy.configPath)
	log.Debugf("Running Proxy service: %s", cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stdout")
		proxy.setErr(err)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		err = trace.Wrap(err, "failed to get stderr")
		proxy.setErr(err)
		return err
	}

	if err := cmd.Start(); err != nil {
		err = trace.Wrap(err, "failed to start teleport")
		proxy.setErr(err)
		return err
	}

	ctx, log = logger.WithField(ctx, "pid", cmd.Process.Pid)
	log.Debug("Proxy service process has been started")

	proxy.mu.Lock()
	var terminateOnce sync.Once
	proxy.terminate = func() {
		terminateOnce.Do(func() {
			log.Debug("Terminating Proxy service process")
			// Signal the process to gracefully terminate by sending SIGQUIT.
			if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
				log.Warn(err)
			}
			// If we're not done in 5 minutes, just kill the process by canceling its context.
			go func() {
				select {
				case <-proxy.doneCh:
				case <-time.After(serviceShutdownTimeout):
					log.Debug("Killing Proxy service process")
				}
				// cancel() results in sending SIGKILL to a process if it's still alive.
				cancel()
			}()
		})
	}
	proxy.mu.Unlock()

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

			proxy.saveStdout(line)

			if proxy.IsReady() {
				continue
			}

			proxy.parseLine(ctx, line)

			if strings.Contains(line, "List of known proxies updated:") {
				log.WithFields(logger.Fields{
					"addr_web": proxy.webProxyAddr,
					"addr_ssh": proxy.sshProxyAddr,
					"addr_tun": proxy.reverseTunnelAddr,
				}).Debugf("Found all addrs of Proxy service process")
				proxy.setReady(true)
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
			proxy.saveStderr(data[:n])
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
		proxy.setErr(err)
		close(proxy.doneCh)
	}()

	<-proxy.doneCh

	if !proxy.IsReady() {
		log.Error("Proxy service is failed to initialize")
		stdoutLines := strings.Split(proxy.Stdout(), "\n")
		for _, line := range stdoutLines[len(stdoutLines)-10:] {
			log.Debug("Proxy service log: ", line)
		}
		log.Debugf("Proxy service stderr: %q", proxy.Stderr())

		// If it's still not ready lets signal that it's finally not ready.
		proxy.setReady(false)
		// Set an err just in case if it's not set before.
		proxy.setErr(trace.Errorf("failed to initialize"))
	}

	return trace.Wrap(proxy.Err())
}

// AuthAddr returns auth service external address.
func (proxy *ProxyService) AuthAddr() Addr {
	return proxy.WebProxyAddr()
}

// WebProxyAddr returns Web Proxy external address.
func (proxy *ProxyService) WebProxyAddr() Addr {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.webProxyAddr
}

// SSHProxyAddr returns SSH Proxy external address.
func (proxy *ProxyService) SSHProxyAddr() Addr {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.sshProxyAddr
}

// ReverseTunnelAddr returns reverse tunnel external address.
func (proxy *ProxyService) ReverseTunnelAddr() Addr {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.reverseTunnelAddr
}

// WebAndSSHProxyAddr returns string in a format "host:webport,sshport" needed as tsh --proxy option.
func (proxy *ProxyService) WebAndSSHProxyAddr() string {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return fmt.Sprintf("%s:%s,%s", proxy.webProxyAddr.Host, proxy.webProxyAddr.Port, proxy.sshProxyAddr.Port)
}

// Err returns proxy service error. It's nil If process is not done yet.
func (proxy *ProxyService) Err() error {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.error
}

// Shutdown terminates the proxy service process and waits for its completion.
func (proxy *ProxyService) Shutdown(ctx context.Context) error {
	proxy.doTerminate()
	select {
	case <-proxy.doneCh:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// Stdout returns a collected proxy service process stdout.
func (proxy *ProxyService) Stdout() string {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.stdout.String()
}

// Stderr returns a collected proxy service process stderr.
func (proxy *ProxyService) Stderr() string {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.stderr.String()
}

// WaitReady waits for proxy service initialization.
func (proxy *ProxyService) WaitReady(ctx context.Context) (bool, error) {
	select {
	case <-proxy.readyCh:
		return proxy.IsReady(), nil
	case <-ctx.Done():
		return false, trace.Wrap(ctx.Err(), "proxy service is not ready")
	}
}

// IsReady indicates if proxy service is initialized properly.
func (proxy *ProxyService) IsReady() bool {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	return proxy.isReady
}

func (proxy *ProxyService) doTerminate() {
	proxy.mu.Lock()
	terminate := proxy.terminate
	proxy.mu.Unlock()
	terminate()
}

func (proxy *ProxyService) parseLine(ctx context.Context, line string) {
	if submatch := regexpWebProxyStarting.FindStringSubmatch(line); submatch != nil {
		proxy.mu.Lock()
		defer proxy.mu.Unlock()
		proxy.webProxyAddr = Addr{Host: "localhost", Port: submatch[1]}
		return
	}

	if submatch := regexpSSHProxyStarting.FindStringSubmatch(line); submatch != nil {
		proxy.mu.Lock()
		defer proxy.mu.Unlock()
		proxy.sshProxyAddr = Addr{Host: "127.0.0.1", Port: submatch[1]}
		return
	}

	if submatch := regexpReverseTunnelStarting.FindStringSubmatch(line); submatch != nil {
		proxy.mu.Lock()
		defer proxy.mu.Unlock()
		proxy.reverseTunnelAddr = Addr{Host: "localhost", Port: submatch[1]}
		return
	}
}

func (proxy *ProxyService) saveStdout(line string) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	proxy.stdout.WriteString(line)
}

func (proxy *ProxyService) saveStderr(chunk []byte) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	proxy.stderr.Write(chunk)
}
