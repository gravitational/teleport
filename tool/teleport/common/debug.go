// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// DebugClient specifies the debug service client.
type DebugClient interface {
	// SetLogLevel changes the application's log level and a change status message.
	SetLogLevel(context.Context, string) (string, error)
	// GetLogLevel fetches the current log level.
	GetLogLevel(context.Context) (string, error)
	// CollectProfile collects a pprof profile.
	CollectProfile(context.Context, string, int) ([]byte, error)
	// GetReadiness checks if the instance is ready to serve requests.
	GetReadiness(context.Context) (debugclient.Readiness, error)
	// GetRawMetrics fetches the unprocessed Prometheus metrics.
	GetRawMetrics(context.Context) (io.ReadCloser, error)
	// GetProcessInfo fetches the teleport process info.
	GetProcessInfo(context.Context) (debugclient.ProcessInfo, error)
	SocketPath() string
}

func onSetLogLevel(configPath string, level string) error {
	ctx := context.Background()
	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	setMessage, err := setLogLevel(ctx, clt, level)
	if err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}

	fmt.Println(setMessage)
	return nil
}

func setLogLevel(ctx context.Context, clt DebugClient, level string) (string, error) {
	if contains := slices.Contains(logutils.SupportedLevelsText, strings.ToUpper(level)); !contains {
		return "", trace.BadParameter("%q log level not supported", level)
	}

	return clt.SetLogLevel(ctx, level)
}

func onGetLogLevel(configPath string) error {
	ctx := context.Background()
	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	currentLogLevel, err := getLogLevel(ctx, clt)
	if err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}

	fmt.Printf("Current log level %q\n", currentLogLevel)
	return nil
}

func getLogLevel(ctx context.Context, clt DebugClient) (string, error) {
	return clt.GetLogLevel(ctx)
}

// defaultCollectProfileSeconds defines the default collect profiles seconds
// value.
const defaultCollectProfileSeconds = 10

// profilesWithoutSnapshotSupport list of profiles that DO NOT support taking
// snapshots (seconds = 0).
var profilesWithoutSnapshotSupport = map[string]struct{}{
	"profile": {},
	"trace":   {},
}

// defaultCollectProfiles defines the default profiles to be collected in case
// none is provided.
var defaultCollectProfiles = []string{"goroutine", "heap", "profile"}

func onCollectProfiles(configPath string, rawProfiles string, seconds int) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var output bytes.Buffer
	if err := collectProfiles(ctx, clt, &output, rawProfiles, seconds); err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}

	fmt.Print(output.String())
	return nil
}

// collectProfiles collects the profiles and generate a compressed tarball
// file.
func collectProfiles(ctx context.Context, clt DebugClient, buf io.Writer, rawProfiles string, seconds int) error {
	profiles := defaultCollectProfiles
	if rawProfiles != "" {
		profiles = slices.Compact(strings.Split(rawProfiles, ","))
	}

	fileTime := time.Now()
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for _, profile := range profiles {
		profileSeconds := seconds
		// When the profile doesn't support snapshot, we set a default seconds
		// value to avoid blocking users when they consume those profiles.
		if _, ok := profilesWithoutSnapshotSupport[profile]; seconds == 0 && ok {
			profileSeconds = 10
		}

		contents, err := clt.CollectProfile(ctx, profile, profileSeconds)
		if err != nil {
			return trace.Wrap(err)
		}

		hd := &tar.Header{
			Name:    profile + ".pprof",
			Size:    int64(len(contents)),
			Mode:    0600,
			ModTime: fileTime,
		}
		if err := tw.WriteHeader(hd); err != nil {
			return trace.Wrap(err)
		}
		if _, err := tw.Write(contents); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.NewAggregate(tw.Close(), gw.Close())
}

// onReadyz checks if the instance is ready to serve requests.
func onReadyz(ctx context.Context, configPath string) error {
	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := readyz(ctx, clt); err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}

	return nil
}

