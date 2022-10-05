/*
Copyright 2018-2022 Gravitational, Inc.

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

package sqlbk

import (
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultPurgePeriod is the default frequency for purging database records.
	DefaultPurgePeriod = 20 * time.Second

	// DefaultDatabase is default name of the backend database.
	DefaultDatabase = "teleport"

	// DefaultRetryDelayPeriod is the default delay before a transaction will retry on
	// serialization failure.
	DefaultRetryDelayPeriod = 250 * time.Millisecond

	// DefaultRetryTimeout is the default amount time allocated to retrying transactions.
	DefaultRetryTimeout = 10 * time.Second
)

// Config defines a configuration for the Backend.
type Config struct {
	// Addr defines the host:port of the database instance.
	Addr string `json:"addr,omitempty"`

	// Database is the database where teleport will store its data.
	Database string `json:"database,omitempty"`

	// TLS defines configurations for validating server certificates
	// and mutual authentication.
	TLS struct {
		// ClientKeyFile is the path to the database user's private
		// key file used for authentication.
		ClientKeyFile string `json:"client_key_file,omitempty"`

		// ClientCertFile is the path to the database user's certificate
		// file used for authentication.
		ClientCertFile string `json:"client_cert_file,omitempty"`

		// TLSCAFile is the trusted certificate authority used to generate the
		// client certificates.
		CAFile string `json:"ca_file,omitempty"`
	} `json:"tls"`

	// BufferSize is a default buffer size used to emit events.
	BufferSize int `json:"buffer_size,omitempty"`

	// EventsTTL is amount of time before an event is purged.
	EventsTTL time.Duration `json:"events_ttl,omitempty"`

	// PollStreamPeriod is the polling period for the event stream.
	PollStreamPeriod time.Duration `json:"poll_stream_period,omitempty"`

	// PurgePeriod is the frequency for purging database records.
	PurgePeriod time.Duration `json:"purge_period,omitempty"`

	// RetryDelayPeriod is the frequency a transaction is retried due to
	// serialization conflict.
	RetryDelayPeriod time.Duration `json:"retry_period,omitempty"`

	// RetryTimeout is the amount of time allocated to retrying transactions.
	// Setting a value less than RetryDelayPeriod disables retries.
	RetryTimeout time.Duration `json:"retry_timeout,omitempty"`

	// Clock overrides the clock used by the backend.
	Clock clockwork.Clock `json:"-"`

	// Log defines the log entry used by the backend.
	Log *logrus.Entry `json:"-"`
}

// CheckAndSetDefaults validates required fields and sets default
// values for fields that have not been set.
func (c *Config) CheckAndSetDefaults() error {
	if c.Database == "" {
		c.Database = DefaultDatabase
	}
	if c.BufferSize <= 0 {
		c.BufferSize = backend.DefaultBufferCapacity
	}
	if c.EventsTTL == 0 {
		c.EventsTTL = backend.DefaultEventsTTL
	}
	if c.PollStreamPeriod <= 0 {
		c.PollStreamPeriod = backend.DefaultPollStreamPeriod
	}
	if c.PurgePeriod <= 0 {
		c.PurgePeriod = DefaultPurgePeriod
	}
	if c.RetryDelayPeriod == 0 {
		c.RetryDelayPeriod = DefaultRetryDelayPeriod
	}
	if c.RetryTimeout == 0 {
		c.RetryTimeout = DefaultRetryTimeout
	}
	if c.EventsTTL < c.PollStreamPeriod {
		return trace.BadParameter("PollStreamPeriod must be greater than EventsTTL to emit storage events")
	}
	if c.Log == nil {
		return trace.BadParameter("Log is required")
	}
	if c.Clock == nil {
		return trace.BadParameter("Clock is required")
	}
	if c.Addr == "" {
		return trace.BadParameter("Addr is required")
	}
	if c.TLS.CAFile == "" {
		return trace.BadParameter("TLS.CAFile is required")
	}
	if c.TLS.ClientKeyFile == "" {
		return trace.BadParameter("TLS.ClientKeyFile is required")
	}
	if c.TLS.ClientCertFile == "" {
		return trace.BadParameter("TLS.ClientCertFile is required")
	}
	return nil
}
