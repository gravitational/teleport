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

package identity

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/utils"
)

func OutputServiceBuilder(
	cfg *OutputConfig,
	alpnUpgradeCache *internal.ALPNUpgradeCache,
	defaultCredentialLifetime bot.CredentialLifetime,
	insecure, fips bool,
) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &OutputService{
			botAuthClient:             deps.Client,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			defaultCredentialLifetime: defaultCredentialLifetime,
			insecure:                  insecure,
			fips:                      fips,
			cfg:                       cfg,
			reloadCh:                  deps.ReloadCh,
			executablePath:            autoupdate.StableExecutable,
			alpnUpgradeCache:          alpnUpgradeCache,
			proxyPinger:               deps.ProxyPinger,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
			log:                       deps.Logger,
			statusReporter:            deps.GetStatusReporter(),
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(OutputServiceType, cfg.Name, buildFn)
}

// OutputService produces credentials which can be used to connect to
// Teleport's API or SSH.
type OutputService struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient             *apiclient.Client
	botIdentityReadyCh        <-chan struct{}
	defaultCredentialLifetime bot.CredentialLifetime
	insecure, fips            bool
	cfg                       *OutputConfig
	log                       *slog.Logger
	proxyPinger               connection.ProxyPinger
	statusReporter            readyz.Reporter
	reloadCh                  <-chan struct{}
	// executablePath is called to get the path to the tbot executable.
	// Usually this is os.Executable
	executablePath    func() (string, error)
	alpnUpgradeCache  *internal.ALPNUpgradeCache
	identityGenerator *identity.Generator
	clientBuilder     *client.Builder
}

func (s *OutputService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("identity-output (%s)", s.cfg.Destination.String()),
	)
}

