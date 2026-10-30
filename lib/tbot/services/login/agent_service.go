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
	"crypto/ecdh"
	"crypto/ecdsa"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	apiclient "github.com/gravitational/teleport/api/client"
	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	loginagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginagent/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/client/credentialencryption"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/ipc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/botfs"
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

	// encryptionPivSlotKey is the PIV slot used in the fake hardware key
	// reference for the session encryption key.
	encryptionPivSlotKey = hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9D
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

	clusterName string
	privateKey  crypto.Signer

	// encryptionKey holds the bot's current session encryption key for use
	// by ECDH, kept up to date on every Login call.
	encryptionKey atomic.Pointer[ecdsa.PrivateKey]
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

	clusterName, err := s.botAuthClient.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err, "getting cluster name")
	}
	s.clusterName = clusterName.GetClusterName()

	dir, ok := s.cfg.Destination.(*destination.Directory)
	if !ok {
		return trace.BadParameter("expected destination to be directory, was: %T", s.cfg.Destination)
	}
	if err := s.verifyDirectoryPermissions(ctx, dir.Path); err != nil {
		return trace.Wrap(err)
	}

	server, err := ipc.NewServer(ctx, dir.Path,
		ipc.WithActiveSocketCheck(func(ctx context.Context, conn *grpc.ClientConn) error {
			// Use the hardware key agent's Ping RPC to check if an existing
			// socket is still active.
			_, err := hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(conn).
				Ping(ctx, hardwarekeyagentv1.PingRequest_builder{}.Build())
			return trace.Wrap(err)
		}),
	)
	if err != nil {
		return trace.Wrap(err, "creating local agent server")
	}
	defer server.Stop(ctx)
	loginagentv1.RegisterLoginAgentServiceServer(server, s)

	// As the IPC package creates the socket and certificate file directly,
	// rather than going through our destination abstraction, we need to
	// manually fix file permissions.
	//
	// 	1. AgentConfig.Init create the directory and sets its ACLs.
	// 	2. ipc.NewServer will also try to create the directory with os.MkdirAll
	// 	   but it's a no-op because the directory already exists.
	// 	3. ipc.NewServer will create the cert.pem with mode 0600, and the socket
	// 	   with net.Listen (with mode 0777 &^ umask).
	// 	4. At this point the agent is only usable by tbot user! Which is no good
	// 	   in environments like Beams where tbot runs as a different user to the
	// 	   user shell.
	// 	5. If ACLs are enabled, we configure them on cert.pem.
	// 	6. We also `chmod 777` the socket file, and rely on the directory ACLs
	// 	   and permissions to restrict access to it - this is the same as the
	// 	   internal.CreateListener method's behavior.
	if dir.ACLsEnabled() {
		//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
		if err := botfs.ConfigureACL(filepath.Join(dir.Path, ipc.CertFileName), dir.Readers); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := os.Chmod(filepath.Join(dir.Path, ipc.SocketFileName), os.ModePerm); err != nil {
		return trace.Wrap(err)
	}

	// Private key always remains in-memory, so it's okay and simpler to reuse it.
	if s.privateKey, err = cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.botAuthClient),
		cryptosuites.BotImpersonatedIdentity,
	); err != nil {
		return trace.Wrap(err, "generating private key")
	}

	hardwarekeyagent.RegisterServer(server, s, knownKey)

	s.statusReporter.Report(readyz.Healthy)
	return trace.Wrap(server.Serve(ctx))
}

