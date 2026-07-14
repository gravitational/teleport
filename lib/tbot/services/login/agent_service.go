// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package login

import (
	"cmp"
	"context"
	"crypto"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	loginagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginagent/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
	"github.com/gravitational/teleport/lib/localagent"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

const (
	// pivSerialNumber is the serial number used in the fake hardware key
	// reference we return from the login agent service.
	pivSerialNumber uint32 = 0xFFFFFFFF

	// pivSlotKey is the PIV slot used in the fake hardware key reference we
	// return from the login agent service.
	pivSlotKey = hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9A
)

// AgentServiceBuilder returns a builder for the login agent service.
func AgentServiceBuilder(cfg *AgentConfig, opts ...AgentOpt) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &AgentService{
			cfg:                       cfg,
			defaultCredentialLifetime: bot.DefaultCredentialLifetime,
			botAuthClient:             deps.Client,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			identityGenerator:         deps.IdentityGenerator,
			statusReporter:            deps.GetStatusReporter(),
			logger:                    deps.Logger,
		}
		for _, optFn := range opts {
			optFn(svc)
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(
		AgentServiceType,
		cfg.GetName(),
		buildFn,
	)
}

// AgentService implements a "login agent" for tsh to non-interactively bootstrap
// its identity from tbot.
type AgentService struct {
	loginagentv1.UnimplementedLoginAgentServiceServer

	cfg                       *AgentConfig
	defaultCredentialLifetime bot.CredentialLifetime
	identityGenerator         *identity.Generator
	botIdentityReadyCh        <-chan struct{}
	statusReporter            readyz.Reporter
	botAuthClient             *apiclient.Client
	logger                    *slog.Logger

	privateKey crypto.Signer
}

// String satisfies fmt.Stringer.
func (s *AgentService) String() string { return s.cfg.GetName() }

// Run the service until the given context is canceled.
func (s *AgentService) Run(ctx context.Context) error {
	select {
	case <-s.botIdentityReadyCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	dir, ok := s.cfg.Destination.(*destination.Directory)
	if !ok {
		return trace.BadParameter("expected destination to be directory, was: %T", s.cfg.Destination)
	}

	server, err := localagent.NewServer(dir.Path)
	if err != nil {
		return trace.Wrap(err, "creating local agent server")
	}
	defer server.Stop(ctx)
	loginagentv1.RegisterLoginAgentServiceServer(server, s)

	// Private key always remains in-memory, so it's okay and simpler to reuse it.
	if s.privateKey, err = cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.botAuthClient),
		cryptosuites.BotImpersonatedIdentity,
	); err != nil {
		return trace.Wrap(err, "generating private key")
	}

	knownKey := func(ref *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo) (bool, error) {
		slotKey, err := hardwarekey.PIVSlotKeyFromProto(pivSlotKey)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return ref.SerialNumber == pivSerialNumber && ref.SlotKey == slotKey, nil
	}
	if _, err = libhwk.NewAgentServerWithLocalAgent(
		ctx,
		server,
		s,
		knownKey,
	); err != nil {
		return trace.Wrap(err)
	}

	s.statusReporter.Report(readyz.Healthy)
	return trace.Wrap(server.Serve(ctx))
}

// Login RPC non-interactively bootstraps a Teleport client's credentials.
func (s *AgentService) Login(ctx context.Context, req *loginagentv1.LoginRequest) (*loginagentv1.LoginResponse, error) {
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)

	opts := []identity.GenerateOption{
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithPrivateKey(s.privateKey),
	}

	if v := req.GetRouteToCluster(); v != "" {
		opts = append(opts, identity.WithRouteToCluster(v))
	}
	if v := req.GetKubernetesCluster(); v != "" {
		opts = append(opts, identity.WithKubernetesCluster(v))
	}

	identity, err := s.identityGenerator.Generate(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err, "getting host CA certificates")
	}

	trustedCerts := make([]*loginagentv1.TrustedCerts, len(hostCAs))
	for idx, cert := range hostCAs {
		trustedCerts[idx] = loginagentv1.TrustedCerts_builder{
			ClusterName:       cert.GetClusterName(),
			SshAuthorizedKeys: services.GetSSHCheckingKeys(cert),
			TlsCaCerts:        services.GetTLSCerts(cert),
		}.Build()
	}

	slotKey, err := hardwarekey.PIVSlotKeyFromProto(pivSlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := keys.MarshalPrivateKey(&hardwarekey.Signer{
		Ref: &hardwarekey.PrivateKeyRef{
			SerialNumber:         pivSerialNumber,
			SlotKey:              slotKey,
			PublicKey:            s.privateKey.Public(),
			Policy:               hardwarekey.PromptPolicyNone,
			AttestationStatement: &hardwarekey.AttestationStatement{},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "encoding private key reference")
	}

	return loginagentv1.LoginResponse_builder{
		Username:    identity.TLSIdentity.Username,
		SshCert:     identity.CertBytes,
		TlsCert:     identity.TLSCertBytes,
		PrivateKey:  privateKey,
		HostSigners: trustedCerts,
	}.Build(), nil
}

// Sign satisfies hardwarekey.Service.
func (s *AgentService) Sign(
	_ context.Context,
	keyRef *hardwarekey.PrivateKeyRef,
	keyInfo hardwarekey.ContextualKeyInfo,
	rand io.Reader,
	digest []byte,
	opts crypto.SignerOpts,
) ([]byte, error) {
	if keyInfo.AgentKeyInfo.UnknownAgentKey {
		return nil, trace.BadParameter(
			"refusing to sign for unknown key (serial_number=%d, piv_slot=0x%x)",
			keyRef.SerialNumber,
			keyRef.SlotKey,
		)
	}
	signature, err := s.privateKey.Sign(rand, digest, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signature, nil
}

// NewPrivateKey satisfies hardwarekey.Service.
func (*AgentService) NewPrivateKey(context.Context, hardwarekey.PrivateKeyConfig) (*hardwarekey.Signer, error) {
	// This method shouldn't be called because, when using the login agent, the
	// client does not manage its own private key.
	return nil, trace.NotImplemented("generating new private keys is not supported")
}

// GetFullKeyRef satisfies hardwarekey.Service.
func (*AgentService) GetFullKeyRef(uint32, hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	// This method is marked for deletion in v19.
	return nil, trace.NotImplemented("GetFullKeyRef is not implemented")
}
