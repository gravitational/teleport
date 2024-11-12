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

package authorizedkeys

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

var (
	// ErrUnsupportedPlatform is returned when the operating system is not supported.
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

// Watcher watches for changes to authorized_keys files
// and reports them to the cluster. If the cluster does not have
// scanning enabled, the watcher will hold until the feature is enabled.
type Watcher struct {
	// client is the client to use to communicate with the cluster.
	client       ClusterClient
	logger       *slog.Logger
	clock        clockwork.Clock
	hostID       string
	getHostUsers func() ([]user.User, error)
	// keyNames is the list of key names that have been reported to the cluster.
	keyNames []string
}

// ClusterClient is the client to use to communicate with the cluster.
type ClusterClient interface {
	GetClusterAccessGraphConfig(context.Context) (*clusterconfigpb.AccessGraphConfig, error)
	AccessGraphSecretsScannerClient() accessgraphsecretsv1pb.SecretsScannerServiceClient
}

// WatcherConfig is the configuration for the Watcher.
type WatcherConfig struct {
	// Client is the client to use to communicate with the cluster.
	Client ClusterClient
	// Logger is the logger to use.
	Logger *slog.Logger
	// Clock is the clock to use.
	Clock clockwork.Clock
	// HostID is the ID of the host.
	HostID string
	// getRuntimeOS returns the runtime operating system.
	// used for testing purposes.
	getRuntimeOS func() string
	// getHostUsers is a function that returns the list of users on the system.
	// used for testing purposes. When nil, it uses the default implementation
	// that leverages getpwent.
	getHostUsers func() ([]user.User, error)
}

// NewWatcher creates a new Watcher instance.
// Returns [ErrUnsupportedPlatform] if the operating system is not supported.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {

	switch platform := getOS(config); platform {
	case constants.LinuxOS, constants.DarwinOS:
	default:
		return nil, trace.Wrap(ErrUnsupportedPlatform)
	}

	if config.HostID == "" {
		return nil, trace.BadParameter("missing host ID")
	}
	if config.Client == nil {
		return nil, trace.BadParameter("missing client")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Clock == nil {
		config.Clock = clockwork.NewRealClock()
	}
	if config.getHostUsers == nil {
		config.getHostUsers = getHostUsers
	}

	w := &Watcher{
		client:       config.Client,
		logger:       config.Logger,
		clock:        config.Clock,
		hostID:       config.HostID,
		getHostUsers: config.getHostUsers,
	}

	return w, nil
}

func (w *Watcher) Run(ctx context.Context) error {
	return trace.Wrap(w.monitorClusterConfigAndStart(ctx))
}

func (w *Watcher) monitorClusterConfigAndStart(ctx context.Context) error {
	const tickerInterval = 30 * time.Minute
	return trace.Wrap(supervisorRunner(ctx, supervisorRunnerConfig{
		clock:                 w.clock,
		tickerInterval:        tickerInterval,
		runner:                w.start,
		checkIfMonitorEnabled: w.isAuthorizedKeysReportEnabled,
		logger:                w.logger,
	}))
}

// start starts the watcher.
func (w *Watcher) start(ctx context.Context) error {
	wg := sync.WaitGroup{}
	defer wg.Wait()

	fileWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := fileWatcher.Close(); err != nil {
			w.logger.WarnContext(ctx, "Failed to close watcher", "error", err)
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reload := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-fileWatcher.Events:
			innerLoop:
				for {
					select {
					case <-ctx.Done():
						return
					case <-fileWatcher.Events:
					case reload <- struct{}{}:
						break innerLoop
					}
				}
			case err := <-fileWatcher.Errors:
				w.logger.WarnContext(ctx, "Error watching authorized_keys file", "error", err)
			}
		}
	}()

	const etcPasswd = "/etc/passwd"
	if err := fileWatcher.Add(etcPasswd); err != nil {
		w.logger.WarnContext(ctx, "Failed to add watcher for file", "error", err)
	}

	// Wait for the initial delay before sending the first report to spread the load.
	// The initial delay is a random value between 0 and maxInitialDelay.
	const maxInitialDelay = 5 * time.Minute
	select {
	case <-ctx.Done():
		return nil
	case <-w.clock.After(retryutils.FullJitter(maxInitialDelay)):
	}

	jitterFunc := retryutils.HalfJitter
	// maxReSendInterval is the maximum interval to re-send the authorized keys report
	// to the cluster in case of no changes.
	const maxReSendInterval = accessgraph.AuthorizedKeyDefaultKeyTTL - 20*time.Minute
	expirationTimer := w.clock.NewTimer(jitterFunc(maxReSendInterval))
	defer expirationTimer.Stop()

	// monitorTimer is the timer to monitor existing authorized keys.
	const monitorTimerInterval = 3 * time.Minute
	monitorTimer := w.clock.NewTimer(jitterFunc(monitorTimerInterval))
	defer monitorTimer.Stop()

	resetTimer := func(timer clockwork.Timer, interval time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.Chan():
			default:
			}
		}
		timer.Reset(jitterFunc(interval))
	}

	var requiresReportToExtendTTL bool
	for {

		keysReported, err := w.fetchAndReportAuthorizedKeys(ctx, fileWatcher, requiresReportToExtendTTL)
		expirationTimerInterval := maxReSendInterval
		if err != nil {
			w.logger.WarnContext(ctx, "Failed to report authorized keys", "error", err)
			expirationTimerInterval = maxInitialDelay
		}

		// If the keys were reported, reset the expiration timer.
		if keysReported || requiresReportToExtendTTL {
			resetTimer(expirationTimer, expirationTimerInterval)
		}

		// reset the mandatory report flag.
		requiresReportToExtendTTL = false

		resetTimer(monitorTimer, monitorTimerInterval)

		select {
		case <-ctx.Done():
			return nil
		case <-reload:
		case <-expirationTimer.Chan():
			requiresReportToExtendTTL = true
		case <-monitorTimer.Chan():
		}
	}
}

