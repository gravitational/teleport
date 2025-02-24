/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/stringset"

	"github.com/gravitational/teleport/integrations/event-handler/lib"
)

// FluentdConfig represents fluentd instance configuration
type FluentdConfig struct {
	// FluentdURL fluentd url for audit log events
	FluentdURL string `help:"fluentd url" required:"true" env:"FDFWD_FLUENTD_URL"`

	// FluentdSessionURL
	FluentdSessionURL string `help:"fluentd session url" required:"true" env:"FDFWD_FLUENTD_SESSION_URL"`

	// FluentdCert is a path to fluentd cert
	FluentdCert string `help:"fluentd TLS certificate file" type:"existingfile" env:"FDWRD_FLUENTD_CERT"`

	// FluentdKey is a path to fluentd key
	FluentdKey string `help:"fluentd TLS key file" type:"existingfile" env:"FDWRD_FLUENTD_KEY"`

	// FluentdCA is a path to fluentd CA
	FluentdCA string `help:"fluentd TLS CA file" type:"existingfile" env:"FDWRD_FLUENTD_CA"`

	// FluentdMaxConnections caps the number of connections to fluentd. Defaults to a dynamic value
	// calculated relative to app-level concurrency.
	FluentdMaxConnections int `help:"Maximum number of connections to fluentd" env:"FDWRD_MAX_CONNECTIONS"`
}

// TeleportConfig is Teleport instance configuration
type TeleportConfig struct {
	// TeleportAddr is a Teleport addr
	TeleportAddr string `help:"Teleport addr" env:"FDFWD_TELEPORT_ADDR" default:"localhost:3025"`

	// TeleportIdentityFile is a path to Teleport identity file
	TeleportIdentityFile string `help:"Teleport identity file" type:"existingfile" name:"teleport-identity" env:"FDFWD_TELEPORT_IDENTITY"`

	// TeleportRefreshEnabled will reload the identity file from disk on the
	// configured interval.
	TeleportRefreshEnabled bool `help:"Configures the identity file to be reloaded from disk at a configured interval." env:"FDFWD_TELEPORT_REFRESH_ENABLED"`

	// TeleportRefreshInterval is how often the identity file should
	// be reloaded from disk.
	TeleportRefreshInterval time.Duration `help:"Configures how often the identity file should be reloaded from disk." env:"FDFWD_TELEPORT_REFRESH_INTERVAL" default:"1m"`

	// TeleportCA is a path to Teleport CA file
	TeleportCA string `help:"Teleport TLS CA file" type:"existingfile" env:"FDFWD_TELEPORT_CA"`

	// TeleportCert is a path to Teleport cert file
	TeleportCert string `help:"Teleport TLS certificate file" type:"existingfile" env:"FDWRD_TELEPORT_CERT"`

	// TeleportKey is a path to Teleport key file
	TeleportKey string `help:"Teleport TLS key file" type:"existingfile" env:"FDFWD_TELEPORT_KEY"`
}

// Check verifies that a valid configuration is set
func (cfg *TeleportConfig) Check() error {
	provided := stringset.NewWithCap(3)
	missing := stringset.NewWithCap(3)
	if cfg.TeleportCert != "" {
		provided.Add("`teleport.cert`")
	} else {
		missing.Add("`teleport.cert`")
	}

	if cfg.TeleportKey != "" {
		provided.Add("`teleport.key`")
	} else {
		missing.Add("`teleport.key`")
	}

	if cfg.TeleportCA != "" {
		provided.Add("`teleport.ca`")
	} else {
		missing.Add("`teleport.ca`")
	}

	if len(provided) > 0 && len(provided) < 3 {
		return trace.BadParameter(
			"configuration setting(s) %s are provided but setting(s) %s are missing",
			strings.Join(provided.ToSlice(), ", "),
			strings.Join(missing.ToSlice(), ", "),
		)
	}

	if cfg.TeleportIdentityFile != "" && len(provided) != 0 {
		return trace.BadParameter("configuration setting `identity` is mutually exclusive with the `cert`, `key` and `ca` settings")
	}
	if len(provided) == 0 && cfg.TeleportIdentityFile == "" {
		return trace.BadParameter("neither `identity` file nor `cert`, `key` and `ca` files configured")
	}
	return nil
}

