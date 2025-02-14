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
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/teleport/lib/utils"
)

// IdentityOutputService produces credentials which can be used to connect to
// Teleport's API or SSH.
type IdentityOutputService struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient     *authclient.Client
	botCfg            *config.BotConfig
	cfg               *config.IdentityOutput
	getBotIdentity    getBotIdentityFn
	log               *slog.Logger
	proxyPingCache    *proxyPingCache
	reloadBroadcaster *channelBroadcaster
	resolver          reversetunnelclient.Resolver
	// executablePath is called to get the path to the tbot executable.
	// Usually this is os.Executable
	executablePath   func() (string, error)
	alpnUpgradeCache *alpnProxyConnUpgradeRequiredCache
}

func (s *IdentityOutputService) String() string {
	return fmt.Sprintf("identity-output (%s)", s.cfg.Destination.String())
}

func (s *IdentityOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *IdentityOutputService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		name:       "output-renewal",
		f:          s.generate,
		interval:   s.botCfg.RenewalInterval,
		retryLimit: renewalRetryLimit,
		log:        s.log,
		reloadCh:   reloadCh,
	})
	return trace.Wrap(err)
}

func createAccessRequest(botUser string, req *config.AccessRequest) (types.AccessRequest, error) {
	// TODO: support resource IDs
	r, err := services.NewAccessRequestWithResources(botUser, req.Roles, []types.ResourceID{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Reason != "" {
		r.SetRequestReason(req.Reason)
	}
	if len(req.Reviewers) > 0 {
		r.SetSuggestedReviewers(req.Reviewers)
	}

	// TODO: TTLs, max duration, AssumeStartTime, resource IDs, etc

	return r, nil
}

func (s *IdentityOutputService) awaitRequestResolution(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	filter := types.AccessRequestFilter{
		User: req.GetUser(),
		ID:   req.GetName(),
	}
	watcher, err := s.botAuthClient.NewWatcher(ctx, types.Watch{
		Name: "bot-await-request-approval",
		Kinds: []types.WatchKind{{
			Kind:   types.KindAccessRequest,
			Filter: filter.IntoMap(),
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer watcher.Close()

	// Wait for OpInit event so that returned watcher is ready.
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("failed to watch for access requests: received an unexpected event while waiting for the initial OpInit")
		}
	case <-watcher.Done():
		return nil, trace.Wrap(watcher.Error())
	}

	// get initial state of request
	reqState, err := services.GetAccessRequest(ctx, s.botAuthClient, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for {
		if !reqState.GetState().IsPending() {
			return reqState, nil
		}

		select {
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				var ok bool
				reqState, ok = event.Resource.(*types.AccessRequestV3)
				if !ok {
					return nil, trace.BadParameter("unexpected resource type %T", event.Resource)
				}
			case types.OpDelete:
				return nil, trace.Errorf("request %s has expired or been deleted...", event.Resource.GetName())
			default:
				s.log.WarnContext(ctx, "Skipping unknown event type", "event_type", event.Type)
			}
		case <-watcher.Done():
			return nil, trace.Wrap(watcher.Error())
		}
	}
}

// executeAccessRequest creates an access request and waits for resolution,
// returning either an error or a completed ID if the request was approved.
func (s *IdentityOutputService) executeAccessRequest(ctx context.Context) (string, error) {
	request, err := createAccessRequest(s.getBotIdentity().TLSIdentity.Username, s.cfg.AccessRequest)
	if err != nil {
		return "", trace.Wrap(err)
	}

	created, err := s.botAuthClient.CreateAccessRequestV2(ctx, request)
	if err != nil {
		return "", trace.Wrap(err)
	}

	s.log.InfoContext(ctx, "Created access request, waiting for approval...", "request_id", created.GetName())

	completed, err := s.awaitRequestResolution(ctx, created)
	if err != nil {
		return "", trace.Wrap(err)
	}

	s.log.InfoContext(ctx, "Access request approved, continuing", "request_id", completed.GetName(), "approval_reason", completed.GetResolveReason())

	return completed.GetName(), nil
}

func (s *IdentityOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"IdentityOutputService/generate",
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

	var accessRequests []string
	roles := s.cfg.Roles

	if s.cfg.AccessRequest != nil {
		id, err := s.executeAccessRequest(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		accessRequests = append(accessRequests, id)
	} else if len(roles) == 0 {
		// TODO: sloppy handling of roles + access requests, which cannot be
		// combined.
		roles, err = fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err, "fetching default roles")
		}
	}

	s.log.InfoContext(ctx, "generating identity", "access_requests", accessRequests, "roles", roles)

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateTTL,
		func(req *proto.UserCertsRequest) {
			req.ReissuableRoleImpersonation = s.cfg.AllowReissue

			if len(accessRequests) > 0 {
				req.AccessRequests = accessRequests
				req.RoleRequests = []string{}
				req.UseRoleRequests = false
			}
		},
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

	if s.cfg.Cluster != "" {
		id, err = generateIdentity(
			ctx,
			s.botAuthClient,
			id,
			roles,
			s.botCfg.CertificateTTL,
			func(req *proto.UserCertsRequest) {
				req.RouteToCluster = s.cfg.Cluster
				req.ReissuableRoleImpersonation = s.cfg.AllowReissue
			},
		)
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

	if err := s.render(ctx, id, hostCAs, userCAs, databaseCAs); err != nil {
		return trace.Wrap(err)
	}

	if s.cfg.SSHConfigMode == config.SSHConfigModeOn {
		clusterNames, err := getClusterNames(ctx, impersonatedClient, id.ClusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		proxyPing, err := s.proxyPingCache.ping(ctx)
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
			s.botCfg,
		); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *IdentityOutputService) render(
	ctx context.Context,
	id *identity.Identity,
	hostCAs, userCAs, databaseCAs []types.CertAuthority,
) error {
	ctx, span := tracer.Start(
		ctx,
		"IdentityOutputService/render",
	)
	defer span.End()

	keyRing, err := NewClientKeyRing(id, hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := writeTLSCAs(ctx, s.cfg.Destination, hostCAs, userCAs, databaseCAs); err != nil {
		return trace.Wrap(err)
	}

	if err := writeIdentityFile(ctx, s.log, keyRing, s.cfg.Destination); err != nil {
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
	isUpgradeRequired(ctx context.Context, addr string, insecure bool) (bool, error)
}

func renderSSHConfig(
	ctx context.Context,
	log *slog.Logger,
	proxyPing *proxyPingResponse,
	clusterNames []string,
	dest bot.Destination,
	certAuthGetter certAuthGetter,
	getExecutablePath func() (string, error),
	alpnTester alpnTester,
	botCfg *config.BotConfig,
) error {
	ctx, span := tracer.Start(
		ctx,
		"renderSSHConfig",
	)
	defer span.End()

	proxyAddr, err := proxyPing.proxyWebAddr()
	if err != nil {
		return trace.Wrap(err, "determining proxy web addr")
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
	destDirectory, ok := dest.(*config.DestinationDirectory)
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
	if err != nil {
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
		connUpgradeRequired, err = alpnTester.isUpgradeRequired(
			ctx, proxyAddr, botCfg.Insecure,
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
		Insecure:             botCfg.Insecure,
		FIPS:                 botCfg.FIPS,
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

			Insecure:          botCfg.Insecure,
			FIPS:              botCfg.FIPS,
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

func getClusterNames(
	ctx context.Context, client *authclient.Client, connectedClusterName string,
) ([]string, error) {
	allClusterNames := []string{connectedClusterName}

	leafClusters, err := client.GetRemoteClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, lc := range leafClusters {
		allClusterNames = append(allClusterNames, lc.GetName())
	}

	return allClusterNames, nil
}
