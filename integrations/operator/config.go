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

package main

import (
	"flag"
	"time"

	"github.com/gravitational/trace"
)

const defaultSyncPeriod = 10 * time.Hour

// operatorConfig contains all the operator-specific configuration
type operatorConfig struct {
	metricsAddr      string
	probeAddr        string
	pprofAddr        string
	leaderElectionID string
	syncPeriod       time.Duration
	namespace        string
	logLevel         string
}

// BindFlags binds operatorConfig fields to CLI flags.
// When calling flag.Parse(), the operator configuration will be parsed and
// the structure populated. As the user might provide invalid values or
// configuration might come from different sources (env vars, filesystem, ...),
// one must run CheckAndSetDefault() before consuming the configuration.
func (c *operatorConfig) BindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&c.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	// pprof is disabled by default
	fs.StringVar(&c.pprofAddr, "pprof-bind-address", "", "The address the pprof endpoint binds to, leave empty to disable.")
	fs.StringVar(&c.leaderElectionID, "leader-election-id", "431e83f4.teleport.dev", "Leader Election Id to use.")
	fs.StringVar(&c.namespace, "namespace", "", "The namespace containing the Teleport CRs.")
	fs.DurationVar(&c.syncPeriod, "sync-period", defaultSyncPeriod, "Operator sync period (format: https://pkg.go.dev/time#ParseDuration)")
	fs.StringVar(&c.logLevel, "log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR).")
}

// CheckAndSetDefaults checks the operatorConfig and populates unspecified
// fields for non-flag sources (env vars or filesystem).
func (c *operatorConfig) CheckAndSetDefaults() error {
	var err error
	if c.namespace == "" {
		c.namespace, err = GetKubernetesNamespace()
		if err != nil {
			return trace.BadParameter(
				"Specifying namespace is mandatory. This can be done through the `-namespace` flag, the `%s` variable, or the file `%s`.",
				namespaceEnvVar, namespacePath,
			)
		}
	}
	return nil
}
