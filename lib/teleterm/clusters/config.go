/*
Copyright 2021 Gravitational, Inc.

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

package clusters

import (
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Config is the cluster service config
type Config struct {
	// Dir is the directory to store cluster profiles
	Dir string
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// InsecureSkipVerify is an option to skip TLS cert check
	InsecureSkipVerify bool
	// Log is a component logger
	Log logrus.FieldLogger
}

// CheckAndSetDefaults checks the configuration for its validity and sets default values if needed
func (c *Config) CheckAndSetDefaults() error {
	if c.Dir == "" {
		return trace.BadParameter("missing working directory")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "teleterm: storage")
	}

	return nil
}
