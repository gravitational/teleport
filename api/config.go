package api

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// ClientConfig contains configuration of the client
type ClientConfig struct {
	// Addrs is a list of addresses to dial
	Addrs []utils.NetAddr
	// Dialer is a custom dialer, if provided
	// is used instead of the list of addresses
	Dialer ContextDialer
	// KeepAlivePeriod defines period between keep alives
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies amount of missed keep alives
	// to wait for until declaring connection as broken
	KeepAliveCount int
	// TLS is a TLS config
	TLS *tls.Config
}

// CheckAndSetDefaults checks and sets default config values
func (c *ClientConfig) CheckAndSetDefaults() error {
	if len(c.Addrs) == 0 && c.Dialer == nil {
		return trace.BadParameter("set parameter Addrs or DialContext")
	}
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if c.KeepAlivePeriod == 0 {
		c.KeepAlivePeriod = defaults.ServerKeepAliveTTL
	}
	if c.KeepAliveCount == 0 {
		c.KeepAliveCount = defaults.KeepAliveCountMax
	}
	if c.Dialer == nil {
		c.Dialer = NewAddrDialer(c.Addrs, c.KeepAlivePeriod)
	}
	if c.TLS.ServerName == "" {
		c.TLS.ServerName = teleport.APIDomain
	}
	// this logic is necessary to force client to always send certificate
	// regardless of the server setting, otherwise client may pick
	// not to send the client certificate by looking at certificate request
	if len(c.TLS.Certificates) != 0 {
		cert := c.TLS.Certificates[0]
		c.TLS.Certificates = nil
		c.TLS.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
	}

	return nil
}

// ContextDialer represents network dialer interface that uses context
type ContextDialer interface {
	// DialContext is a function that dials to the specified address
	DialContext(in context.Context, network, addr string) (net.Conn, error)
}

// ContextDialerFunc is a function wrapper that implements
// ContextDialer interface
type ContextDialerFunc func(in context.Context, network, addr string) (net.Conn, error)

// DialContext is a function that dials to the specified address
func (f ContextDialerFunc) DialContext(in context.Context, network, addr string) (net.Conn, error) {
	return f(in, network, addr)
}

// NewAddrDialer returns new dialer from a list of addresses
func NewAddrDialer(addrs []utils.NetAddr, keepAliveInterval time.Duration) ContextDialer {
	dialer := net.Dialer{
		Timeout:   defaults.DefaultDialTimeout,
		KeepAlive: keepAliveInterval,
	}
	return ContextDialerFunc(func(in context.Context, network, _ string) (net.Conn, error) {
		var err error
		var conn net.Conn
		for _, addr := range addrs {
			conn, err = dialer.DialContext(in, network, addr.Addr)
			if err == nil {
				return conn, nil
			}
			log.Errorf("Failed to dial auth server %v: %v.", addr.Addr, err)
		}
		// not wrapping on purpose to preserve the original error
		return nil, err
	})
}
