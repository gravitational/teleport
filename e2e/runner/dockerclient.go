/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	sdkclient "github.com/docker/go-sdk/client"
	dockercontext "github.com/docker/go-sdk/context"
	"github.com/moby/moby/client"
)

// dockerHostURL is the parsed DOCKER_HOST, or nil when talking to a local daemon.
var dockerHostURL = sync.OnceValues(func() (*url.URL, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		return nil, nil
	}

	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parsing DOCKER_HOST %q: %w", host, err)
	}

	return u, nil
})

// dockerAPI is the moby client for the daemon named by DOCKER_HOST. The ssh:// scheme is handled
// here because neither the moby client nor go-sdk implements it.
var dockerAPI = sync.OnceValues(func() (*client.Client, error) {
	u, err := dockerHostURL()
	if err != nil {
		return nil, err
	}

	if u == nil || u.Scheme != "ssh" {
		// CurrentDockerHost prefers a rootless socket over DOCKER_HOST, so it is only consulted when
		// DOCKER_HOST is unset, to keep docker's own precedence.
		host := ""
		if u != nil {
			host = u.String()
		} else {
			resolved, err := dockercontext.CurrentDockerHost()
			if err != nil {
				return nil, fmt.Errorf("resolving docker host: %w", err)
			}

			host = resolved
		}

		cli, err := client.New(client.FromEnv, client.WithHost(host))
		if err != nil {
			return nil, fmt.Errorf("creating docker client for %q: %w", host, err)
		}

		return cli, nil
	}

	cli, err := client.New(
		client.WithHost("http://docker.example.com"),
		client.WithDialContext(sshDialContext(u)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client over ssh: %w", err)
	}

	return cli, nil
})

// dockerSDK wraps the moby client so go-sdk's container helpers use the same transport.
var dockerSDK = sync.OnceValues(func() (sdkclient.SDKClient, error) {
	api, err := dockerAPI()
	if err != nil {
		return nil, err
	}

	cli, err := sdkclient.New(context.Background(), sdkclient.WithDockerAPI(api))
	if err != nil {
		return nil, fmt.Errorf("creating docker sdk client: %w", err)
	}

	return cli, nil
})

// sshDialContext returns a dialer that reaches the remote daemon by running the docker CLI there,
// which is how the docker CLI itself implements ssh:// transport.
func sshDialContext(u *url.URL) func(ctx context.Context, network, addr string) (net.Conn, error) {
	// -T because the stream carries the docker API, and an ssh config forcing a PTY would corrupt it.
	args := []string{"-T", "-o", "ConnectTimeout=30"}
	if u.User != nil {
		args = append(args, "-l", u.User.Username())
	}
	if port := u.Port(); port != "" {
		args = append(args, "-p", port)
	}
	// The terminator goes before the destination, as the docker CLI does it, so that it does not depend
	// on ssh consuming one that follows the destination.
	args = append(args, "--", u.Hostname(), "docker", "system", "dial-stdio")

	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		cmd := exec.CommandContext(ctx, "ssh", args...)
		cmd.Stderr = os.Stderr

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("ssh stdin: %w", err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("ssh stdout: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("starting ssh: %w", err)
		}

		return &commandConn{cmd: cmd, reader: stdout, writer: stdin}, nil
	}
}

type commandConn struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
	writer io.WriteCloser
}

func (c *commandConn) Read(b []byte) (int, error)  { return c.reader.Read(b) }
func (c *commandConn) Write(b []byte) (int, error) { return c.writer.Write(b) }

func (c *commandConn) Close() error {
	_ = c.writer.Close()
	_ = c.reader.Close()

	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	_ = c.cmd.Wait()

	return nil
}

func (c *commandConn) LocalAddr() net.Addr                { return commandAddr{} }
func (c *commandConn) RemoteAddr() net.Addr               { return commandAddr{} }
func (c *commandConn) SetDeadline(_ time.Time) error      { return nil }
func (c *commandConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *commandConn) SetWriteDeadline(_ time.Time) error { return nil }

type commandAddr struct{}

func (commandAddr) Network() string { return "ssh" }
func (commandAddr) String() string  { return "ssh" }

// nodePublicHost is the address the node advertises for its SSH port. The container publishes that port
// on whichever machine runs the daemon, so with a remote DOCKER_HOST it is not the machine running the
// tests, and the proxy would otherwise dial itself.
func nodePublicHost() (string, error) {
	u, err := dockerHostURL()
	if err != nil {
		return "", err
	}

	return publicHostFor(u)
}

func publicHostFor(u *url.URL) (string, error) {
	if u == nil {
		return "localhost", nil
	}

	switch u.Scheme {
	case "ssh":
		target, err := sshRouteTarget(u)
		if err != nil {
			return "", err
		}

		host, _, err := net.SplitHostPort(target)
		if err != nil {
			return "", fmt.Errorf("splitting ssh route target %q: %w", target, err)
		}

		return host, nil
	case "tcp", "http", "https":
		return u.Hostname(), nil
	default:
		return "localhost", nil
	}
}

// sshRouteTarget resolves the real host and port behind a DOCKER_HOST ssh URL by asking ssh to apply its own config
func sshRouteTarget(u *url.URL) (string, error) {
	out, err := exec.Command("ssh", "-G", u.Hostname()).Output()
	if err != nil {
		return "", fmt.Errorf("resolving ssh host %q: %w", u.Hostname(), err)
	}

	hostname, port := parseSSHConfig(out)
	if hostname == "" {
		return "", fmt.Errorf("ssh did not report a hostname for %q", u.Hostname())
	}

	// An explicit port in DOCKER_HOST wins over the one ssh config supplies.
	if p := u.Port(); p != "" {
		port = p
	}

	return net.JoinHostPort(hostname, port), nil
}

func parseSSHConfig(out []byte) (hostname, port string) {
	port = "22"

	for line := range strings.Lines(string(out)) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}

		switch key {
		case "hostname":
			hostname = value
		case "port":
			port = value
		}
	}

	return hostname, port
}
