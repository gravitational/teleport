// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot"
	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
	"github.com/gravitational/teleport/tool/common"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "workload")

func main() {
	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(context.Background())
	defer cancel()

	if err := Run(ctx, os.Args[1:]); err != nil {
		if exitError, ok := errors.AsType[*common.ExitCodeError](err); ok {
			os.Exit(exitError.Code)
		}
		utils.FatalError(err)
	}
}

func Run(ctx context.Context, args []string) error {
	app := kingpin.New("init", "Antithesis workload init.").Interspersed(true)
	var configPaths []string
	app.Flag("config", "Path to a tbot configuration file. May be specified multiple times.").Short('c').Required().StringsVar(&configPaths)

	if _, err := app.Parse(args); err != nil {
		app.Usage(args)
		return trace.Wrap(err, "parsing args")
	}

	readyC := make(chan error, len(configPaths))

	group, ctx := errgroup.WithContext(ctx)
	for _, path := range configPaths {
		cfg, err := tbotconfig.ReadConfigFromFile(path, false)
		if err != nil {
			return trace.Wrap(err, "loading bot config from path %s", path)
		}
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating bot config from path %s", path)
		}
		b := tbot.New(cfg, log.With(
			teleport.ComponentLabel, "tbot",
			"config_path", path,
			"diag_addr", cfg.DiagAddr,
		))

		group.Go(func() error {
			return b.Run(ctx)
		})

		group.Go(func() error {
			select {
			case readyC <- waitForReady(ctx, cfg.DiagAddr):
				return nil
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		})

	}

	group.Go(func() error {
		for range configPaths {
			select {
			case err := <-readyC:
				if err != nil {
					return trace.Wrap(err)
				}
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}
		slog.InfoContext(ctx, "all bot instances reported ready, issue setup_complete")
		lifecycle.SetupComplete(map[string]any{})
		return nil
	})

	return group.Wait()
}

func waitForReady(ctx context.Context, diagAddr string) error {
	if diagAddr == "" {
		return trace.BadParameter("--diag-addr is required")
	}

	// Allow plain host:port syntax; url.Parse will fail without a scheme, so if
	// none is specified, prepend http://
	if !strings.Contains(diagAddr, "://") {
		diagAddr = "http://" + diagAddr
	}

	baseURL, err := url.Parse(diagAddr)
	if err != nil {
		return trace.Wrap(err, "parsing --diag-addr")
	}

	endpoint := baseURL.JoinPath("wait")

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(100 * time.Millisecond),
		Jitter: retryutils.HalfJitter,
		First:  250 * time.Millisecond,
		Max:    2 * time.Second,
	})
	if err != nil {
		return trace.Wrap(err, "creating retry helper")
	}

	client, err := defaults.HTTPClient()
	if err != nil {
		return trace.Wrap(err, "creating http client")
	}

	log.InfoContext(ctx, "waiting for bot to become available")

	now := time.Now()

	for i := 1; ; i++ {
		l := log.With("attempt", i)
		err = waitFetch(ctx, client, endpoint)
		if err == nil {
			break
		} else {
			l.DebugContext(ctx, "wait failed, retrying", "error", err)
		}

		retry.Inc()
		select {
		case <-ctx.Done():
			l.WarnContext(ctx, "context canceled before bot became ready", "last_error", err)
			return ctx.Err()
		case <-retry.After():
		}
	}

	log.InfoContext(ctx, "bot reported healthy", "after", time.Since(now))
	return nil
}

// waitFetch fetches a status report from the given endpoint using the provided
// client. It only returns without an error if the endpoint returns a valid
// and healthy status report.
func waitFetch(ctx context.Context, client *http.Client, endpoint *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return trace.Wrap(err, "building wait request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err, "making http request")
	}
	defer resp.Body.Close()

	bytes, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the status doesn't appear to be OK, try to parse out service status if
	// possible. 404 is ignored because the JSON structure is different.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		var status readyz.OverallStatus
		if err := json.Unmarshal(bytes, &status); err == nil {
			log.WarnContext(ctx, "bot is not yet ready", "status", status.Status)
			return trace.ConnectionProblem(nil, "bot is not yet ready: %s", status.Status)
		}

		// Note: `trace.ReadError()` doesn't provide any useful info for 5xx
		// errors, so return something sane if it failed to parse above.
		return trace.ConnectionProblem(nil, "unexpected response from server: %s", string(bytes))
	}

	// Given the above check, `trace.ReadError` just handles 404s.
	return trace.Wrap(trace.ReadError(resp.StatusCode, bytes), "response from wait API")
}
