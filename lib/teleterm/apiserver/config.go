// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiserver

import (
	"github.com/gravitational/teleport/lib/teleterm/daemon"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// Config is the APIServer configuration
type Config struct {
	// HostAddr is the APIServer host address
	HostAddr string
	// Daemon is the terminal daemon service
	Daemon *daemon.Service
	// Log is a component logger
	Log logrus.FieldLogger
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.HostAddr == "" {
		return trace.BadParameter("missing HostAddr")
	}

	if c.Daemon == nil {
		return trace.BadParameter("missing daemon service")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "teleterm: api_server")
	}

	return nil
}
