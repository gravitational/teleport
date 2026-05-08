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

package identity

import (
	"cmp"
	"context"
	"crypto"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apiclient "github.com/gravitational/teleport/api/client"
	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

// pivSerialNumber is the serial number used in the fake hardware key reference
// we encode in the identity file, for the key agent service.
const pivSerialNumber uint32 = 0xFFFFFFFF

// pivSlotKey is the PIV slot used in the fake hardware key reference we encode
// in the identity file, for the key agent service.
var pivSlotKey = func() hardwarekey.PIVSlotKey {
	key, err := hardwarekey.PIVSlotKeyFromProto(hardwarekeyagentv1.PIVSlotKey_PIV_SLOT_KEY_9A)
	if err != nil {
		panic(err) // This should never fail.
	}
	return key
}()

// KeyAgentServiceBuilder returns a service builder for the key agent service.
func KeyAgentServiceBuilder(cfg *KeyAgentConfig, opts ...KeyAgentOpt) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &KeyAgentService{
			cfg:                       cfg,
			botAuthClient:             deps.Client,
			identityGenerator:         deps.IdentityGenerator,
			reloadCh:                  deps.ReloadCh,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			statusReporter:            deps.GetStatusReporter(),
			logger:                    deps.Logger,
			defaultCredentialLifetime: bot.DefaultCredentialLifetime,
		}
		for _, fn := range opts {
			fn(svc)
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(KeyAgentServiceType, cfg.Name, buildFn)
}

// KeyAgentService allows you to generate an identity file *without* private key
// material, in environments where exfiltration of the private key is a concern,
// such as Beams.
//
// It works by implementing the same gRPC API as the Hardware Key Agent. If you
// configure the service's destination to `$TMPDIR/.Teleport-PIV`, tsh will
// seamlessly forward signing requests to tbot, as if the user had a YubiKey,
// without any client-side configuration.
//
// The generated identity file will contain a nonsensical hardware key reference
// with a fixed serial number and PIV slot.
type KeyAgentService struct {
	cfg                       *KeyAgentConfig
	botAuthClient             *apiclient.Client
	identityGenerator         *identity.Generator
	defaultCredentialLifetime bot.CredentialLifetime
	reloadCh                  <-chan struct{}
	botIdentityReadyCh        <-chan struct{}
	statusReporter            readyz.Reporter
	logger                    *slog.Logger
}

// Run the key agent until the given context is canceled.
func (s *KeyAgentService) Run(ctx context.Context) error {
	select {
	case <-s.botIdentityReadyCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	identity, err := s.renewIdentity(ctx, nil)
	if err != nil {
		return trace.Wrap(err, "generating initial identity")
	}

	// We re-use the private key between renewals so we don't have to worry
	// about temporary mismatches between the public key in the identity file
	// and the private key in-memory.
	privKey := identity.PrivateKey.Signer

	dir, ok := s.cfg.Destination.(*destination.Directory)
	if !ok {
		return trace.BadParameter("destination must be a directory")
	}
	knownKey := func(ref *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo) (bool, error) {
		return ref.SerialNumber == pivSerialNumber && ref.SlotKey == pivSlotKey, nil
	}
	hwks, err := libhwk.NewAgentServer(
		ctx,
		&hardwareKeyService{privateKey: privKey},
		dir.Path,
		knownKey,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// NewAgentServer creates the directory so we hold off on writing the
	// identity file until it's in-place.
	if err := s.writeIdentityFile(ctx, identity); err != nil {
		return trace.Wrap(err)
	}
	s.statusReporter.Report(readyz.Healthy)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return hwks.Serve(groupCtx)
	})
	group.Go(func() error {
		return s.identityRenewalLoop(groupCtx, privKey)
	})
	return trace.Wrap(group.Wait())
}

func (s *KeyAgentService) identityRenewalLoop(ctx context.Context, privKey crypto.Signer) error {
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service: s.String(),
		Name:    "identity-renewal",
		F: func(ctx context.Context) error {
			identity, err := s.renewIdentity(ctx, privKey)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := s.writeIdentityFile(ctx, identity); err != nil {
				return trace.Wrap(err)
			}
			return nil
		},
		Interval:           cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
		RetryLimit:         internal.RenewalRetryLimit,
		Log:                s.logger,
		ReloadCh:           s.reloadCh,
		StatusReporter:     s.statusReporter,
		WaitBeforeFirstRun: true,
	})
	return trace.Wrap(err, "running identity renewal loop")
}

func (s *KeyAgentService) renewIdentity(ctx context.Context, privKey crypto.Signer) (*identity.Identity, error) {
	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)

	generateOpts := []identity.GenerateOption{
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithReissuableRoleImpersonation(s.cfg.AllowReissue),
		identity.WithLogger(s.logger),
	}

	if privKey != nil {
		generateOpts = append(generateOpts, identity.WithPrivateKey(privKey))
	}

	if s.cfg.DelegationSessionID == "" {
		generateOpts = append(generateOpts, identity.WithRoles(s.cfg.Roles))
	} else {
		generateOpts = append(generateOpts, identity.WithDelegation(s.cfg.DelegationSessionID))
	}

	identity, err := s.identityGenerator.Generate(ctx, generateOpts...)
	if err != nil {
		return nil, trace.Wrap(err, "generating identity")
	}
	return identity, nil
}

func (s *KeyAgentService) writeIdentityFile(ctx context.Context, identity *identity.Identity) error {
	privateKey, err := keys.NewPrivateKey(&hardwarekey.Signer{
		Ref: &hardwarekey.PrivateKeyRef{
			SerialNumber:         pivSerialNumber,
			SlotKey:              pivSlotKey,
			PublicKey:            identity.X509Cert.PublicKey, // X509Cert.PublicKey and SSHCert.PublicKey are the same.
			Policy:               hardwarekey.PromptPolicyNone,
			AttestationStatement: &hardwarekey.AttestationStatement{},
		},
	})
	if err != nil {
		return trace.Wrap(err, "building pseudo hardware key")
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err, "getting host CA certificates")
	}

	keyRing, err := internal.NewClientKeyRing(identity, hostCAs)
	if err != nil {
		return trace.Wrap(err, "building client key ring")
	}
	keyRing.TLSPrivateKey = privateKey
	keyRing.SSHPrivateKey = privateKey

	if err := internal.WriteIdentityFile(
		ctx,
		s.logger,
		keyRing,
		s.cfg.Destination,
	); err != nil {
		return trace.Wrap(err, "writing identity file")
	}

	return nil
}

// String satisfies the bot.Service interface.
func (s *KeyAgentService) String() string { return s.cfg.Name }

type hardwareKeyService struct{ privateKey crypto.Signer }

func (s *hardwareKeyService) Sign(
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

func (*hardwareKeyService) NewPrivateKey(context.Context, hardwarekey.PrivateKeyConfig) (*hardwarekey.Signer, error) {
	// This method shouldn't be called because tsh explicitly bypasses the
	// Hardware Key Agent during login.
	return nil, trace.NotImplemented("generating new private keys is not supported")
}

func (*hardwareKeyService) GetFullKeyRef(uint32, hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	// This method is marked for deletion in v19.
	return nil, trace.NotImplemented("GetFullKeyRef is not implemented")
}