// IngestConfig ingestion configuration
type IngestConfig struct {
	// StorageDir is a path to dv storage dir
	StorageDir string `help:"Storage directory" required:"true" env:"FDFWD_STORAGE" name:"storage"`

	// BatchSize is a fetch batch size
	BatchSize int `help:"Fetch batch size" default:"20" env:"FDFWD_BATCH" name:"batch"`

	// Types are event types to log
	Types []string `help:"Comma-separated list of event types to forward" env:"FDFWD_TYPES"`

	// SkipEventTypesRaw are event types to skip
	SkipEventTypesRaw []string `name:"skip-event-types" help:"Comma-separated list of event types to skip" env:"FDFWD_SKIP_EVENT_TYPES"`

	// SkipEventTypes is a map generated from SkipEventTypesRaw
	SkipEventTypes map[string]struct{} `kong:"-"`

	// SkipSessionTypes are session event types to skip
	SkipSessionTypesRaw []string `name:"skip-session-types" help:"Comma-separated list of session event types to skip" default:"print" env:"FDFWD_SKIP_SESSION_TYPES"`

	// SkipSessionTypes is a map generated from SkipSessionTypes
	SkipSessionTypes map[string]struct{} `kong:"-"`

	// StartTime is a time to start ingestion from
	StartTime *time.Time `help:"Minimum event time in RFC3339 format" env:"FDFWD_START_TIME"`

	// Timeout is the time poller will wait before the new request if there are no events in the queue
	Timeout time.Duration `help:"Polling timeout" default:"5s" env:"FDFWD_TIMEOUT"`

	// DryRun is the flag which simulates execution without sending events to fluentd
	DryRun bool `help:"Events are read from Teleport, but are not sent to fluentd. Separate stroage is used. Debug flag."`

	// ExitOnLastEvent exit when last event is processed
	ExitOnLastEvent bool `help:"Exit when last event is processed"`

	// Concurrency sets the number of concurrent sessions to ingest
	Concurrency int `help:"Number of concurrent sessions" default:"5"`

	//WindowSize is the size of the window to process events
	WindowSize time.Duration `help:"Window size to process events" default:"24h"`
}

// LockConfig represents locking configuration
type LockConfig struct {
	// LockEnabled represents locking enabled flag
	LockEnabled bool `help:"Enable user auto-locking" name:"lock-enabled" default:"false" env:"FDFWD_LOCKING_ENABLED"`
	// LockFailedAttemptsCount number of failed attempts which triggers locking
	LockFailedAttemptsCount int `help:"Number of failed attempts in lock-period which triggers locking" name:"lock-failed-attempts-count" default:"3" env:"FDFWD_LOCKING_FAILED_ATTEMPTS"`
	// LockPeriod represents rate limiting period
	LockPeriod time.Duration `help:"Time period where lock-failed-attempts-count failed attempts will trigger locking" name:"lock-period" default:"1m" env:"FDFWD_LOCKING_PERIOD"`
	// LockFor represents the duration of the new lock
	LockFor time.Duration `help:"Time period for which user gets lock" name:"lock-for" env:"FDFWD_LOCKING_FOR"`
}

// StartCmdConfig is start command description
type StartCmdConfig struct {
	FluentdConfig
	TeleportConfig
	IngestConfig
	LockConfig
}

// ConfigureCmdConfig holds CLI options for teleport-event-handler configure
type ConfigureCmdConfig struct {
	// Out path and file prefix to put certificates into
	Out string `arg:"true" help:"Output directory" type:"existingdir" required:"true"`

	// Output is a mock arg for now, it specifies export target
	Output string `help:"Export target service" type:"string" required:"true" default:"fluentd"`

	// Addr is Teleport auth proxy instance address
	Addr string `arg:"true" help:"Teleport auth proxy instance address" type:"string" required:"true" default:"localhost:3025"`

	// CAName CA certificate and key name
	CAName string `arg:"true" help:"CA certificate and key name" required:"true" default:"ca"`

	// ServerName server certificate and key name
	ServerName string `arg:"true" help:"Server certificate and key name" required:"true" default:"server"`

	// ClientName client certificate and key name
	ClientName string `arg:"true" help:"Client certificate and key name" required:"true" default:"client"`

	// Certificate TTL
	TTL time.Duration `help:"Certificate TTL" required:"true" default:"87600h"`

	// DNSNames is a DNS subjectAltNames for server cert
	DNSNames []string `help:"Certificate SAN hosts" default:"localhost"`

	// HostNames is an IP subjectAltNames for server cert
	IP []string `help:"Certificate SAN IPs"`

	// Length is RSA key length
	Length int `help:"Key length" enum:"1024,2048,4096" default:"4096"`
}

