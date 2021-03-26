/*
Copyright 2020 Gravitational, Inc.

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

// Package defaults defines Teleport-specific defaults
package defaults

import (
	"time"

	"github.com/gravitational/teleport/api/constants"
)

const (
	// Namespace is default namespace
	Namespace = "default"

	// ServerKeepAliveTTL is a period between server keep-alives,
	// when servers announce only presence without sending full data
	ServerKeepAliveTTL = 60 * time.Second

	// DefaultDialTimeout is a default TCP dial timeout we set for our
	// connection attempts
	DefaultDialTimeout = 30 * time.Second

	// KeepAliveCountMax is the number of keep-alive messages that can be sent
	// without receiving a response from the client before the client is
	// disconnected. The max count mirrors ClientAliveCountMax of sshd.
	KeepAliveCountMax = 3

	// MaxCertDuration limits maximum duration of validity of issued certificate
	MaxCertDuration = 30 * time.Hour

	// CertDuration is a default certificate duration.
	CertDuration = 12 * time.Hour

	// KeepAliveInterval is interval at which Teleport will send keep-alive
	// messages to the client. The default interval of 5 minutes (300 seconds) is
	// set to help keep connections alive when using AWS NLBs (which have a default
	// timeout of 350 seconds)
	KeepAliveInterval = 5 * time.Minute

	// ServerAnnounceTTL is a period between heartbeats
	// Median sleep time between node pings is this value / 2 + random
	// deviation added to this time to avoid lots of simultaneous
	// heartbeats coming to auth server
	ServerAnnounceTTL = 600 * time.Second
)

// EnhancedEvents returns the default list of enhanced events.
func EnhancedEvents() []string {
	return []string{
		constants.EnhancedRecordingCommand,
		constants.EnhancedRecordingNetwork,
	}
}
