/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package health

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
)

// KeyManager allows getting a signer for each CA we expect to health check for.
type KeyManager interface {
	GetTLSSigner(context.Context, types.CertAuthority) (crypto.Signer, error)
	GetSSHSigner(context.Context, types.CertAuthority) (ssh.Signer, error)
	GetJWTSigner(context.Context, types.CertAuthority) (crypto.Signer, error)
}

// ActiveHealthCheckConfig contains values for configuring an [ActiveHealthChecker].
type ActiveHealthCheckConfig struct {
	// Interval is the duration waited between signing requests when there have
	// been no failures.
	Interval time.Duration
	// FailureInterval is the duration waited between calls after a failure
	// occurs.
	FailureInterval time.Duration
	// Callback should be a non-blocking call that is passed the result of
	// each signing request made by the health checker.
	// The first callback will occur only after receiving a cert authority event.
	Callback func(error)
	// ResourceC is a channel for receiving cert authority events.
	// It is expected that the full list of cert authorities is provided
	// in each event.
	ResourceC <-chan []types.CertAuthority
	// KeyManager allows getting signers for each CA we expect to make requests against.
	KeyManager KeyManager
	// Logger configures a structured logger to use.
	Logger *slog.Logger
}

// NewActiveHealthChecker constructs an [ActiveHealthChecker] instance.
func NewActiveHealthChecker(c ActiveHealthCheckConfig) (*ActiveHealthChecker, error) {
	if c.Callback == nil {
		return nil, trace.BadParameter("health check callback must be specified")
	}
	if c.Interval == 0 {
		c.Interval = time.Minute
	}
	if c.FailureInterval == 0 {
		c.FailureInterval = time.Second * 10
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}

	return &ActiveHealthChecker{
		interval:        c.Interval,
		failureInterval: c.FailureInterval,
		callback:        c.Callback,
		c:               c.ResourceC,
		firstEvent:      make(chan struct{}, 1),
		m:               c.KeyManager,
		logger:          c.Logger,
		signers:         make([]*healthSigner, 0),
		healthFn:        sign,
	}, nil
}

// ActiveHealthChecker makes signing requests to CAs and reports errors back to
// the configured callback. CAs are health checked one at a time at the given
// interval.
type ActiveHealthChecker struct {
	interval        time.Duration
	failureInterval time.Duration
	healthFn        func(*healthSigner) error

	// firstEvent is sent on and closed after receiving the first message on c.
	// If c is closed before any message is received firstEvent is closed without
	// being sent on.
	firstEvent chan struct{}
	c          <-chan []types.CertAuthority

	m        KeyManager
	callback func(error)
	logger   *slog.Logger

	// mu protects signers.
	mu      sync.RWMutex
	signers []*healthSigner
}