// isAuthorizedKeysReportEnabled checks if the cluster has authorized keys report enabled.
func (w *Watcher) isAuthorizedKeysReportEnabled(ctx context.Context) (bool, error) {
	accessGraphConfig, err := w.client.GetClusterAccessGraphConfig(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return accessGraphConfig.GetEnabled() && accessGraphConfig.GetSecretsScanConfig().GetSshScanEnabled(), nil
}

// fetchAuthorizedKeys fetches the authorized keys from the system.
func (w *Watcher) fetchAuthorizedKeys(
	ctx context.Context,
	fileWatcher *fsnotify.Watcher,
) ([]*accessgraphsecretsv1pb.AuthorizedKey, error) {
	users, err := w.getHostUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var keys []*accessgraphsecretsv1pb.AuthorizedKey
	for _, u := range users {
		if u.HomeDir == "" {
			w.logger.DebugContext(ctx, "Skipping user with empty home directory", "user", u.Name)
			continue
		}

		for _, file := range []string{"authorized_keys", "authorized_keys2"} {
			authorizedKeysPath := filepath.Join(u.HomeDir, ".ssh", file)
			if fs, err := os.Stat(authorizedKeysPath); err != nil || fs.IsDir() {
				continue
			}

			hostKeys, err := w.parseAuthorizedKeysFile(ctx, u, authorizedKeysPath)
			if errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				w.logger.WarnContext(ctx, "Failed to parse authorized_keys file", "error", err)
				continue
			}

			// Add the file to the watcher. If file was already added, this is a no-op.
			if err := fileWatcher.Add(authorizedKeysPath); err != nil {
				w.logger.WarnContext(ctx, "Failed to add watcher for file", "error", err)
			}
			keys = append(keys, hostKeys...)
		}
	}
	return keys, nil
}

