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
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

var errShutdown = errors.New("watcher is shutting down")

const (
	// etcPasswordPath is the path to the password file.
	// This file is used to get the list of users on the system and their home directories.
	etcPasswordPath = "/etc/passwd"
)

// Watcher watches for changes to authorized_keys files
// and reports them to the cluster. If the cluster does not have
// scanning enabled, the watcher will hold until the feature is enabled.
type Watcher struct {
	// client is the client to use to communicate with the cluster.
	client ClusterClient
	logger *slog.Logger
	clock  clockwork.Clock
	hostID string
}

// ClusterClient is the client to use to communicate with the cluster.
type ClusterClient interface {
	GetClusterAccessGraphConfig(context.Context) (*clusterconfigpb.AccessGraphConfig, error)
	AccessGraphSecretsScannerClient() accessgraphsecretsv1pb.SecretsScannerServiceClient
}

// WatcherConfig is the configuration for the Watcher.
type WatcherConfig struct {
	// ClusterName is the name of the cluster.
	ClusterName string
	// Client is the client to use to communicate with the cluster.
	Client ClusterClient
	// Logger is the logger to use.
	Logger *slog.Logger
	// Clock is the clock to use.
	Clock clockwork.Clock
	// HostID is the ID of the host.
	HostID string
}

// NewWatcher creates a new Watcher instance.
func NewWatcher(ctx context.Context, config WatcherConfig) (*Watcher, error) {
	w := &Watcher{
		client: config.Client,
		logger: config.Logger,
		clock:  config.Clock,
		hostID: config.HostID,
	}

	return w, nil
}

func (w *Watcher) Run(ctx context.Context) error {
	return trace.Wrap(w.monitorClusterConfigAndStart(ctx))
}

func (w *Watcher) monitorClusterConfigAndStart(ctx context.Context) error {
	const tickerInterval = 5 * time.Minute
	var (
		isRunning     = false
		runCtx        context.Context
		runcCtxCancel context.CancelCauseFunc
		wg            sync.WaitGroup
		mu            sync.Mutex
	)

	t := w.clock.NewTimer(tickerInterval)
	for {
		switch enabled, err := w.isAuthorizedKeysReportEnabled(ctx); {
		case err != nil:
			w.logger.WarnContext(ctx, "Failed to check if authorized keys report is enabled", "error", err)

		case enabled:
			if !isRunning {
				runCtx, runcCtxCancel = context.WithCancelCause(ctx)
				mu.Lock()
				isRunning = true
				mu.Unlock()
				wg.Add(1)
				go func(ctx context.Context, cancel context.CancelCauseFunc) {
					defer func() {
						wg.Done()
						cancel(errShutdown)
						mu.Lock()
						isRunning = false
						mu.Unlock()
					}()
					switch err := w.start(ctx); {
					case errors.Is(err, errShutdown):
						w.logger.DebugContext(ctx, "Watcher is shutting down")
					case err != nil:
						w.logger.WarnContext(ctx, "Watcher failed", "error", err)
					}
				}(runCtx, runcCtxCancel)

			}

		default:
			mu.Lock()
			if isRunning {
				runcCtxCancel(errShutdown)
				isRunning = false
				wg.Wait()
			}
			mu.Unlock()
		}

		select {
		case <-t.Chan():
			if !t.Stop() {
				<-t.Chan()
			}
			t.Reset(tickerInterval)
		case <-ctx.Done():
			return nil
		}
	}
}

// start starts the watcher.
func (w *Watcher) start(ctx context.Context) error {
	wg := sync.WaitGroup{}
	defer wg.Wait()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	if err := watcher.Add(etcPasswordPath); err != nil {
		w.logger.Warn("Failed to add watcher for file", "error", err)
	}

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

	stream, err := w.client.AccessGraphSecretsScannerClient().ReportAuthorizedKeys(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	timer := w.clock.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		users, err := userList()
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
			if err != nil {
				w.logger.Warn("Failed to parse authorized_keys file", "error", err)
				continue
			}

			// Add the file to the watcher. If file was already added, this is a no-op.
			if err := watcher.Add(authorizedKeysPath); err != nil {
				w.logger.Warn("Failed to add watcher for file", "error", err)
			}
			keys = append(keys, hostKeys...)
		}

		if err := stream.Send(
			&accessgraphsecretsv1pb.ReportAuthorizedKeysRequest{Keys: keys},
		); err != nil {
			return trace.Wrap(err)
		}

		if !timer.Stop() {
			<-timer.Chan()
		}
		timer.Reset(5 * time.Minute)

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
	return accessGraphConfig.GetEnabled() && accessGraphConfig.GetSshScanEnabled(), nil
}

// userList retrieves all users on the system
func userList() ([]user.User, error) {
	file, err := os.Open(etcPasswordPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var users []user.User
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
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
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

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
