//+build !desktop_access_beta

// This file lets us compile /lib/srv/desktop without including the real RDP
// implementation yet. Use the desktop_access_beta build tag to include the
// real implementation.

package rdpclient

import (
	"errors"

	"github.com/gravitational/teleport/lib/srv/desktop/deskproto"
)

// Options for creating a new Client.
type Options struct {
	// Addr is the network address of the RDP server, in the form host:port.
	Addr string
	// InputMessage is called to receive a message from the client for the RDP
	// server. This function should block until there is a message.
	InputMessage func() (deskproto.Message, error)
	// OutputMessage is called to send a message from RDP server to the client.
	OutputMessage func(deskproto.Message) error
}

// Client is the dummy RDP client.
type Client struct {
}

// New creates and connects a new Client based on opts.
func New(opts Options) (*Client, error) {
	return &Client{}, errors.New("the real rdpclient.Client implementation was not included in this build")
}

// Wait blocks until the client disconnects and runs the cleanup.
func (c *Client) Wait() error {
	return errors.New("the real rdpclient.Client implementation was not included in this build")
}
