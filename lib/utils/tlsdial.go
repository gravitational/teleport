// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"context"
	"crypto/tls"
	"net"
	"strings"

	"github.com/gravitational/trace"
)

// DialWithContextFunc dials with context
type DialWithContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// TLSDial dials and establishes TLS connection using custom dialer
// is similar to tls.DialWithDialer
func TLSDial(ctx context.Context, dial DialWithContextFunc, network, addr string, tlsConfig *tls.Config) (*tls.Conn, error) {
	if tlsConfig == nil {
		return nil, trace.BadParameter("tls config must be specified")
	}

	plainConn, err := dial(ctx, network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if tlsConfig.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		c := tlsConfig.Clone()
		c.ServerName = hostname
		tlsConfig = c
	}

	conn := tls.Client(plainConn, tlsConfig)
	errC := make(chan error, 1)
	go func() {
		err := conn.HandshakeContext(ctx)
		errC <- err
	}()

	select {
	case err := <-errC:
		if err != nil {
			plainConn.Close()
			return nil, trace.Wrap(err)
		}
	case <-ctx.Done():
		plainConn.Close()
		return nil, trace.BadParameter("tls handshake has been canceled due to timeout")
	}

	if tlsConfig.InsecureSkipVerify {
		return conn, nil
	}

	if err := conn.VerifyHostname(tlsConfig.ServerName); err != nil {
		plainConn.Close()
		return nil, trace.Wrap(err)
	}

	return conn, nil
}