// CLI represents command structure
type CLI struct {
	// Config is the path to configuration file
	Config kong.ConfigFlag `help:"Path to TOML configuration file" optional:"true" short:"c" type:"existingfile" env:"FDFWD_CONFIG"`

	// Debug is a debug logging mode flag
	Debug bool `help:"Debug logging" short:"d" env:"FDFWD_DEBUG"`

	// Version is the version print command
	Version struct{} `cmd:"true" help:"Print plugin version"`

	// Configure is the generate certificates command configuration
	Configure ConfigureCmdConfig `cmd:"true" help:"Generate mTLS certificates for Fluentd"`

	// Start is the start command configuration
	Start StartCmdConfig `cmd:"true" help:"Start log ingestion"`
}

// Validate validates start command arguments and prints them to log
func (c *StartCmdConfig) Validate() error {
	if c.StartTime != nil {
		t := c.StartTime.Truncate(time.Second)
		c.StartTime = &t
	}
	if err := c.TeleportConfig.Check(); err != nil {
		return trace.Wrap(err)
	}
	c.SkipSessionTypes = lib.SliceToAnonymousMap(c.SkipSessionTypesRaw)
	c.SkipEventTypes = lib.SliceToAnonymousMap(c.SkipEventTypesRaw)

	if c.FluentdMaxConnections < 1 {
		// 2x concurrency is effectively uncapped.
		c.FluentdMaxConnections = c.Concurrency * 2
	}

	return nil
}

// Dump dumps configuration values to the log
func (c *StartCmdConfig) Dump(ctx context.Context, log *slog.Logger) {
	// Log configuration variables
	log.InfoContext(ctx, "Using batch size", "batch", c.BatchSize)
	log.InfoContext(ctx, "Using concurrency", "concurrency", c.Concurrency)
	log.InfoContext(ctx, "Using type filter", "types", c.Types)
	log.InfoContext(ctx, "Using type exclude filter", "skip_event_types", c.SkipEventTypes)
	log.InfoContext(ctx, "Skipping session events of type", "types", c.SkipSessionTypes)
	log.InfoContext(ctx, "Using start time", "value", c.StartTime)
	log.InfoContext(ctx, "Using timeout", "timeout", c.Timeout)
	log.InfoContext(ctx, "Using Fluentd url", "url", c.FluentdURL)
	log.InfoContext(ctx, "Using Fluentd session url", "url", c.FluentdSessionURL)
	log.InfoContext(ctx, "Using Fluentd ca", "ca", c.FluentdCA)
	log.InfoContext(ctx, "Using Fluentd cert", "cert", c.FluentdCert)
	log.InfoContext(ctx, "Using Fluentd key", "key", c.FluentdKey)
	log.InfoContext(ctx, "Using Fluentd max connections", "max_connections", c.FluentdMaxConnections)
	log.InfoContext(ctx, "Using window size", "window_size", c.WindowSize)

	if c.TeleportIdentityFile != "" {
		log.InfoContext(ctx, "Using Teleport identity file", "file", c.TeleportIdentityFile)
	}
	if c.TeleportRefreshEnabled {
		log.InfoContext(ctx, "Using Teleport identity file refresh", "interval", c.TeleportRefreshInterval)
	}

	if c.TeleportKey != "" {
		log.InfoContext(ctx, "Using Teleport addr", "addr", c.TeleportAddr)
		log.InfoContext(ctx, "Using Teleport CA", "ca", c.TeleportCA)
		log.InfoContext(ctx, "Using Teleport cert", "cert", c.TeleportCert)
		log.InfoContext(ctx, "Using Teleport key", "key", c.TeleportKey)
	}

	if c.LockEnabled {
		log.InfoContext(ctx, "Auto-locking enabled", "count", c.LockFailedAttemptsCount, "period", c.LockPeriod)
	}

	if c.DryRun {
		log.WarnContext(ctx, "Dry run! Events are not sent to Fluentd. Separate storage is used.")
	}
}