func readyz(ctx context.Context, clt DebugClient) error {
	readiness, err := clt.GetReadiness(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if !readiness.Ready {
		return trace.Errorf("not ready (PID:%d): %s", readiness.PID, readiness.Status)
	}

	fmt.Printf("ready (PID:%d)\n", readiness.PID)
	return nil
}

// onMetrics fetches the current Prometheus metrics.
func onMetrics(ctx context.Context, configPath string) error {
	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	metrics, err := clt.GetRawMetrics(ctx)
	if err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}
	defer metrics.Close()

	if _, err := io.Copy(os.Stdout, metrics); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// onProcessInfo prints Teleport process info for debugging.
func onProcessInfo(ctx context.Context, configPath string, top bool, showConfig bool, serviceFilter string) error {
	clt, dataDir, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := processInfoOutputOptions{
		showConfig:    showConfig,
		serviceFilter: strings.TrimSpace(serviceFilter),
	}

	if top {
		return runTopProcessInfo(ctx, clt, dataDir, opts)
	}

	info, err := clt.GetProcessInfo(ctx)
	if err != nil {
		return convertToReadableErr(err, dataDir, clt.SocketPath())
	}

	// TODO: support text, json, binary, tar, etc. formats
	return printProcessInfo(info, opts)
}

type processInfoOutputOptions struct {
	showConfig    bool
	serviceFilter string
}

type serviceGroup struct {
	name         string
	services     []string
	hasConfig    bool
	critical     bool
	runningSince time.Time
	config       string
	errors       map[string]string
}

func printProcessInfo(info debugclient.ProcessInfo, opts processInfoOutputOptions) error {
	groups := buildServiceGroups(info, opts)
	if opts.serviceFilter != "" && len(groups) == 0 {
		return trace.NotFound("service %q not found in process info", opts.serviceFilter)
	}

	fmt.Printf("PID: %d\n", info.PID)
	fmt.Printf("Collected At: %s\n", formatTopTime(info.CollectedAt))
	fmt.Printf("Overall State: %s\n", info.OverallState)

	fmt.Println("")
	fmt.Println("Health Signals")
	printSignal("control_plane_connectivity", info.Signals.ControlPlaneConnectivity)
	printSignal("watcher_cache_lag", info.Signals.WatcherCacheLag)
	printSignal("metric_digest", info.Signals.MetricDigest)
	printSignal("degraded_state_registry", info.Signals.DegradedStateRegistry)
	printSignal("backend_lock_contention", info.Signals.BackendLockContention)
	printSignal("rotation_ca_status", info.Signals.RotationCAStatus)
	printSignal("startup_ready_durations", info.Signals.StartupReadyDurations)

	if len(info.HeartbeatTimeline) > 0 {
		fmt.Println("")
		fmt.Println("Heartbeat Timeline")
		components := make([]string, 0, len(info.HeartbeatTimeline))
		for component := range info.HeartbeatTimeline {
			components = append(components, component)
		}
		slices.Sort(components)
		for _, component := range components {
			hb := info.HeartbeatTimeline[component]
			fmt.Printf("- %s: state=%s consecutive_errors=%d last_event=%s last_error=%s last_ok=%s\n",
				component, hb.State, hb.ConsecutiveHeartbeatErrors, hb.LastEvent,
				formatTopTime(hb.LastHeartbeatError), formatTopTime(hb.LastHeartbeatOK))
		}
	}

	fmt.Println("")
	fmt.Println("Services")
	for _, group := range groups {
		fmt.Printf("- %s: has config: %v critical: %v running_since: %s subservices: %d\n",
			group.name, group.hasConfig, group.critical, formatTopTime(group.runningSince), len(group.services))

		if len(group.services) > 1 {
			for _, serviceName := range group.services {
				fmt.Printf("  - %s\n", serviceName)
			}
		}

		if len(group.errors) > 0 {
			errorKeys := make([]string, 0, len(group.errors))
			for serviceName := range group.errors {
				errorKeys = append(errorKeys, serviceName)
			}
			slices.Sort(errorKeys)
			for _, serviceName := range errorKeys {
				fmt.Printf("  error[%s]=%s\n", serviceName, group.errors[serviceName])
			}
		}

		if opts.showConfig && group.config != "" {
			fmt.Printf("  config:\n%s\n", group.config)
		}
	}
	return nil
}

func buildServiceGroups(info debugclient.ProcessInfo, opts processInfoOutputOptions) []serviceGroup {
	useExactMatch := strings.Contains(opts.serviceFilter, ".")
	groupByName := make(map[string]*serviceGroup)

	serviceNames := make([]string, 0, len(info.ServiceDebugInfo))
	for serviceName := range info.ServiceDebugInfo {
		serviceNames = append(serviceNames, serviceName)
	}
	slices.Sort(serviceNames)

	for _, serviceName := range serviceNames {
		serviceInfo := info.ServiceDebugInfo[serviceName]
		root := topLevelServiceName(serviceName)

		if !matchesServiceFilter(opts.serviceFilter, useExactMatch, root, serviceName) {
			continue
		}

		groupKey := root
		if useExactMatch {
			groupKey = serviceName
		}

		group, ok := groupByName[groupKey]
		if !ok {
			group = &serviceGroup{
				name:   groupKey,
				errors: make(map[string]string),
			}
			groupByName[groupKey] = group
		}

		group.services = append(group.services, serviceName)
		if serviceInfo.HasInfo {
			group.hasConfig = true
		}
		if serviceInfo.IsCritical {
			group.critical = true
		}
		if !serviceInfo.RunningSince.IsZero() && (group.runningSince.IsZero() || serviceInfo.RunningSince.Before(group.runningSince)) {
			group.runningSince = serviceInfo.RunningSince
		}
		if serviceInfo.Error != "" {
			group.errors[serviceName] = serviceInfo.Error
		}
		if group.config == "" && serviceInfo.ServiceConfig != "" {
			group.config = serviceInfo.ServiceConfig
		}
	}

	groups := make([]serviceGroup, 0, len(groupByName))
	for _, group := range groupByName {
		slices.Sort(group.services)
		groups = append(groups, *group)
	}
	slices.SortFunc(groups, func(a, b serviceGroup) int {
		return strings.Compare(a.name, b.name)
	})
	return groups
}

func matchesServiceFilter(filter string, useExactMatch bool, rootService, fullService string) bool {
	if filter == "" {
		return true
	}
	if useExactMatch {
		return fullService == filter
	}
	return rootService == filter || fullService == filter
}

func topLevelServiceName(serviceName string) string {
	root, _, hasChild := strings.Cut(serviceName, ".")
	if hasChild {
		return root
	}
	return serviceName
}

func runTopProcessInfo(ctx context.Context, clt DebugClient, dataDir string, opts processInfoOutputOptions) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		info, err := clt.GetProcessInfo(ctx)
		if err != nil {
			return convertToReadableErr(err, dataDir, clt.SocketPath())
		}
		renderTopProcessInfo(info, opts)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func renderTopProcessInfo(info debugclient.ProcessInfo, opts processInfoOutputOptions) {
	groups := buildServiceGroups(info, opts)
	// ANSI clear screen + move cursor to top-left.
	fmt.Print("\033[2J\033[H")
	fmt.Printf("teleport debug process --top | pid=%d | state=%s | collected=%s\n",
		info.PID, info.OverallState, formatTopTime(info.CollectedAt))
	fmt.Println("Press Ctrl+C to exit.")
	fmt.Println("")

	printSignal("control_plane_connectivity", info.Signals.ControlPlaneConnectivity)
	printSignal("watcher_cache_lag", info.Signals.WatcherCacheLag)
	printSignal("metric_digest", info.Signals.MetricDigest)
	printSignal("degraded_state_registry", info.Signals.DegradedStateRegistry)
	printSignal("backend_lock_contention", info.Signals.BackendLockContention)
	printSignal("rotation_ca_status", info.Signals.RotationCAStatus)
	printSignal("startup_ready_durations", info.Signals.StartupReadyDurations)

	fmt.Println("")
	fmt.Println("Top Services")
	for _, group := range groups {
		fmt.Printf("- %-24s critical=%-5v has_config=%-5v running_since=%s subservices=%d\n",
			group.name, group.critical, group.hasConfig, formatTopTime(group.runningSince), len(group.services))
		if len(group.errors) > 0 {
			fmt.Printf("  errors=%d\n", len(group.errors))
		}
	}
}

func printSignal(name string, signal debugclient.Signal) {
	fmt.Printf("- %s: status=%s summary=%s\n", name, signal.Status, signal.Summary)
	if len(signal.Details) == 0 {
		return
	}
	keys := make([]string, 0, len(signal.Details))
	for key := range signal.Details {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Printf("  %s=%s\n", key, signal.Details[key])
	}
}

func formatTopTime(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Format(time.RFC3339)
}

// newDebugClient initializes the debug client based on the Teleport
// configuration. It also returns the data dir and socket path used.
func newDebugClient(configPath string) (DebugClient, string, error) {
	cfg, err := config.ReadConfigFile(configPath)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// ReadConfigFile returns nil configuration if the file doesn't exists.
	// In that case, fallback to default data dir path. The data directory
	// is not required so should use the default if not specified.
	dataDir := defaults.DataDir
	if cfg != nil && cfg.DataDir != "" {
		dataDir = cfg.DataDir
	}

	return debugclient.NewClient(dataDir), dataDir, nil
}

// convertToReadableErr converts debug service client error into a more friendly
// messages.
func convertToReadableErr(err error, dataDir, socketPath string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return trace.Errorf("request canceled")
	case trace.IsConnectionProblem(err):
		return trace.BadParameter("Unable to reach debug service socket at %q."+
			"\n\nVerify if you have enough permissions to open the socket and if the path"+
			" to your data directory (%q) is correct. The command assumes the data"+
			" directory from your configuration file, you can provide the path to it using the --config flag.", socketPath, dataDir)
	default:
		return err
	}
}
