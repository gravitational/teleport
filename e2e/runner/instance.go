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
	"log/slog"
	"time"
)

type testInstance struct {
	browser            string
	log                *slog.Logger
	proxyPort          int
	authPort           int
	sshPort            int
	e2eDir             string
	dataDir            string
	teleportConfigPath string
	teleport           *teleportInstance
	node               *dockerNode
}

// start starts the Teleport instance and SSH node for this test instance.
// Called lazily from the playwright runner so that at most 2 instances run concurrently.
func (inst *testInstance) start(ctx context.Context) error {
	var err error

	defer func() {
		if err != nil {
			inst.stop()
		}
	}()

	if inst.teleport != nil {
		if err = inst.teleport.start(ctx); err != nil {
			return fmt.Errorf("failed to start Teleport for %s: %w", inst.browser, err)
		}
		if err = inst.teleport.waitReady(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("teleport for %s failed to become ready: %w", inst.browser, err)
		}
		if err = seedRecordings(ctx, inst.e2eDir, inst.dataDir); err != nil {
			return fmt.Errorf("failed to seed session recordings for %s: %w", inst.browser, err)
		}
	}

	if inst.node != nil {
		if err = inst.node.start(ctx); err != nil {
			return fmt.Errorf("failed to start docker node for %s: %w", inst.browser, err)
		}
		if err = inst.node.waitJoined(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("docker node for %s failed to join cluster: %w", inst.browser, err)
		}
	}

	return nil
}

func (inst *testInstance) stop() {
	if inst.node != nil {
		inst.node.stop(context.Background())
	}

	if inst.teleport != nil {
		inst.teleport.stop()
	}
}

var browserColors = map[string]string{
	"chromium": "\033[36m", // cyan
	"firefox":  "\033[33m", // yellow
	"webkit":   "\033[35m", // magenta
	"connect":  "\033[32m", // green
}

type prefixHandler struct {
	slog.Handler
	browser string
}

func (h *prefixHandler) Handle(ctx context.Context, r slog.Record) error {
	color, ok := browserColors[h.browser]
	if !ok {
		color = "\033[37m"
	}

	r.Message = fmt.Sprintf("%s[%s]\033[0m %s", color, h.browser, r.Message)

	return h.Handler.Handle(ctx, r)
}

func newBrowserLogger(browser string) *slog.Logger {
	return slog.New(&prefixHandler{
		Handler: slog.Default().Handler(),
		browser: browser,
	})
}