// Run executes the main health checking loop, iterating over CAs and making
// signing requests.
func (c *ActiveHealthChecker) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := c.watch(ctx)
		c.logger.ErrorContext(ctx, "CA event watcher exited", "error", err)
		cancel()
	}()

	select {
	case _, ok := <-c.firstEvent:
		if !ok {
			return trace.Errorf("failed to start active health checker: failed to receive first CA event")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	var (
		signer *healthSigner
		err    error
	)
	for {
		signer, err = c.step(signer)
		c.callback(err)
		if err != nil {
			ticker.Reset(c.failureInterval)
		} else {
			ticker.Reset(c.interval)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *ActiveHealthChecker) step(curr *healthSigner) (*healthSigner, error) {
	if !c.exists(curr) {
		n, err := c.nextSigner(curr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		curr = n
	}
	if curr != nil {
		err := c.healthFn(curr)
		if err != nil {
			return curr, trace.Wrap(err)
		}
	}
	next, err := c.nextSigner(curr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return next, nil
}

func (c *ActiveHealthChecker) exists(s *healthSigner) bool {
	if s == nil {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, signer := range c.signers {
		if signer.Equal(s) {
			return true
		}
	}
	return false
}

func (c *ActiveHealthChecker) nextSigner(last *healthSigner) (*healthSigner, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := len(c.signers)
	if n == 0 {
		return nil, trace.Errorf("no signers present")
	}
	if last == nil {
		return c.signers[0], nil
	}
	for i := range c.signers {
		if last.id <= c.signers[i].id {
			continue
		}
		return c.signers[i], nil
	}
	return c.signers[0], nil
}

func (c *ActiveHealthChecker) watch(ctx context.Context) error {
	once := sync.Once{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cas, ok := <-c.c:
			if !ok {
				once.Do(func() { close(c.firstEvent) })
				return trace.Errorf("CA resource watcher closed")
			}
			c.mu.Lock()
			var signers []*healthSigner
			for _, ca := range cas {
				signers = append(signers, c.getHealthSigners(ctx, ca)...)
			}
			slices.SortStableFunc(signers, func(a, b *healthSigner) int {
				return strings.Compare(a.id, b.id)
			})
			c.signers = signers
			c.mu.Unlock()
			once.Do(func() {
				c.firstEvent <- struct{}{}
				close(c.firstEvent)
			})
		}
	}
}

func (c *ActiveHealthChecker) getHealthSigners(ctx context.Context, ca types.CertAuthority) []*healthSigner {
	var signers []*healthSigner
	ks := ca.GetActiveKeys()
	if len(ks.TLS) > 0 {
		signer, err := c.m.GetTLSSigner(ctx, ca)
		if err == nil {
			signers = append(signers, &healthSigner{
				crypto: signer,
				id:     ca.GetID().String() + "-tls",
			})
		}
	}
	if len(ks.SSH) > 0 {
		signer, err := c.m.GetSSHSigner(ctx, ca)
		if err == nil {
			signers = append(signers, &healthSigner{
				ssh: signer,
				id:  ca.GetID().String() + "-ssh",
			})
		}
	}
	if len(ks.JWT) > 0 {
		signer, err := c.m.GetJWTSigner(ctx, ca)
		if err == nil {
			signers = append(signers, &healthSigner{
				crypto: signer,
				id:     ca.GetID().String() + "-jwt",
			})
		}
	}
	return signers
}

// healthSigner wraps a crypto OR ssh signer with an ID. The ID is the CA ID plus
// a suffix to indicate the signer type of "-tls", "-ssh", "-jwt". This suffix
// is necessary to differentiate signers associated with the same CA.
type healthSigner struct {
	id     string
	crypto crypto.Signer
	ssh    ssh.Signer
}

// sign performs a signing request given a healthSigner.
func sign(s *healthSigner) error {
	msg := []byte("healthcheck")
	if s.crypto != nil {
		var (
			digest []byte
			opts   crypto.SignerOpts
		)
		switch pub := s.crypto.Public().(type) {
		case *ecdsa.PublicKey, *rsa.PublicKey:
			h := sha256.Sum256(msg)
			digest = h[:]
			opts = crypto.SHA256
		case ed25519.PublicKey:
			digest = msg
			opts = &ed25519.Options{}
		default:
			return trace.Errorf("failed signing with crypto signer: %s unexpected key type %T", s.id, pub)
		}
		_, err := s.crypto.Sign(rand.Reader, digest, opts)
		if err != nil {
			return trace.Wrap(err, "failed signing with crypto signer: %s", s.id)
		}
	} else if s.ssh != nil {
		_, err := s.ssh.Sign(rand.Reader, msg)
		if err != nil {
			return trace.Wrap(err, "failed signing with ssh signer: %s", s.id)
		}
	} else {
		return trace.Errorf("unable to test key signing: missing signer: %s", s.id)
	}
	return nil
}

type keycompare interface {
	Equal(crypto.PublicKey) bool
}

// Equal compares healthSigner a's public key to healthSigner b's public key.
func (a *healthSigner) Equal(b *healthSigner) bool {
	if a.crypto != nil && b.crypto != nil {
		kc, ok := a.crypto.Public().(keycompare)
		return ok && kc.Equal(b.crypto.Public())
	} else if a.ssh != nil && b.ssh != nil {
		return bytes.Equal(a.ssh.PublicKey().Marshal(), b.ssh.PublicKey().Marshal())
	}
	return false
}
