/*
Copyright 2022 Gravitational, Inc.

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

package certwatcher

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// TODO: Maybe reuse the type from service, and move this into lib/service ?
type KeyPairPath struct {
	Certificate string
	PrivateKey  string
}

type Config struct {
	KeyPairPaths []KeyPairPath
	// Watch defines whether or not the system should detect and load new
	// certificates.
	Watch bool
}

// CertWatcher watches a list of cert-key pair paths, and automatically loads
// in certificates as they change. This allows new certificates to be used
// without a full reload of Teleport.
// TODO: Refine this comment
type CertWatcher struct {
	cfg Config
	log logrus.FieldLogger

	mu           sync.RWMutex
	certificates []tls.Certificate
}

func New(log logrus.FieldLogger, cfg Config) *CertWatcher {
	return &CertWatcher{
		cfg: cfg,
		log: log,
	}
}

func (cw *CertWatcher) Start(ctx context.Context) error {
	// Synchronously load initially configured certificates
	if err := cw.loadCertificates(); err != nil {
		return trace.Wrap(err)
	}

	// Watch certificates
	if !cw.cfg.Watch {
		return nil
	}

	go func() {
		cw.log.Info("Starting to watch certificate paths for changes")
		defer func() {
			cw.log.Info("Stopped watching certificate paths for changes")
		}()

		// TODO: Replace ticker with inotify, or have both, but increase ticker
		// time either way. Low value for development currently in use.
		t := time.NewTicker(time.Second * 5)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				// TODO: Retry X times if there's a failure, the user could be
				// in the middle of copying certificates across.
				// If repeatedly fails, wait for next tick/inotify event.
				if err := cw.loadCertificates(); err != nil {
					cw.log.WithError(err).Warn("Failed to load certificates")
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (cw *CertWatcher) loadCertificates() error {
	certs := []tls.Certificate{}
	for _, pair := range cw.cfg.KeyPairPaths {
		cw.log.Infof(
			"Loading TLS certificate %v and key %v.",
			pair.Certificate, pair.PrivateKey,
		)

		certificate, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
		if err != nil {
			// TODO: Should one pair failing to load, cause all to fail to load ?
			return trace.Wrap(err)
		}
		certs = append(certs, certificate)
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.certificates = certs

	return nil
}

// GetCertificate is compatible with tls.Config.GetCertificate, allowing
// CertWatcher to be a source of certificates for a TLS listener.
func (cw *CertWatcher) GetCertificate(
	clientHello *tls.ClientHelloInfo,
) (*tls.Certificate, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	// Certificate selection logic as in crypto/tls getCertificate
	if len(cw.certificates) == 1 {
		// There's only one choice, so no point doing any work.
		return &cw.certificates[0], nil
	}

	for _, cert := range cw.certificates {
		if err := clientHello.SupportsCertificate(&cert); err == nil {
			return &cert, nil
		}
	}

	if len(cw.certificates) > 1 {
		// If no certificates are "supported", fallback to the first certificate
		return &cw.certificates[0], nil
	}

	return nil, nil
}
