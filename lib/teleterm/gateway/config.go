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

package gateway

import (
	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// CreateGatewayParams describes gateway parameters
type Config struct {
	// URI is the gateway URI
	URI uri.ResourceURI
	// TargetName is the remote resource name
	TargetName string
	// TargetURI is the remote resource URI
	TargetURI string
	// TargetUser is the target user name
	TargetUser string

	// Port is the gateway port
	LocalPort string
	// LocalAddress is the local address
	LocalAddress string
	// Protocol is the gateway protocol
	Protocol string
	// CACertPath
	CACertPath string
	// CertPath
	CertPath string
	// KeyPath
	KeyPath string
	// Insecure
	Insecure bool
	// WebProxyAddr
	WebProxyAddr string
	// Log is a component logger
	Log logrus.FieldLogger
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.URI.String() == "" {
		c.URI = uri.NewGatewayURI(uuid.NewString())
	}

	if c.LocalAddress == "" {
		c.LocalAddress = "localhost"
	}

	if c.LocalPort == "" {
		c.LocalPort = "0"
	}

	if c.Log == nil {
		c.Log = logrus.WithField("gateway", c.URI.String())
	}

	if c.TargetName == "" {
		return trace.BadParameter("missing target name")
	}

	if c.TargetURI == "" {
		return trace.BadParameter("missing target URI")
	}

	return nil
}
