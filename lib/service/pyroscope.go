// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/grafana/pyroscope-go"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TODO: Replace logger when pyroscope uses slog
type pyroscopeLogger struct {
	l *slog.Logger
}

type roundTripper struct {
	tripper http.RoundTripper
	timeout time.Duration
	logger  *slog.Logger
}

// CloseIdleConnections ensures idle connections of the wrapped
// [http.RoundTripper] are closed.
func (rt roundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}

	if tr, ok := rt.tripper.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

func (l pyroscopeLogger) Infof(format string, args ...any) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	//nolint:sloglint // msg cannot be constant
	l.l.Info(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Debugf(format string, args ...any) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	//nolint:sloglint // msg cannot be constant
	l.l.Debug(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Errorf(format string, args ...any) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelError) {
		return
	}

	//nolint:sloglint // msg cannot be constant
	l.l.Error(fmt.Sprintf(format, args...))
}

func (rt roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := rt.tripper.RoundTrip(req)
	duration := time.Since(start)

	threshold := rt.timeout * 90 / 100
	if duration > threshold {
		rt.logger.DebugContext(req.Context(), "Pyroscope upload exceeded threshold", "upload_duration", duration, "upload_threshold", threshold, "upload_url", logutils.StringerAttr(req.URL))
	}

	return resp, err
}

// createPyroscopeConfig generates the Pyroscope configuration for the Teleport process.
func createPyroscopeConfig(ctx context.Context, logger *slog.Logger, address string) (pyroscope.Config, error) {
	if address == "" {
		return pyroscope.Config{}, trace.BadParameter("pyroscope address is empty")
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	const httpTimeout = 60 * time.Second

	httpClient := &http.Client{
		Timeout: httpTimeout,
		Transport: roundTripper{
			tripper: http.DefaultTransport,
			timeout: httpTimeout,
			logger:  logger,
		},
	}

	config := pyroscope.Config{
		ApplicationName: teleport.ComponentTeleport,
		ServerAddress:   address,
		Logger:          pyroscope.Logger(pyroscopeLogger{l: logger}),
		Tags: map[string]string{
			"host":    hostname,
			"version": teleport.Version,
			"git_ref": teleport.Gitref,
		},
		HTTPClient: httpClient,
		UploadRate: 60 * time.Second,
	}

	// Evaluate if profile configuration is customized
	if p := getPyroscopeProfileTypesFromEnv(); len(p) == 0 {
		logger.InfoContext(ctx, "No profile types enabled, using default")
	} else {
		config.ProfileTypes = p
		logger.InfoContext(ctx, "Pyroscope will configure profiles from env")
	}

	if rate := os.Getenv("TELEPORT_PYROSCOPE_UPLOAD_RATE"); rate != "" {
		parsedRate, err := time.ParseDuration(rate)
		if err != nil {
			logger.InfoContext(ctx, "invalid TELEPORT_PYROSCOPE_UPLOAD_RATE, ignoring value", "provided_value", rate, "error", err)
		} else {
			logger.InfoContext(ctx, "TELEPORT_PYROSCOPE_UPLOAD_RATE configured", "rate", parsedRate)
			config.UploadRate = parsedRate
		}
	}

	if value, isSet := os.LookupEnv("TELEPORT_PYROSCOPE_KUBE_COMPONENT"); isSet {
		config.Tags["component"] = value
	}

	if value, isSet := os.LookupEnv("TELEPORT_PYROSCOPE_KUBE_NAMESPACE"); isSet {
		config.Tags["namespace"] = value
	}

	return config, nil
}

// initPyroscope instruments Teleport to run with continuous profiling for Pyroscope
func (process *TeleportProcess) initPyroscope(address string) {
	logger := process.logger.With(teleport.ComponentKey, "pyroscope")
	config, err := createPyroscopeConfig(process.ExitContext(), logger, address)
	if err != nil {
		logger.ErrorContext(process.ExitContext(), "failed to create Pyroscope config", "address", address, "error", err)
		return
	}

	profiler, err := pyroscope.Start(config)
	if err != nil {
		logger.ErrorContext(process.ExitContext(), "error starting pyroscope profiler", "address", address, "error", err)
	} else {
		logger.InfoContext(process.ExitContext(), "Pyroscope has successfully started")
		process.OnExit("pyroscope.profiler", func(payload any) {
			// Observed rare and inconsistent panics, short term solution is to not wait for flush
			profiler.Flush(false)
			_ = profiler.Stop()
		})
	}
}

// getPyroscopeProfileTypesFromEnv sets the profile types based on environment variables.
func getPyroscopeProfileTypesFromEnv() []pyroscope.ProfileType {
	var profileTypes []pyroscope.ProfileType

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_MEMORY_ENABLED") == "true" {
		profileTypes = append(profileTypes,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
		)
	}

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_CPU_ENABLED") == "true" {
		profileTypes = append(profileTypes, pyroscope.ProfileCPU)
	}

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_GOROUTINES_ENABLED") == "true" {
		profileTypes = append(profileTypes, pyroscope.ProfileGoroutines)
	}

	return profileTypes
}
