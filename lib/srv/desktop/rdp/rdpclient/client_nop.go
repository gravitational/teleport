//+build !desktop_access_beta

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

// This file lets us compile /lib/srv/desktop without including the real RDP
// implementation yet. Use the desktop_access_beta build tag to include the
// real implementation.

package rdpclient

import (
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/srv/desktop/deskproto"
)

// Config for creating a new Client.
type Config struct {
	// Addr is the network address of the RDP server, in the form host:port.
	Addr string
	// InputMessage is called to receive a message from the client for the RDP
	// server. This function should block until there is a message.
	InputMessage func() (deskproto.Message, error)
	// OutputMessage is called to send a message from RDP server to the client.
	OutputMessage func(deskproto.Message) error
	// Log is the logger for status messages.
	Log logrus.FieldLogger
}

// Client is the dummy RDP client.
type Client struct {
}

// New creates and connects a new Client based on opts.
func New(cfg Config) (*Client, error) {
	return &Client{}, errors.New("the real rdpclient.Client implementation was not included in this build")
}

// Wait blocks until the client disconnects and runs the cleanup.
func (c *Client) Wait() error {
	return errors.New("the real rdpclient.Client implementation was not included in this build")
}