// Login RPC non-interactively bootstraps a Teleport client's credentials.
func (s *AgentService) Login(ctx context.Context, req *loginagentv1.LoginRequest) (*loginagentv1.LoginResponse, error) {
	// We do not support logging in to remote/trusted clusters, but there's no
	// way for tsh to know whether the given cluster name is the root or a leaf
	// before making this RPC.
	if v := req.GetRouteToCluster(); v != "" && v != s.clusterName {
		return nil, trace.NotImplemented("login agent does not support logging in to a remote cluster")
	}

	// We also do not support scope pins, yet.
	if req.GetScope() != "" {
		return nil, trace.NotImplemented("login agent does not support scopes")
	}

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)

	opts := []identity.GenerateOption{
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithPrivateKey(s.privateKey),
	}

	if v := req.GetKubernetesCluster(); v != "" {
		opts = append(opts, identity.WithKubernetesCluster(v))
	}

	identity, err := s.identityGenerator.Generate(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostCA, err := s.botAuthClient.GetCertAuthority(
		ctx,
		types.CertAuthID{
			Type:        types.HostCA,
			ClusterName: s.clusterName,
		},
		false, /* loadSecrets */
	)
	if err != nil {
		return nil, trace.Wrap(err, "getting host CA certificate")
	}
	trustedCerts := []*loginagentv1.TrustedCerts{
		loginagentv1.TrustedCerts_builder{
			ClusterName:       hostCA.GetClusterName(),
			SshAuthorizedKeys: services.GetSSHCheckingKeys(hostCA),
			TlsCaCerts:        services.GetTLSCerts(hostCA),
		}.Build(),
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

	s.encryptionKey.Store(identity.EncryptionPrivateKey)
	var encryptionKeyPEM []byte
	if identity.EncryptionPrivateKey != nil && identity.TLSIdentity.EncryptionKeyID != "" {
		encryptionKeyPEM, err = s.marshalEncryptionKeyRef(identity.EncryptionPrivateKey)
		if err != nil {
			return nil, trace.Wrap(err, "encoding session encryption key reference")
		}
	}

	return loginagentv1.LoginResponse_builder{
		Username:      identity.TLSIdentity.Username,
		SshCert:       identity.CertBytes,
		TlsCert:       identity.TLSCertBytes,
		PrivateKey:    privateKey,
		HostSigners:   trustedCerts,
		EncryptionKey: encryptionKeyPEM,
	}.Build(), nil
}

func (s *AgentService) marshalEncryptionKeyRef(key *ecdsa.PrivateKey) ([]byte, error) {
	encryptionSlotKey, err := hardwarekey.PIVSlotKeyFromProto(encryptionPivSlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	exchanger, err := hardwarekey.NewKeyExchanger(&hardwarekey.PrivateKeyRef{
		SerialNumber:         pivSerialNumber,
		SlotKey:              encryptionSlotKey,
		PublicKey:            &key.PublicKey,
		Policy:               hardwarekey.PromptPolicyNone,
		AttestationStatement: &hardwarekey.AttestationStatement{},
	}, s)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	encryptionKey, err := credentialencryption.NewPrivateKey(exchanger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return credentialencryption.MarshalPrivateKey(encryptionKey)
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

// ECDH satisfies hardwarekey.KeyExchangerService.
func (s *AgentService) ECDH(_ context.Context, ref *hardwarekey.PrivateKeyRef, remote *ecdh.PublicKey) ([]byte, error) {
	key := s.encryptionKey.Load()
	if key == nil || !key.PublicKey.Equal(ref.PublicKey) {
		return nil, trace.NotFound("unknown session encryption key")
	}
	ecdhKey, err := key.ECDH()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sharedSecret, err := ecdhKey.ECDH(remote)
	return sharedSecret, trace.Wrap(err)
}

// knownKey satisfies hardwarekeyagent.KnownHardwareKeyFn, recognizing both
// the fake signing key ref and the session encryption key ref.
func knownKey(ref *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo) (bool, error) {
	if ref.SerialNumber != pivSerialNumber {
		return false, nil
	}

	slotKey, err := hardwarekey.PIVSlotKeyFromProto(pivSlotKey)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if ref.SlotKey == slotKey {
		return true, nil
	}

	encryptionSlotKey, err := hardwarekey.PIVSlotKeyFromProto(encryptionPivSlotKey)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return ref.SlotKey == encryptionSlotKey, nil
}
