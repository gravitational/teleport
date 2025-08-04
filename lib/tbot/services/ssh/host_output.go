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

package ssh

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	apiclient "github.com/gravitational/teleport/api/client"
	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func HostOutputServiceBuilder(cfg *HostOutputConfig, defaultCredentialLifetime bot.CredentialLifetime) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &HostOutputService{
			botAuthClient:             deps.Client,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			defaultCredentialLifetime: defaultCredentialLifetime,
			cfg:                       cfg,
			reloadCh:                  deps.ReloadCh,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
		}
		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

type HostOutputService struct {
	defaultCredentialLifetime bot.CredentialLifetime
	botAuthClient             *apiclient.Client
	botIdentityReadyCh        <-chan struct{}
	cfg                       *HostOutputConfig
	log                       *slog.Logger
	statusReporter            readyz.Reporter
	reloadCh                  <-chan struct{}
	identityGenerator         *identity.Generator
	clientBuilder             *client.Builder
}

func (s *HostOutputService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("ssh-host (%s)", s.cfg.Destination.String()),
	)
}

func (s *HostOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *HostOutputService) Run(ctx context.Context) error {
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service:         s.String(),
		Name:            "output-renewal",
		F:               s.generate,
		Interval:        cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
		StatusReporter:  s.statusReporter,
	})
	return trace.Wrap(err)
}

func (s *HostOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"HostOutputService/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	// Check the ACLs. We can't fix them, but we can warn if they're
	// misconfigured. We'll need to precompute a list of keys to check.
	// Note: This may only log a warning, depending on configuration.
	if err := s.cfg.Destination.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
		return trace.Wrap(err)
	}
	// Ensure this destination is also writable. This is a hard fail if
	// ACLs are misconfigured, regardless of configuration.
	if err := identity.VerifyWrite(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "verifying destination")
	}

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	id, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithRoles(s.cfg.Roles),
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}

	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()
	clusterName := id.Get().ClusterName

	// generate a keypair
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.botAuthClient),
		cryptosuites.HostSSH)
	if err != nil {
		return trace.Wrap(err)
	}
	privKey, err := keys.NewPrivateKey(key)
	if err != nil {
		return trace.Wrap(err)
	}
	// For now, we'll reuse the bot's regular TTL, and hostID and nodeName are
	// left unset.
	res, err := impersonatedClient.TrustClient().GenerateHostCert(ctx, &trustpb.GenerateHostCertRequest{
		Key:         privKey.MarshalSSHPublicKey(),
		HostId:      "",
		NodeName:    "",
		Principals:  s.cfg.Principals,
		ClusterName: clusterName,
		Role:        string(types.RoleNode),
		Ttl:         durationpb.New(cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).TTL),
	},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	keyRing := &libclient.KeyRing{
		SSHPrivateKey: privKey,
		Cert:          res.SshCertificate,
	}

	cfg := identityfile.WriteConfig{
		OutputPath: SSHHostCertPath,
		Writer:     internal.NewBotConfigWriter(ctx, s.cfg.Destination, ""),
		KeyRing:    keyRing,
		Format:     identityfile.FormatOpenSSH,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	userCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	exportedCAs, err := exportSSHUserCAs(userCAs, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	userCAPath := SSHHostCertPath + SSHHostUserCASuffix
	if err := s.cfg.Destination.Write(ctx, userCAPath, []byte(exportedCAs)); err != nil {
		return trace.Wrap(err)
	}

	files = append(files, userCAPath)

	s.log.DebugContext(
		ctx,
		"Wrote OpenSSH host cert files",
		"files", files,
	)

	return nil
}

const (
	// sshHostTrimPrefix is the prefix that should be removed from the generated
	// SSH CA.
	sshHostTrimPrefix = "cert-authority "
)

// exportSSHUserCAs generates SSH CAs.
func exportSSHUserCAs(cas []types.CertAuthority, localAuthName string) (string, error) {
	var exported string

	for _, ca := range cas {
		// Don't export trusted CAs.
		if ca.GetClusterName() != localAuthName {
			continue
		}

		for _, key := range ca.GetTrustedSSHKeyPairs() {
			s, err := sshutils.MarshalAuthorizedKeysFormat(ca.GetClusterName(), key.PublicKey)
			if err != nil {
				return "", trace.Wrap(err)
			}

			// remove "cert-authority "
			s = strings.TrimPrefix(s, sshHostTrimPrefix)

			exported += s
		}
	}

	return exported, nil
}
