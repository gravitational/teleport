/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

type SSHHostOutputService struct {
	botAuthClient     *authclient.Client
	botCfg            *config.BotConfig
	cfg               *config.SSHHostOutput
	getBotIdentity    getBotIdentityFn
	log               *slog.Logger
	reloadBroadcaster *channelBroadcaster
	resolver          reversetunnelclient.Resolver
}

func (s *SSHHostOutputService) String() string {
	return fmt.Sprintf("ssh-host (%s)", s.cfg.Destination.String())
}

func (s *SSHHostOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *SSHHostOutputService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		service:    s.String(),
		name:       "output-renewal",
		f:          s.generate,
		interval:   cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).RenewalInterval,
		retryLimit: renewalRetryLimit,
		log:        s.log,
		reloadCh:   reloadCh,
	})
	return trace.Wrap(err)
}

func (s *SSHHostOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"SSHHostOutputService/generate",
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

	var err error
	roles := s.cfg.Roles
	if len(roles) == 0 {
		roles, err = fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err, "fetching default roles")
		}
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()
	clusterName := facade.Get().ClusterName

	// generate a keypair
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.botAuthClient),
		cryptosuites.HostSSH)
	if err != nil {
		return trace.Wrap(err)
	}
	privKey, err := keys.NewSoftwarePrivateKey(key)
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
		Ttl:         durationpb.New(cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL),
	},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	keyRing := &client.KeyRing{
		SSHPrivateKey: privKey,
		Cert:          res.SshCertificate,
	}

	cfg := identityfile.WriteConfig{
		OutputPath: config.SSHHostCertPath,
		Writer:     newBotConfigWriter(ctx, s.cfg.Destination, ""),
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

	userCAPath := config.SSHHostCertPath + config.SSHHostUserCASuffix
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
