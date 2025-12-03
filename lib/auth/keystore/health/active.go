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
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
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
	// The first callback will occur only after recieving a cert authority event.
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
		return nil, trace.BadParameter("signing monitor callback must be specified")
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
		clock:           clockwork.NewRealClock(),
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
// the configured callback. CAs are healthchecked one at a time at the given
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
	clock    clockwork.Clock
	logger   *slog.Logger

	// mu protects signers.
	mu      sync.RWMutex
	signers []*healthSigner
}

// Run
func (c *ActiveHealthChecker) Run(ctx context.Context) error {
	c.logger.DebugContext(ctx, "Starting active health checker")
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

	var (
		signer  *healthSigner
		failing bool
		err     error
	)
	callback := func(err error) {
		failing = err != nil
		c.callback(err)
	}
	ticker := c.clock.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		c.logger.DebugContext(ctx, "Getting CA for health checking")
		c.retry(ctx, c.failureInterval, func() bool {
			signer, err = c.nextSigner(signer)
			if err != nil {
				callback(err)
				c.logger.ErrorContext(ctx, "Failed to get next CA for health checking", "error", err)
				return false
			}
			return true
		})

		c.retry(ctx, c.failureInterval, func() bool {
			if !c.exists(signer) {
				c.logger.DebugContext(ctx, "CA has been removed from cache", "ca_id", signer.caID)
				return true
			}
			c.logger.DebugContext(ctx, "Executing health check", "ca_id", signer.caID)
			err := c.healthFn(signer)
			if err != nil {
				c.logger.DebugContext(
					ctx, "Received error from health check, waiting for next interval",
					"ca_id", signer.caID,
					"interval", c.failureInterval,
					"error", err,
				)
				callback(err)
				return false
			}
			callback(nil)
			return true
		})

		if failing {
			c.logger.DebugContext(ctx, "Last health check failed, skipping interval to get next CA")
			continue
		}
		c.logger.DebugContext(ctx, "Finished health check, waiting for next interval", "interval", c.interval)
		ticker.Reset(c.interval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.Chan():
		}
	}
}

// retry calls the given function until it returns true.
func (c *ActiveHealthChecker) retry(ctx context.Context, interval time.Duration, doneFn func() bool) {
	tick := c.clock.NewTicker(interval)
	defer tick.Stop()
	for !doneFn() {
		select {
		case <-ctx.Done():
			return
		case <-tick.Chan():
		}
	}
}

func (c *ActiveHealthChecker) exists(s *healthSigner) bool {
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
	if len(c.signers) == 0 {
		return nil, trace.Errorf("failed to health check keystore: no signers present")
	}
	for i, signer := range c.signers {
		if last == nil {
			return signer, nil
		}
		if signer.Equal(last) && len(c.signers) > i+1 {
			return c.signers[i+1], nil
		}
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
				if len(ca.GetActiveKeys().TLS) > 0 {
					tlsKey := ca.GetActiveKeys().TLS[0]
					c.logger.DebugContext(ctx, "trying get tls signer", "key", string(tlsKey.Key), "type", tlsKey.KeyType.String())
					signer, err := c.m.GetTLSSigner(ctx, ca)
					if err != nil {
						c.logger.DebugContext(ctx, "failed to get tls signer", "ca_id", ca.GetID().String(), "error", err)
						continue
					}
					signers = append(signers, &healthSigner{
						crypto: signer,
						caID:   ca.GetID().String(),
					})
				}
				if len(ca.GetActiveKeys().SSH) > 0 {
					signer, err := c.m.GetSSHSigner(ctx, ca)
					if err != nil {
						c.logger.DebugContext(ctx, "failed to get ssh signer", "ca_id", ca.GetID().String(), "error", err)
						continue
					}
					signers = append(signers, &healthSigner{
						ssh:  signer,
						caID: ca.GetID().String(),
					})
				}
				if len(ca.GetActiveKeys().JWT) > 0 {
					signer, err := c.m.GetJWTSigner(ctx, ca)
					if err != nil {
						c.logger.DebugContext(ctx, "failed to get jwt signer", "ca_id", ca.GetID().String(), "error", err)
						continue
					}
					signers = append(signers, &healthSigner{
						crypto: signer,
						caID:   ca.GetID().String(),
					})
				}

			}
			c.signers = signers
			c.logger.DebugContext(ctx, "Received CA watch event, updating CAs")
			c.mu.Unlock()
			once.Do(func() {
				c.firstEvent <- struct{}{}
				close(c.firstEvent)
			})
		}
	}
}

// healthSigner wraps a crypto OR ssh signer with the CA ID.
type healthSigner struct {
	crypto crypto.Signer
	ssh    ssh.Signer
	caID   string
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
			return trace.Errorf("failed signing with crypto signer: %s unexpected key type %T", s.caID, pub)
		}
		_, err := s.crypto.Sign(rand.Reader, digest, opts)
		if err != nil {
			return trace.Wrap(err, "failed signing with crypto signer: %s", s.caID)
		}
	} else if s.ssh != nil {
		_, err := s.ssh.Sign(rand.Reader, msg)
		if err != nil {
			return trace.Wrap(err, "failed signing with ssh signer: %s", s.caID)
		}
	} else {
		return trace.Errorf("unable to test key signing: missing signer: %s", s.caID)
	}
	return nil
}

type keycompare interface {
	Equal(crypto.PublicKey) bool
}

// Equal compares healthSigner a's public key to healthSigner b's public key.
func (a *healthSigner) Equal(b *healthSigner) bool {
	if a.crypto != nil && b.crypto != nil {
		puba := a.crypto.Public()
		pubb := b.crypto.Public()
		acomp, aok := puba.(keycompare)
		return aok && acomp.Equal(pubb)
	} else if a.ssh != nil && b.ssh != nil {
		return bytes.Equal(a.ssh.PublicKey().Marshal(), b.ssh.PublicKey().Marshal())
	}
	return false
}