// fetchAndReportAuthorizedKeys fetches the authorized keys from the system and reports them to the cluster.
func (w *Watcher) fetchAndReportAuthorizedKeys(
	ctx context.Context,
	fileWatcher *fsnotify.Watcher,
	requiresReportToExtendTTL bool,
) (reported bool, returnErr error) {

	// fetchAuthorizedKeys fetches the authorized keys from the system.
	keys, err := w.fetchAuthorizedKeys(ctx, fileWatcher)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// for the given keys, sort the key names and return them.
	// This is used to compare the key names with the previously reported key names.
	// Key names are hashed fingerprints of the keys and the host user name so they
	// are unique per key and user.
	keyNames := getSortedKeyNames(keys)
	// If the cluster does not require a report to extend the TTL of the authorized keys,
	// and the key names are the same, there is no need to report the keys.
	if !requiresReportToExtendTTL && slices.Equal(w.keyNames, keyNames) {
		return false, nil
	}

	// Report the authorized keys to the cluster.
	w.keyNames = keyNames

	stream, err := w.client.AccessGraphSecretsScannerClient().ReportAuthorizedKeys(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer func() {
		if closeErr := stream.CloseSend(); closeErr != nil && !errors.Is(closeErr, io.EOF) {
			w.logger.WarnContext(ctx, "Failed to close stream", "error", closeErr)
		}

		// wait for the stream to be closed by the server.
		_, err = stream.Recv()
		if errors.Is(returnErr, io.EOF) {
			returnErr = err
		} else {
			returnErr = trace.NewAggregate(err, returnErr)
		}
		if errors.Is(returnErr, io.EOF) {
			returnErr = nil
		}
	}()
	const maxKeysPerReport = 500
	for i := 0; i < len(keys); i += maxKeysPerReport {
		start := i
		end := min(i+maxKeysPerReport, len(keys))
		if err := stream.Send(
			&accessgraphsecretsv1pb.ReportAuthorizedKeysRequest{
				Keys:      keys[start:end],
				Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_ADD,
			},
		); err != nil {
			return false, trace.Wrap(err)
		}
	}

	if err := stream.Send(
		&accessgraphsecretsv1pb.ReportAuthorizedKeysRequest{Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_SYNC},
	); err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

func (w *Watcher) parseAuthorizedKeysFile(ctx context.Context, u user.User, authorizedKeysPath string) ([]*accessgraphsecretsv1pb.AuthorizedKey, error) {
	file, err := os.Open(authorizedKeysPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			w.logger.WarnContext(ctx, "Failed to close file", "error", err, "path", authorizedKeysPath)
		}
	}()

	var keys []*accessgraphsecretsv1pb.AuthorizedKey
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		payload := scanner.Bytes()
		if len(payload) == 0 || payload[0] == '#' {
			continue
		}
		parsedKey, comment, _, _, err := ssh.ParseAuthorizedKey(payload)
		if err != nil {
			w.logger.WarnContext(ctx, "Failed to parse authorized key", "error", err)
			continue
		} else if parsedKey == nil {
			continue
		}

		authorizedKey, err := accessgraph.NewAuthorizedKey(
			&accessgraphsecretsv1pb.AuthorizedKeySpec{
				HostId:         w.hostID,
				HostUser:       u.Username,
				KeyFingerprint: ssh.FingerprintSHA256(parsedKey),
				KeyComment:     comment,
				KeyType:        parsedKey.Type(),
			},
		)
		if err != nil {
			w.logger.WarnContext(ctx, "Failed to create authorized key", "error", err)
			continue
		}
		keys = append(keys, authorizedKey)
	}

	return keys, nil
}

func getOS(config WatcherConfig) string {
	goos := runtime.GOOS
	if config.getRuntimeOS != nil {
		goos = config.getRuntimeOS()
	}
	return goos
}

func getSortedKeyNames(keys []*accessgraphsecretsv1pb.AuthorizedKey) []string {
	keyNames := make([]string, 0, len(keys))
	for _, key := range keys {
		keyNames = append(keyNames, key.GetMetadata().GetName())
	}
	sort.Strings(keyNames)
	return keyNames
}
