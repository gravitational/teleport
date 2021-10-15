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

// Package rdpclient implements an RDP client.
package rdpclient

import (
	"context"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Config for creating a new Client.
type Config struct {
	// Addr is the network address of the RDP server, in the form host:port.
	Addr string
	// UserCertGenerator generates user certificates for RDP authentication.
	GenerateUserCert GenerateUserCertFn

	// AuthorizeFn is called to authorize a user connecting to a Windows desktop.
	AuthorizeFn func(login string) error

	// TODO(zmb3): replace these callbacks with a tdp.Conn

	// InputMessage is called to receive a message from the client for the RDP
	// server. This function should block until there is a message.
	InputMessage func() (tdp.Message, error)
	// OutputMessage is called to send a message from RDP server to the client.
	OutputMessage func(tdp.Message) error

	// Log is the logger for status messages.
	Log logrus.FieldLogger
}

// GenerateUserCertFn generates user certificates for RDP authentication.
type GenerateUserCertFn func(ctx context.Context, username string) (certDER, keyDER []byte, err error)

//nolint:unused
func (c *Config) checkAndSetDefaults() error {
	if c.Addr == "" {
		return trace.BadParameter("missing Addr in rdpclient.Config")
	}
	if c.GenerateUserCert == nil {
		return trace.BadParameter("missing GenerateUserCert in rdpclient.Config")
	}
	if c.InputMessage == nil {
		return trace.BadParameter("missing InputMessage in rdpclient.Config")
	}
	if c.OutputMessage == nil {
		return trace.BadParameter("missing OutputMessage in rdpclient.Config")
	}
	if c.AuthorizeFn == nil {
		return trace.BadParameter("missing AuthorizeFn in rdpclient.Config")
	}
	if c.Log == nil {
		c.Log = logrus.New()
	}
	c.Log = c.Log.WithField("rdp-addr", c.Addr)
	return nil
}
