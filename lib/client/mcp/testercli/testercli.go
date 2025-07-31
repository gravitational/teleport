/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package testercli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

type Config struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Command     string
	Args        []string
	Interactive bool
}

type testerCLI struct {
	cfg      Config
	client   *mcpclient.Client
	info     slog.Logger
	tools    *mcp.ListToolsResult
	recorder *recorder
	scanner  *bufio.Scanner
	init     *mcp.InitializeResult
}

func (c *testerCLI) initialize(ctx context.Context) error {
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "teleport-mcp-tester",
				Version: teleport.Version,
			},
		},
	}
	initResponse, err := c.client.Initialize(ctx, initRequest)
	if err != nil {
		return trace.Wrap(err, "client initialization failed")
	}
	c.init = initResponse
	return nil
}

func Run(ctx context.Context, cfg Config) error {
	m := &model{
		cfg: cfg,
	}
	defer m.close()
	_, err := tea.NewProgram(
		m,
		tea.WithInput(cfg.Stdin),
		tea.WithOutput(cfg.Stdout),
		tea.WithAltScreen(),
		//tea.WithMouseCellMotion(),
	).Run()
	return trace.Wrap(err)
}

func run(ctx context.Context, cfg Config) (*testerCLI, error) {
	// TODO we must properly handle shutdown.
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return trace.Wrap(cmd.Process.Signal(syscall.SIGINT))
		}
		return nil
	}
	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	cmdStdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmdStderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	r := &recorder{
		enableColor: utils.IsTerminal(cfg.Stdout),
	}
	transport := transport.NewIO(
		r.makeStdout(cmdStdout),
		r.makeStdin(cmdStdin),
		r.makeStderr(cmdStderr),
	)
	go func() {
		io.Copy(io.Discard, transport.Stderr())
	}()
	transport.Start(ctx)
	client := mcpclient.NewClient(transport)
	cli := &testerCLI{
		cfg:      cfg,
		client:   client,
		recorder: r,
		scanner:  bufio.NewScanner(cfg.Stdin),
	}

	if err := cli.initialize(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return cli, nil
}
