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

package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	waitNoResolveDefaultPeriod  = "10s"
	waitNoResolveDefaultTimeout = "10m"
)

type waitFlags struct {
	duration time.Duration
	domain   string
	period   time.Duration
	timeout  time.Duration
}

func onWaitDuration(flags waitFlags) error {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	defer cancel()

	return trace.Wrap(waitDuration(ctx, flags.duration))
}

func onWaitNoResolve(flags waitFlags) error {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	defer cancel()

	return trace.Wrap(waitNoResolve(ctx, flags.domain, flags.period, flags.timeout))
}

func waitDuration(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return trace.BadParameter("no duration provided")
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	<-timeoutCtx.Done()

	err := timeoutCtx.Err()
	if !errors.Is(err, context.DeadlineExceeded) {
		return trace.Wrap(err)
	}
	return nil
}

func waitNoResolve(ctx context.Context, domain string, period, timeout time.Duration) error {
	if domain == "" {
		return trace.BadParameter("no domain provided")
	}

	if period == 0 {
		return trace.BadParameter("no period provided")
	}

	if timeout == 0 {
		return trace.BadParameter("no timeout provided")
	}
	log := slog.With("domain", domain)
	log.InfoContext(ctx, "waiting until the domain stops resolving to ensure that every auth server running the previous major version has been updated/terminated")

	var err error
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// We resolve the previous auth service until there's no IP returned.
	// This means all pods got rollout, and we don't risk connecting to
	// an auth pod running the previous version
	periodic := interval.New(interval.Config{
		Duration:      period,
		FirstDuration: time.Millisecond,
		Jitter:        retryutils.SeventhJitter,
	})
	defer periodic.Stop()

	exit := false
	for !exit {
		select {
		case <-ctx.Done():
			// Context has been canceled, either we reached the timeout
			// or something else happened to the parent context
			err = ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				return trace.LimitExceeded(
					"timeout (%s) reached, but domain '%s' is still resolving",
					timeout,
					domain,
				)
			}
			return trace.Wrap(err)

		case <-periodic.Next():
			exit, err = checkDomainNoResolve(ctx, domain, log)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	log.InfoContext(ctx, "no endpoints found, exiting with success code")
	return nil
}

func checkDomainNoResolve(ctx context.Context, domainName string, log *slog.Logger) (exit bool, err error) {
	endpoints, err := resolveEndpoints(domainName)
	if err != nil {
		var dnsErr *net.DNSError
		if !errors.As(trace.Unwrap(err), &dnsErr) {
			log.ErrorContext(ctx, "unexpected error when resolving domain", "error", err)
			return false, trace.Wrap(err)
		}

		if dnsErr.IsNotFound {
			log.InfoContext(ctx, "domain not found")
			return true, nil
		}

		// Creating a new logger because the linter doesn't want both key/value and slog.Attr in the same log write.
		log := log.With(slog.Group("dns_error",
			"name", dnsErr.Name,
			"server", dnsErr.Server,
			"is_timeout", dnsErr.IsTimeout,
			"is_temporary", dnsErr.IsTemporary,
			"is_not_found", dnsErr.IsNotFound,
			// Logging the error type can help understanding where the error comes from
			"wrapped_error_type", fmt.Sprintf("%T", dnsErr.Unwrap()),
		))
		if dnsErr.Temporary() {
			log.WarnContext(ctx, "temporary error when resolving domain", "error", err)
			return false, nil
		}
		log.ErrorContext(ctx, "error when resolving domain", "error", err)
		return false, nil
	}
	if len(endpoints) == 0 {
		log.InfoContext(ctx, "domain found and resolution returned no endpoints")
		return true, nil
	}
	log.InfoContext(ctx, "endpoints found when resolving domain", "endpoints", endpoints)
	return false, nil
}

func resolveEndpoints(serviceName string) ([]net.IP, error) {
	ips, err := net.LookupIP(serviceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ips, nil
}
