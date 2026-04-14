/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package identityapi

import (
	"cmp"
	"context"
	"crypto"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"

	apiclient "github.com/gravitational/teleport/api/client"
	identityapiv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identityapi/v1"
	apiidentityfile "github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	keyidentityapi "github.com/gravitational/teleport/api/utils/keys/identityapi"
	"github.com/gravitational/teleport/api/utils/keys/identityapiagent"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/cert"
)

// Service produces a single Teleport identity file backed by a local signing API.
type Service struct {
	botAuthClient             *apiclient.Client
	botIdentityReadyCh        <-chan struct{}
	defaultCredentialLifetime bot.CredentialLifetime
	cfg                       *Config
	reloadCh                  <-chan struct{}
	identityGenerator         *identity.Generator
	log                       *slog.Logger
	statusReporter            readyz.Reporter

	mu              sync.RWMutex
	currentIdentity *identity.Identity
}

func (s *Service) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("identity-api (%s)", s.cfg.Destination.String()),
	)
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.cfg.Destination.Init(ctx, []string{}); err != nil {
		return trace.Wrap(err, "initializing destination")
	}
	if err := identity.VerifyWrite(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "verifying destination")
	}

	if err := s.waitForBotIdentity(ctx); err != nil {
		return trace.Wrap(err)
	}

	initialIdentity, hostCAs, err := s.nextIdentity(ctx)
	if err != nil {
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "generating initial identity")
	}
	s.setCurrentIdentity(initialIdentity)

	server, listener, cleanup, err := s.newServer(ctx)
	if err != nil {
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "starting identity-api server")
	}
	defer cleanup()

	if err := s.writeIdentityFile(ctx, initialIdentity, hostCAs); err != nil {
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "writing initial identity file")
	}

	s.log.InfoContext(ctx, "Listener opened for identity-api signer", "addr", listener.Addr().String())

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return trace.Wrap(server.Serve(listener))
	})
	eg.Go(func() error {
		err := internal.RunOnInterval(egCtx, internal.RunOnIntervalConfig{
			Service:         s.String(),
			Name:            "identity-api-renewal",
			F:               s.generate,
			Interval:        cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
			RetryLimit:      internal.RenewalRetryLimit,
			Log:             s.log,
			ReloadCh:        s.reloadCh,
			StatusReporter:  s.statusReporter,
			IdentityReadyCh: nil,
		})
		return trace.Wrap(err)
	})
	eg.Go(func() error {
		<-egCtx.Done()
		server.Stop()
		return nil
	})

	s.statusReporter.Report(readyz.Healthy)
	return trace.Wrap(eg.Wait())
}

func (s *Service) OneShot(ctx context.Context) error {
	return trace.BadParameter("identity-api service does not support oneshot mode")
}

func (s *Service) waitForBotIdentity(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.botIdentityReadyCh:
		return nil
	}
}

func (s *Service) currentSigner() (crypto.Signer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.currentIdentity == nil || s.currentIdentity.PrivateKey == nil {
		return nil, trace.NotFound("identity-api signer is not initialized")
	}

	return s.currentIdentity.PrivateKey, nil
}

func (s *Service) currentIdentitySnapshot() *identity.Identity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentIdentity
}

func (s *Service) setCurrentIdentity(id *identity.Identity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentIdentity = id
}