func (s *OutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *OutputService) Run(ctx context.Context) error {
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

func (s *OutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputService/generate",
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
	identityOpts := []identity.GenerateOption{
		identity.WithRoles(s.cfg.Roles),
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithReissuableRoleImpersonation(s.cfg.AllowReissue),
		identity.WithLogger(s.log),
	}
	id, err := s.identityGenerator.GenerateFacade(ctx, identityOpts...)
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

	if s.cfg.Cluster != "" {
		id, err = s.identityGenerator.GenerateFacade(ctx, append(identityOpts,
			identity.WithCurrentIdentityFacade(id),
			identity.WithRouteToCluster(s.cfg.Cluster),
		)...)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	userCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	databaseCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.DatabaseCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.render(ctx, id.Get(), hostCAs, userCAs, databaseCAs); err != nil {
		return trace.Wrap(err)
	}

	if s.cfg.SSHConfigMode == SSHConfigModeOn {
		clusterNames, err := internal.GetClusterNames(ctx, impersonatedClient, id.Get().ClusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		proxyPing, err := s.proxyPinger.Ping(ctx)
		if err != nil {
			return trace.Wrap(err, "pinging proxy")
		}
		if err := renderSSHConfig(
			ctx,
			s.log,
			proxyPing,
			clusterNames,
			s.cfg.Destination,
			s.botAuthClient,
			s.executablePath,
			s.alpnUpgradeCache,
			s.insecure,
			s.fips,
		); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *OutputService) render(
	ctx context.Context,
	id *identity.Identity,
	hostCAs, userCAs, databaseCAs []types.CertAuthority,
) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputService/render",
	)
	defer span.End()

	keyRing, err := internal.NewClientKeyRing(id, hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := internal.WriteTLSCAs(ctx, s.cfg.Destination, hostCAs, userCAs, databaseCAs); err != nil {
		return trace.Wrap(err)
	}

	if err := internal.WriteIdentityFile(ctx, s.log, keyRing, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "writing identity file")
	}
	if err := identity.SaveIdentity(
		ctx, id, s.cfg.Destination, identity.DestinationKinds()...,
	); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	return nil
}

type certAuthGetter interface {
	GetCertAuthority(
		ctx context.Context,
		id types.CertAuthID,
		includeSigningKeys bool,
	) (types.CertAuthority, error)
}

type alpnTester interface {
	IsUpgradeRequired(ctx context.Context, addr string, insecure bool) (bool, error)
}

func renderSSHConfig(
	ctx context.Context,
	log *slog.Logger,
	proxyPing *connection.ProxyPong,
	clusterNames []string,
	dest destination.Destination,
	certAuthGetter certAuthGetter,
	getExecutablePath func() (string, error),
	alpnTester alpnTester,
	insecure, fips bool,
) error {
	ctx, span := tracer.Start(
		ctx,
		"renderSSHConfig",
	)
	defer span.End()

	proxyAddr, err := proxyPing.ProxySSHAddr()
	if err != nil {
		return trace.Wrap(err, "determining proxy ssh addr")
	}
	proxyHost, proxyPort, err := utils.SplitHostPort(proxyAddr)
	if err != nil {
		return trace.BadParameter(
			"proxy %+v has no usable public address: %v",
			proxyAddr, err,
		)
	}

	// We'll write known_hosts regardless of Destination type, it's still
	// useful alongside a manually-written ssh_config.
	knownHosts, clusterKnownHosts, err := ssh.GenerateKnownHosts(
		ctx,
		certAuthGetter,
		clusterNames,
		proxyHost,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(
		ctx, ssh.KnownHostsName, []byte(knownHosts),
	); err != nil {
		return trace.Wrap(err)
	}

	// We only want to proceed further if we have a directory destination
	destDirectory, ok := dest.(*destination.Directory)
	if !ok {
		return nil
	}

	// Backend note: Prefer to use absolute paths for filesystem backends.
	// If the backend is something else, use "". ssh_config will generate with
	// paths relative to the Destination. This doesn't work with ssh in
	// practice so adjusting the config for impossible-to-determine-in-advance
	// Destination backends is left as an exercise to the user.
	absDestPath, err := filepath.Abs(destDirectory.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	executablePath, err := getExecutablePath()
	if errors.Is(err, autoupdate.ErrUnstableExecutable) {
		log.WarnContext(ctx, "ssh_config will be created with an unstable path to the tbot executable. Please reinstall tbot with Managed Updates to prevent instability.")
	} else if err != nil {
		return trace.Wrap(err)
	}

	var sshConfigBuilder strings.Builder
	knownHostsPath := filepath.Join(absDestPath, ssh.KnownHostsName)
	identityFilePath := filepath.Join(absDestPath, identity.PrivateKeyKey)
	certificateFilePath := filepath.Join(absDestPath, identity.SSHCertKey)

	// Test if ALPN upgrade is required, this will only be necessary if we
	// are using TLS routing.
	connUpgradeRequired := false
	if proxyPing.Proxy.TLSRoutingEnabled {
		connUpgradeRequired, err = alpnTester.IsUpgradeRequired(
			ctx, proxyAddr, insecure,
		)
		if err != nil {
			return trace.Wrap(err, "determining if ALPN upgrade is required")
		}
	}

	// Generate SSH config
	if err := openssh.WriteSSHConfig(&sshConfigBuilder, &openssh.SSHConfigParameters{
		AppName:             openssh.TbotApp,
		ClusterNames:        clusterNames,
		KnownHostsPath:      knownHostsPath,
		IdentityFilePath:    identityFilePath,
		CertificateFilePath: certificateFilePath,
		ProxyHost:           proxyHost,
		ProxyPort:           proxyPort,
		ExecutablePath:      executablePath,
		DestinationDir:      absDestPath,

		PureTBotProxyCommand: true,
		Insecure:             insecure,
		FIPS:                 fips,
		TLSRouting:           proxyPing.Proxy.TLSRoutingEnabled,
		ConnectionUpgrade:    connUpgradeRequired,
		// Session resumption is enabled by default, this can be
		// configurable at a later date if we discover reasons for this to
		// be disabled.
		Resume: true,
	}); err != nil {
		return trace.Wrap(err)
	}

	// Generate the per cluster files
	for _, clusterName := range clusterNames {
		sshConfigName := fmt.Sprintf("%s.%s", clusterName, ssh.ConfigName)
		knownHostsName := fmt.Sprintf("%s.%s", clusterName, ssh.KnownHostsName)
		knownHostsPath := filepath.Join(absDestPath, knownHostsName)

		sb := &strings.Builder{}
		if err := openssh.WriteClusterSSHConfig(sb, &openssh.ClusterSSHConfigParameters{
			AppName:             openssh.TbotApp,
			ClusterName:         clusterName,
			KnownHostsPath:      knownHostsPath,
			IdentityFilePath:    identityFilePath,
			CertificateFilePath: certificateFilePath,
			ProxyHost:           proxyHost,
			ProxyPort:           proxyPort,
			ExecutablePath:      executablePath,
			DestinationDir:      absDestPath,

			Insecure:          insecure,
			FIPS:              fips,
			TLSRouting:        proxyPing.Proxy.TLSRoutingEnabled,
			ConnectionUpgrade: connUpgradeRequired,
			// Session resumption is enabled by default, this can be
			// configurable at a later date if we discover reasons for this to
			// be disabled.
			Resume: true,
		}); err != nil {
			return trace.Wrap(err)
		}
		if err := destDirectory.Write(ctx, sshConfigName, []byte(sb.String())); err != nil {
			return trace.Wrap(err)
		}

		knownHosts, ok := clusterKnownHosts[clusterName]
		if !ok {
			log.WarnContext(
				ctx,
				"No generated known_hosts for cluster, will skip",
				"cluster", clusterName,
			)
			continue
		}
		if err := destDirectory.Write(ctx, knownHostsName, []byte(knownHosts)); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := destDirectory.Write(ctx, ssh.ConfigName, []byte(sshConfigBuilder.String())); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
