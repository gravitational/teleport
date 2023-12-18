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
	"io"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	grpcbackoff "google.golang.org/grpc/backoff"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/lib/credentials"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

const (
	// grpcBackoffMaxDelay is a maximum time gRPC client waits before reconnection attempt.
	grpcBackoffMaxDelay = time.Second * 2
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
)

// TeleportConfig stores config options for where
// the Teleport's Auth server is listening, and what certificates to
// use to authenticate in it.
type TeleportConfig struct {
	// AuthServer specifies the address that the client should connect to.
	// Deprecated: replaced by Addr
	AuthServer string `toml:"auth_server"`
	Addr       string `toml:"addr"`

	ClientKey string `toml:"client_key"`
	ClientCrt string `toml:"client_crt"`
	RootCAs   string `toml:"root_cas"`

	Identity                string        `toml:"identity"`
	RefreshIdentity         bool          `toml:"refresh_identity"`
	RefreshIdentityInterval time.Duration `toml:"refresh_identity_interval"`
}

func (cfg *TeleportConfig) CheckAndSetDefaults() error {
	if err := cfg.CheckTLSConfig(); err != nil {
		return trace.Wrap(err)
	}

	if cfg.Identity != "" && cfg.ClientCrt != "" {
		return trace.BadParameter("configuration setting `identity` is mutually exclusive with all the `client_crt`, `client_key` and `root_cas` settings")
	}

	// Default to refreshing identity minutely.
	if cfg.RefreshIdentityInterval == 0 {
		cfg.RefreshIdentityInterval = time.Minute
	}
	if cfg.RefreshIdentity && cfg.Identity == "" {
		return trace.BadParameter("`refresh_identity` requires that `identity` be set")
	}

	return nil
}

func (cfg *TeleportConfig) CheckTLSConfig() error {
	provided := stringset.NewWithCap(3)
	missing := stringset.NewWithCap(3)

	if cfg.ClientCrt != "" {
		provided.Add("`client_crt`")
	} else {
		missing.Add("`client_crt`")
	}

	if cfg.ClientKey != "" {
		provided.Add("`client_key`")
	} else {
		missing.Add("`client_key`")
	}

	if cfg.RootCAs != "" {
		provided.Add("`root_cas`")
	} else {
		missing.Add("`root_cas`")
	}

	if len(provided) > 0 && len(provided) < 3 {
		return trace.BadParameter(
			"configuration setting(s) %s are provided but setting(s) %s are missing",
			strings.Join(provided.ToSlice(), ", "),
			strings.Join(missing.ToSlice(), ", "),
		)
	}

	return nil
}

// NewIdentityFileWatcher returns a credential compatible with the Teleport
// client. This credential will reload from the identity file at the specified
// path each time interval time passes. This function blocks until the initial
// credential has been loaded and then returns, creating a goroutine in the
// background to manage the reloading that will exit when ctx is canceled.
func NewIdentityFileWatcher(ctx context.Context, path string, interval time.Duration) (*client.DynamicIdentityFileCreds, error) {
	dynamicCred, err := client.NewDynamicIdentityFileCreds(path)
	if err != nil {
		return nil, trace.Wrap(err, "creating dynamic identity file watcher")
	}

	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}

			if err := dynamicCred.Reload(); err != nil {
				log.WithError(err).Error("Failed to reload identity file from disk.")
			}
			timer.Reset(interval)
		}
	}()

	return dynamicCred, nil
}

func (cfg TeleportConfig) NewClient(ctx context.Context) (*client.Client, error) {
	addr := "localhost:3025"
	switch {
	case cfg.Addr != "":
		addr = cfg.Addr
	case cfg.AuthServer != "":
		log.Warn("Configuration setting `auth_server` is deprecated, consider to change it to `addr`")
		addr = cfg.AuthServer
	}

	var creds []client.Credentials
	switch {
	case cfg.Identity != "" && !cfg.RefreshIdentity:
		creds = []client.Credentials{client.LoadIdentityFile(cfg.Identity)}
	case cfg.Identity != "" && cfg.RefreshIdentity:
		cred, err := NewIdentityFileWatcher(ctx, cfg.Identity, cfg.RefreshIdentityInterval)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		creds = []client.Credentials{cred}
	case cfg.ClientCrt != "" && cfg.ClientKey != "" && cfg.RootCAs != "":
		creds = []client.Credentials{client.LoadKeyPair(cfg.ClientCrt, cfg.ClientKey, cfg.RootCAs)}
	default:
		return nil, trace.BadParameter("no credentials configured")
	}

	if validCred, err := credentials.CheckIfExpired(creds); err != nil {
		log.Warn(err)
		if !validCred {
			return nil, trace.BadParameter(
				"No valid credentials found, this likely means credentials are expired. In this case, please sign new credentials and increase their TTL if needed.",
			)
		}
		log.Info("At least one non-expired credential has been found, continuing startup")
	}

	bk := grpcbackoff.DefaultConfig
	bk.MaxDelay = grpcBackoffMaxDelay
	clt, err := client.New(ctx, client.Config{
		Addrs:       []string{addr},
		Credentials: creds,
		DialOpts: []grpc.DialOption{
			grpc.WithConnectParams(grpc.ConnectParams{Backoff: bk, MinConnectTimeout: initTimeout}),
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clt, nil
}

// ReadPassword reads password from file or env var, trims and returns
func ReadPassword(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return "", trace.BadParameter("Error reading password from %v", filename)
		}
		return "", trace.Wrap(err)
	}

	pass := make([]byte, 2000)
	l, err := f.Read(pass)
	if err != nil && err != io.EOF {
		return "", err
	}

	pass = pass[:l] // truncate \0
	return strings.TrimSpace(string(pass)), nil
}
