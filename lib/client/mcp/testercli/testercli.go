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
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
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
}

func (c *testerCLI) Initialize(ctx context.Context) error {
	fmt.Fprintln(c.cfg.Stdout, "\n🏁 Sending \"initialize\" request:")
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

	fmt.Fprintln(c.cfg.Stdout, "✅ Success. Result:")
	fmt.Fprintf(c.cfg.Stdout, "Server name: %s\n", initResponse.ServerInfo.Name)
	fmt.Fprintf(c.cfg.Stdout, "Server version: %s\n", initResponse.ServerInfo.Version)
	return nil
}

func (c *testerCLI) ListTools(ctx context.Context) error {
	fmt.Fprintln(c.cfg.Stdout, "\n📋 Sending \"tools/list\" request:")
	tools, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return trace.Wrap(err, "list tools failed")
	}
	fmt.Fprintln(c.cfg.Stdout, "✅ Success. Result:")
	c.tools = tools

	var rows [][]string
	for _, tool := range tools.Tools {
		rows = append(rows, []string{
			tool.Name,
			tool.Description,
			strings.Join(slices.Collect(maps.Keys(tool.InputSchema.Properties)), ","),
		})
	}
	table := asciitable.MakeTableWithTruncatedColumn(
		[]string{"Tool Name", "Description", "Arguments"},
		rows,
		"Arguments",
	)
	fmt.Fprint(c.cfg.Stdout, table.String())
	return nil
}

func (c *testerCLI) CallTool(ctx context.Context) error {
	w := c.cfg.Stdout
	var tool mcp.Tool
	for {
		fmt.Fprintln(c.cfg.Stdout, "\n📋 tools/call on:")
		for i, tool := range c.tools.Tools {
			fmt.Fprintf(w, "%d. %s\n", i+1, tool.Name)
		}
		fmt.Fprint(w, "Select a tool number or 'b' to go back: ")
		if !c.scanner.Scan() {
			return nil
		}

		input := c.scanner.Text()
		if input == "b" {
			return nil
		}
		index, err := strconv.Atoi(input)
		if err != nil || index <= 0 || index >= len(c.tools.Tools)+1 {
			fmt.Fprintf(w, "❌ invalid input: %s\n", input)
			continue
		}

		tool = c.tools.Tools[index-1]
		break
	}

	fmt.Fprintf(w, "\n🗂️ %q tool selected. Now input values for args:\n", tool.Name)
	args := make(map[string]any)
	for name := range tool.InputSchema.Properties {
		optionalOrRequired := "optional"
		if slices.Contains(tool.InputSchema.Required, tool.Name) {
			optionalOrRequired = "required"
		}
		fmt.Fprintf(c.cfg.Stdout, "%s (%s): ", name, optionalOrRequired)
		if !c.scanner.Scan() {
			return nil
		}
		text := c.scanner.Text()
		if len(text) == 0 {
			continue
		}
		if v, err := strconv.Atoi(text); err == nil {
			args[name] = v
		} else if v, err := strconv.ParseFloat(text, 64); err == nil {
			args[name] = v
		} else if v, err := strconv.ParseBool(text); err == nil {
			args[name] = v
		} else {
			args[name] = text
		}
	}

	result, err := c.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      tool.Name,
			Arguments: args,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, content := range result.Content {
		switch t := content.(type) {
		case mcp.TextContent:
			fmt.Fprintln(w, "✅ Success. Result (text):")
			fmt.Fprintln(w, t.Text)
		default:
			fmt.Fprintln(w, "✅ Success. Result:")
			fmt.Fprintln(w, "%T", content)
		}
	}
	return nil
}

func (c *testerCLI) DumpProtocolLogs(ctx context.Context) error {
	fmt.Fprintln(c.cfg.Stdout, "\n📋 Dumping protocol logs:")
	c.recorder.dump(c.cfg.Stdout)
	return nil
}

func (c *testerCLI) StartInteractive(ctx context.Context) error {
	actions := []struct {
		desc string
		call func(ctx2 context.Context) error
	}{
		{
			desc: "tools/list",
			call: c.ListTools,
		},
		{
			desc: "tools/call",
			call: c.CallTool,
		},
		{
			desc: "dump protocol logs",
			call: c.DumpProtocolLogs,
		},
	}

	w := c.cfg.Stdout
	for {
		fmt.Fprintln(w, "\n🔢 Action menu:")
		for i, action := range actions {
			fmt.Fprintf(w, "%d. %s\n", i+1, action.desc)
		}
		fmt.Fprint(w, "Select an action number or 'q' to quit: ")
		if !c.scanner.Scan() {
			return nil
		}

		input := c.scanner.Text()
		if input == "q" {
			return nil
		}

		actionIndex, err := strconv.Atoi(input)
		if err != nil || actionIndex <= 0 || actionIndex >= len(actions)+1 {
			fmt.Fprintf(w, "❌ invalid input : %s\n", input)
			continue
		}

		action := actions[actionIndex-1]
		if err := action.call(ctx); err != nil {
			fmt.Fprintf(w, "❌ failed to %s: %v\n\n", action.desc, err)
		}
	}
}

func Run(ctx context.Context, cfg Config) error {
	fmt.Fprintf(cfg.Stdout, "🚀 Executing command:\n%v %v\n", cfg.Command, strings.Join(cfg.Args, " "))

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	fmt.Fprintln(cfg.Stdout, "✅ Success.")

	r := &recorder{}
	transport := transport.NewIO(
		r.makeStdout(stdout),
		r.makeStdin(stdin),
		r.makeStderr(stderr),
	)
	go func() {
		io.Copy(io.Discard, transport.Stderr())
	}()
	transport.Start(ctx)
	client := mcpclient.NewClient(transport)
	defer client.Close()
	cli := &testerCLI{
		cfg:      cfg,
		client:   client,
		recorder: r,
		scanner:  bufio.NewScanner(cfg.Stdin),
	}

	if err := cli.Initialize(ctx); err != nil {
		fmt.Fprintf(cfg.Stdout, "❌ %v\n", err)
		cli.DumpProtocolLogs(ctx)
		return trace.Wrap(err)
	}
	if err := cli.ListTools(ctx); err != nil {
		return trace.Wrap(err)
	}
	if !cfg.Interactive {
		return nil
	}
	return trace.Wrap(cli.StartInteractive(ctx))
}
