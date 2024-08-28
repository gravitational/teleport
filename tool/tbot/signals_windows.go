//go:build windows

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// handleSignals handles incoming signals.
func handleSignals(
	ctx context.Context,
	log *slog.Logger,
	cancel context.CancelFunc,
	reloadCh chan<- struct{},
) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP)

	for sig := range signals {
		switch sig {
		case syscall.SIGINT:
			log.InfoContext(ctx, "Received interrupt, triggering shutdown")
			cancel()
			return
		case syscall.SIGHUP:
			log.InfoContext(ctx, "Received reload signal, queueing reload")
			select {
			case reloadCh <- struct{}{}:
			default:
				log.WarnContext(ctx, "Unable to queue reload, reload already queued")
			}
		}
	}
}
