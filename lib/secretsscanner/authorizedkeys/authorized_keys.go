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
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
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
	client           ClusterClient
	logger           *slog.Logger
	clock            clockwork.Clock
	hostID           string
	usersAccountFile string
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
	// etcPasswdFile is the path to the file that contains the users account information on the system.
	// This file is used to get the list of users on the system and their home directories.
	// Value is set to "/etc/passwd" by default.
	etcPasswdFile string
}

// NewWatcher creates a new Watcher instance.
// Returns [ErrUnsupportedPlatform] if the operating system is not supported.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {

	if getOS(config) != constants.LinuxOS {
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
	if config.etcPasswdFile == "" {
		// etcPasswordPath is the path to the password file.
		// This file is used to get the list of users on the system and their home directories.
		const etcPasswordPath = "/etc/passwd"
		config.etcPasswdFile = etcPasswordPath
	}

	w := &Watcher{
		client:           config.Client,
		logger:           config.Logger,
		clock:            config.Clock,
		hostID:           config.HostID,
		usersAccountFile: config.etcPasswdFile,
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
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
			case <-watcher.Events:
			innerLoop:
				for {
					select {
					case <-ctx.Done():
						return
					case <-watcher.Events:
					case reload <- struct{}{}:
						break innerLoop
					}
				}
			case err := <-watcher.Errors:
				w.logger.Warn("Error watching authorized_keys file", "error", err)
			}
		}
	}()

	if err := watcher.Add(w.usersAccountFile); err != nil {
		w.logger.Warn("Failed to add watcher for file", "error", err)
	}

	stream, err := w.client.AccessGraphSecretsScannerClient().ReportAuthorizedKeys(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for the initial delay before sending the first report to spread the load.
	// The initial delay is a random value between 0 and maxInitialDelay.
	const maxInitialDelay = 5 * time.Minute
	select {
	case <-ctx.Done():
		return nil
	case <-w.clock.After(retryutils.NewFullJitter()(maxInitialDelay)):
	}

	jitterFunc := retryutils.NewHalfJitter()
	// maxReSendInterval is the maximum interval to re-send the authorized keys report
	// to the cluster in case of no changes.
	const maxReSendInterval = accessgraph.AuthorizedKeyDefaultKeyTTL - 20*time.Minute
	timer := w.clock.NewTimer(jitterFunc(maxReSendInterval))
	defer timer.Stop()
	for {

		if err := w.fetchAndReportAuthorizedKeys(ctx, stream, watcher); err != nil {
			w.logger.Warn("Failed to report authorized keys", "error", err)
		}

		if !timer.Stop() {
			<-timer.Chan()
		}
		timer.Reset(jitterFunc(maxReSendInterval))

		select {
		case <-ctx.Done():
			return nil
		case <-reload:
		case <-timer.Chan():
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

// fetchAndReportAuthorizedKeys fetches the authorized keys from the system and reports them to the cluster.
func (w *Watcher) fetchAndReportAuthorizedKeys(
	ctx context.Context,
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient,
	watcher *fsnotify.Watcher,
) error {
	users, err := userList(ctx, w.logger, w.usersAccountFile)
	if err != nil {
		return trace.Wrap(err)
	}
	var keys []*accessgraphsecretsv1pb.AuthorizedKey
	for _, u := range users {
		if u.HomeDir == "" {
			w.logger.DebugContext(ctx, "Skipping user with empty home directory", "user", u.Name)
			continue
		}

		authorizedKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
		if fs, err := os.Stat(authorizedKeysPath); err != nil || fs.IsDir() {
			continue
		}

		hostKeys, err := w.parseAuthorizedKeysFile(u, authorizedKeysPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			w.logger.Warn("Failed to parse authorized_keys file", "error", err)
			continue
		}

		// Add the file to the watcher. If file was already added, this is a no-op.
		if err := watcher.Add(authorizedKeysPath); err != nil {
			w.logger.Warn("Failed to add watcher for file", "error", err)
		}
		keys = append(keys, hostKeys...)
	}

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
			return trace.Wrap(err)
		}
	}

	if err := stream.Send(
		&accessgraphsecretsv1pb.ReportAuthorizedKeysRequest{Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_SYNC},
	); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// userList retrieves all users on the system
func userList(ctx context.Context, log *slog.Logger, filePath string) ([]user.User, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.DebugContext(ctx, "Failed to close file", "error", err, "file", filePath)
		}
	}()

	var users []user.User
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// username:password:uid:gid:gecos:home:shell
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		users = append(users, user.User{
			Username: parts[0],
			Uid:      parts[2],
			Gid:      parts[3],
			Name:     parts[4],
			HomeDir:  parts[5],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (w *Watcher) parseAuthorizedKeysFile(u user.User, authorizedKeysPath string) ([]*accessgraphsecretsv1pb.AuthorizedKey, error) {
	file, err := os.Open(authorizedKeysPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			w.logger.Warn("Failed to close file", "error", err, "path", authorizedKeysPath)
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
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(payload)
		if err != nil {
			w.logger.Warn("Failed to parse authorized key", "error", err)
			continue
		} else if parsedKey == nil {
			continue
		}

		authorizedKey, err := accessgraph.NewAuthorizedKey(
			&accessgraphsecretsv1pb.AuthorizedKeySpec{
				HostId:         w.hostID,
				HostUser:       u.Username,
				KeyFingerprint: ssh.FingerprintSHA256(parsedKey),
			},
		)
		if err != nil {
			w.logger.Warn("Failed to create authorized key", "error", err)
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
