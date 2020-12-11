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

package api

import "time"

const (
	// DefaultDialTimeout is a default TCP dial timeout we set for our
	// connection attempts
	DefaultDialTimeout = 30 * time.Second

	// HTTPMaxIdleConns is the maximum idle connections across all hosts.
	HTTPMaxIdleConns = 2000

	// HTTPMaxIdleConnsPerHost is the maximum idle connections per-host.
	HTTPMaxIdleConnsPerHost = 1000

	// HTTPMaxConnsPerHost is the maximum number of connections per-host.
	HTTPMaxConnsPerHost = 250

	// HTTPIdleTimeout is a default timeout for idle HTTP connections
	HTTPIdleTimeout = 30 * time.Second

	// KeepAliveCountMax is the number of keep-alive messages that can be sent
	// without receiving a response from the client before the client is
	// disconnected. The value mirrors ClientAliveCountMax of sshd.
	KeepAliveCountMax = 3

	// ServerKeepAliveTTL is a period between server keep alives,
	// when servers only announce presence without sending full data
	ServerKeepAliveTTL = 60 * time.Second

	// SignupTokenTTL is a default TTL for a web signup one time token
	SignupTokenTTL = Duration(time.Hour)

	// MaxSignupTokenTTL is a maximum TTL for a web signup one time token
	// clients can reduce this time, not increase it
	MaxSignupTokenTTL = Duration(48 * time.Hour)

	// MaxChangePasswordTokenTTL is a maximum TTL for password change token
	MaxChangePasswordTokenTTL = Duration(24 * time.Hour)

	// ChangePasswordTokenTTL is a default password change token expiry time
	ChangePasswordTokenTTL = Duration(8 * time.Hour)
)