func (s *Service) generate(ctx context.Context) error {
	id, hostCAs, err := s.nextIdentity(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.writeIdentityFile(ctx, id, hostCAs); err != nil {
		return trace.Wrap(err)
	}

	s.setCurrentIdentity(id)
	return nil
}

func (s *Service) nextIdentity(ctx context.Context) (*identity.Identity, []types.CertAuthority, error) {
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)

	currentIdentity := s.currentIdentitySnapshot()
	var (
		id  *identity.Identity
		err error
	)
	opts := s.generateOptions(effectiveLifetime)
	if currentIdentity != nil {
		opts = append(opts,
			identity.WithCurrentIdentity(currentIdentity),
			identity.WithPrivateKey(currentIdentity.PrivateKey),
		)
	}
	id, err = s.identityGenerator.Generate(ctx, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return id, hostCAs, nil
}

func (s *Service) generateOptions(effectiveLifetime bot.CredentialLifetime) []identity.GenerateOption {
	opts := []identity.GenerateOption{
		identity.WithRoles(s.cfg.Roles),
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithReissuableRoleImpersonation(s.cfg.AllowReissue),
		identity.WithLogger(s.log),
	}
	if s.cfg.Cluster != "" {
		opts = append(opts, identity.WithRouteToCluster(s.cfg.Cluster))
	}
	return opts
}

func (s *Service) writeIdentityFile(ctx context.Context, id *identity.Identity, hostCAs []types.CertAuthority) error {
	refBytes, err := keyidentityapi.EncodeRef(&keyidentityapi.PrivateKeyRef{
		PublicKey: id.PrivateKey.Public(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	idFile := &apiidentityfile.IdentityFile{
		PrivateKey: pem.EncodeToMemory(&pem.Block{
			Type:  keyidentityapi.PrivateKeyPEMType,
			Bytes: refBytes,
		}),
		Certs: apiidentityfile.Certs{
			SSH: id.CertBytes,
			TLS: id.TLSCertBytes,
		},
	}

	for _, ca := range authclient.AuthoritiesToTrustedCerts(hostCAs) {
		for _, publicKey := range ca.AuthorizedKeys {
			knownHost, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
				Hostname:      ca.ClusterName,
				AuthorizedKey: publicKey,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			idFile.CACerts.SSH = append(idFile.CACerts.SSH, []byte(knownHost))
		}
		idFile.CACerts.TLS = append(idFile.CACerts.TLS, ca.TLSCertificates...)
	}

	identityBytes, err := apiidentityfile.Encode(idFile)
	if err != nil {
		return trace.Wrap(err)
	}

	writer := internal.NewBotConfigWriter(ctx, s.cfg.Destination, "")
	if err := writer.WriteFile(internal.IdentityFilePath, identityBytes, apiidentityfile.FilePermissions); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) newServer(ctx context.Context) (*grpcServerWrapper, net.Listener, func(), error) {
	dirDest := s.cfg.Destination.(*destination.Directory)
	identityPath := filepath.Join(dirDest.Path, internal.IdentityFilePath)
	socketPath, certPath, err := identityapiagent.PathsFromIdentityFile(identityPath)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	listener, err := newIdentityAPIListener(ctx, socketPath, certPath)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	cert, err := generateServerCert(certPath)
	if err != nil {
		listener.Close()
		return nil, nil, nil, trace.Wrap(err)
	}

	server, err := identityapiagent.NewServer(s.currentSigner, credentials.NewServerTLSFromCert(&cert))
	if err != nil {
		listener.Close()
		return nil, nil, nil, trace.Wrap(err)
	}

	cleanup := func() {
		server.Stop()
		listener.Close()
		_ = os.Remove(socketPath)
		_ = os.Remove(certPath)
	}

	return &grpcServerWrapper{Server: server}, listener, cleanup, nil
}

type grpcServerWrapper struct {
	Server interface {
		Serve(net.Listener) error
		Stop()
	}
}

func (g *grpcServerWrapper) Serve(listener net.Listener) error { return g.Server.Serve(listener) }
func (g *grpcServerWrapper) Stop()                             { g.Server.Stop() }

func newIdentityAPIListener(ctx context.Context, socketPath, certPath string) (net.Listener, error) {
	listener, err := net.Listen("unix", socketPath)
	if err == nil {
		if chmodErr := os.Chmod(socketPath, 0o600); chmodErr != nil {
			listener.Close()
			return nil, trace.Wrap(chmodErr)
		}
		return listener, nil
	}
	if !errors.Is(err, errAddrInUse) {
		return nil, trace.Wrap(err)
	}

	creds, tlsErr := credentials.NewClientTLSFromFile(certPath, "localhost")
	if tlsErr == nil {
		client, clientErr := identityapiagent.NewClient(socketPath, creds)
		if clientErr == nil {
			pong, pingErr := client.Ping(ctx, &identityapiv1.PingRequest{})
			if pingErr == nil {
				return nil, trace.AlreadyExists("another identity-api instance is already running; PID: %d", pong.Pid)
			}
		}
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, trace.Wrap(err)
	}

	listener, err = net.Listen("unix", socketPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	return listener, nil
}

func generateServerCert(certPath string) (tls.Certificate, error) {
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil, nil, time.Now)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	f, err := os.OpenFile(certPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	defer f.Close()
	if _, err := f.Write(creds.Cert); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return keys.X509KeyPair(creds.Cert, creds.PrivateKey)
}
