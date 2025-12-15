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

package lib

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Terminable interface {
	// Shutdown attempts to gracefully terminate.
	Shutdown(context.Context) error
	// Close does a fast (force) termination.
	Close()
}

func ServeSignals(app Terminable, shutdownTimeout time.Duration) {
	ctx := context.Background()
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC,
		syscall.SIGTERM, // graceful shutdown
		syscall.SIGINT,  // graceful-then-fast shutdown
		syscall.SIGUSR1, // capture pprof profiles
	)
	defer signal.Stop(sigC)

	gracefulShutdown := func() {
		tctx, tcancel := context.WithTimeout(ctx, shutdownTimeout)
		defer tcancel()
		slog.InfoContext(tctx, "Attempting graceful shutdown")
		if err := app.Shutdown(tctx); err != nil {
			slog.InfoContext(tctx, "Graceful shutdown failed, attempting fast shutdown")
			app.Close()
		}
	}
	var alreadyInterrupted bool
	for {
		signal := <-sigC
		switch signal {
		case syscall.SIGTERM:
			gracefulShutdown()
			return
		case syscall.SIGINT:
			if alreadyInterrupted {
				app.Close()
				return
			}
			go gracefulShutdown()
			alreadyInterrupted = true
		case syscall.SIGUSR1:
			if p, ok := app.(interface{ Profile() }); ok {
				go p.Profile()
			}
		}
	}
}
