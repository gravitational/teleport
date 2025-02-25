/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package service

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/openssh"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	servicebreaker "github.com/gravitational/teleport/lib/service/breaker"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const updateClientsJoinWarning = "This agent joined the cluster during the update_clients phase of a host CA rotation, so its services might not be usable by clients that haven't logged in recently."

// reconnectToAuthService continuously attempts to reconnect to the auth
// service until succeeds or process gets shut down
func (process *TeleportProcess) reconnectToAuthService(role types.SystemRole) (*Connector, error) {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.HalfJitter(process.Config.MaxRetryPeriod / 10),
		Step:   process.Config.MaxRetryPeriod / 5,
		Max:    process.Config.MaxRetryPeriod,
		Clock:  process.Clock,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for {
		connector, connectErr := process.connectToAuthService(role)
		if connectErr == nil {
			if connector.Client == nil {
				// Should only hit this if called with RoleAuth or RoleAdmin, which are both local and do not get a
				// client, so it does not make sense to call reconnectToAuthService.
				return nil, trace.BadParameter("reconnectToAuthService got a connector with no client, this is a logic error")
			}

			return connector, nil
		} else {
			switch {
			case errors.As(connectErr, &invalidVersionErr{}):
				return nil, trace.Wrap(connectErr)
			case role == types.RoleNode && strings.Contains(connectErr.Error(), auth.TokenExpiredOrNotFound):
				process.logger.ErrorContext(process.ExitContext(), "Can not join the cluster as node, the token expired or not found. Regenerate the token and try again.")
			default:
				process.logger.ErrorContext(process.ExitContext(), "Failed to establish connection to cluster.", "identity", role, "error", connectErr)
				if process.Config.Version == defaults.TeleportConfigVersionV3 && process.Config.ProxyServer.IsEmpty() {
					process.logger.ErrorContext(process.ExitContext(), "Check to see if the config has auth_server pointing to a Teleport Proxy. If it does, use proxy_server instead of auth_server.")
				}
			}
		}

		// Used for testing that auth service will attempt to reconnect in the provided duration.
		select {
		case process.Config.Testing.ConnectFailureC <- retry.Duration():
		default:
		}

		startedWait := process.Clock.Now()
		// Wait in between attempts, but return if teleport is shutting down
		select {
		case t := <-retry.After():
			process.logger.DebugContext(process.ExitContext(), "Retrying connection to auth server.", "identity", role, "backoff", t.Sub(startedWait))
			retry.Inc()
		case <-process.ExitContext().Done():
			process.logger.InfoContext(process.ExitContext(), "Stopping connection attempts, teleport is shutting down.", "identity", role)
			return nil, ErrTeleportExited
		}
	}
}

type invalidVersionErr struct {
	ClusterMajorVersion int64
	LocalMajorVersion   int64
}

func (i invalidVersionErr) Error() string {
	return fmt.Sprintf("Teleport instance is too new. This instance is running v%d. The auth server is running v%d and only supports instances on v%d or v%d. To connect anyway pass the --skip-version-check flag.", i.LocalMajorVersion, i.ClusterMajorVersion, i.ClusterMajorVersion, i.ClusterMajorVersion-1)
}

func (process *TeleportProcess) assertSystemRoles(rolesToAssert []types.SystemRole) (assertionID string, err error) {
	assertionID = uuid.New().String()
	irm := process.getInstanceRoleEventMapping()
	for _, role := range rolesToAssert {
		event, err := process.WaitForEvent(process.ExitContext(), irm[role])
		if err != nil {
			return "", trace.Errorf("process is exiting")
		}

		conn, ok := (event.Payload).(*Connector)
		if !ok {
			return "", trace.BadParameter("unsupported connector type: %T", event.Payload)
		}

		err = conn.Client.AssertSystemRole(process.ExitContext(), proto.SystemRoleAssertion{
			ServerID:    process.Config.HostUUID,
			AssertionID: assertionID,
			SystemRole:  role,
		})
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return assertionID, nil
}

func (process *TeleportProcess) authServerTooOld(resp *proto.PingResponse) error {
	serverVersion, err := semver.NewVersion(resp.ServerVersion)
	if err != nil {
		return trace.BadParameter("failed to parse reported auth server version as semver: %v", err)
	}

	version := teleport.Version
	if process.Config.Testing.TeleportVersion != "" {
		version = process.Config.Testing.TeleportVersion
	}
	teleportVersion, err := semver.NewVersion(version)
	if err != nil {
		return trace.BadParameter("failed to parse local teleport version as semver: %v", err)
	}

	if serverVersion.Major < teleportVersion.Major {
		if process.Config.SkipVersionCheck {
			process.logger.WarnContext(process.ExitContext(), "This instance is too new. Using a newer major version than the Auth server is unsupported and may impair functionality.", "version", teleportVersion.Major, "auth_version", serverVersion.Major, "supported_versions", []int64{serverVersion.Major, serverVersion.Major - 1})
			return nil
		}
		return trace.Wrap(invalidVersionErr{ClusterMajorVersion: serverVersion.Major, LocalMajorVersion: teleportVersion.Major})
	}

	return nil
}

// connectToAuthService attempts to login into the auth servers specified in the
// configuration and receive credentials.
func (process *TeleportProcess) connectToAuthService(role types.SystemRole, opts ...certOption) (*Connector, error) {
	connector, err := process.connect(role, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.logger.DebugContext(process.ExitContext(), "Client successfully connected to cluster", "client_identity", connector.clientIdentityString())
	process.addConnector(connector)

	return connector, nil
}

type (
	certOption  func(*certOptions)
	certOptions struct{}
)

func (process *TeleportProcess) connect(role types.SystemRole, opts ...certOption) (conn *Connector, err error) {
	var options certOptions
	for _, opt := range opts {
		opt(&options)
	}
	processState, err := process.storage.GetState(context.TODO(), role)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// no state recorded - this is the first connect
		// process will try to connect with the security token.
		c, err := process.firstTimeConnect(role)
		return c, trace.Wrap(err)
	}
	process.logger.DebugContext(process.ExitContext(), "Got connected state.", "rotation_state", logutils.StringerAttr(&processState.Spec.Rotation))

	identity, err := process.getIdentity(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotation := processState.Spec.Rotation

	switch rotation.State {
	// rotation is on standby, so just use whatever is current
	case "", types.RotationStateStandby:
		// The roles of admin and auth are treated in a special way, as in this case
		// the process does not need TLS clients and can use local auth directly.
		if role == types.RoleAdmin || role == types.RoleAuth {
			return newConnector(identity, identity)
		}
		process.logger.InfoContext(process.ExitContext(), "Connecting to the cluster with TLS client certificate.", "cluster", identity.ClusterName)
		connector, err := process.getConnector(identity, identity)
		if err != nil {
			// In the event that a user is attempting to connect a machine to
			// a different cluster it will give a cryptic warning about an
			// unknown certificate authority. Unfortunately we cannot intercept
			// this error as it comes from the http package before a request is
			// made. So provide a more user friendly error as a hint of what
			// they can do to resolve the issue.
			if strings.Contains(err.Error(), "certificate signed by unknown authority") {
				process.logger.ErrorContext(process.ExitContext(), "Was this node already registered to a different cluster? To join this node to a new cluster, remove the data_dir and try again", "data_dir", process.Config.DataDir)
			}
			return nil, trace.Wrap(err)
		}
		return connector, nil
	case types.RotationStateInProgress:
		switch rotation.Phase {
		case types.RotationPhaseInit:
			// Both clients and servers are using old credentials,
			// this phase exists for remote clusters to propagate information about the new CA
			if role == types.RoleAdmin || role == types.RoleAuth {
				return newConnector(identity, identity)
			}
			connector, err := process.getConnector(identity, identity)
			return connector, trace.Wrap(err)
		case types.RotationPhaseUpdateClients:
			// Clients should use updated credentials,
			// while servers should use old credentials to answer auth requests.
			newIdentity, err := process.storage.ReadIdentity(state.IdentityReplacement, role)
			if err != nil {
				if !trace.IsNotFound(err) {
					return nil, trace.Wrap(err)
				}
				newIdentity = identity
				// the only legitimate reason to not have a replacement identity
				// during update_clients is if the agent joined during
				// update_clients and then restarted
				process.logger.WarnContext(
					process.ExitContext(),
					updateClientsJoinWarning,
				)
			}
			if role == types.RoleAdmin || role == types.RoleAuth {
				return newConnector(newIdentity, identity)
			}
			connector, err := process.getConnector(newIdentity, identity)
			return connector, trace.Wrap(err)
		case types.RotationPhaseUpdateServers:
			// Servers and clients are using new identity credentials, but the
			// identity is still set up to trust the old certificate authority certificates.
			if role == types.RoleAdmin || role == types.RoleAuth {
				return newConnector(identity, identity)
			}
			connector, err := process.getConnector(identity, identity)
			return connector, trace.Wrap(err)
		case types.RotationPhaseRollback:
			// In rollback phase, clients and servers should switch back
			// to the old certificate authority-issued credentials,
			// but the new certificate authority should be trusted
			// because not all clients can update at the same time.
			if role == types.RoleAdmin || role == types.RoleAuth {
				return newConnector(identity, identity)
			}
			connector, err := process.getConnector(identity, identity)
			return connector, trace.Wrap(err)
		default:
			return nil, trace.BadParameter("unsupported rotation phase: %q", rotation.Phase)
		}
	default:
		return nil, trace.BadParameter("unsupported rotation state: %q", rotation.State)
	}
}

// newWatcher returns a new watcher,
// either using local auth server connection or remote client
func (process *TeleportProcess) newWatcher(conn *Connector, watch types.Watch) (types.Watcher, error) {
	switch conn.Role() {
	case types.RoleAdmin, types.RoleAuth:
		return process.localAuth.NewWatcher(process.ExitContext(), watch)
	}
	return conn.Client.NewWatcher(process.ExitContext(), watch)
}

// getCertAuthority returns cert authority by ID.
// In case if auth servers, the role is 'TeleportAdmin' and instead of using
// TLS client this method uses the local auth server.
func (process *TeleportProcess) getCertAuthority(conn *Connector, id types.CertAuthID, loadPrivateKeys bool) (types.CertAuthority, error) {
	switch conn.Role() {
	case types.RoleAdmin, types.RoleAuth:
		return process.localAuth.GetCertAuthority(process.ExitContext(), id, loadPrivateKeys)
	}
	ctx, cancel := context.WithTimeout(process.ExitContext(), apidefaults.DefaultIOTimeout)
	defer cancel()
	return conn.Client.GetCertAuthority(ctx, id, loadPrivateKeys)
}

// reRegister receives new identity credentials for proxy, node and auth.
// In case if auth servers, the role is 'TeleportAdmin' and instead of using
// TLS client this method uses the local auth server.
func (process *TeleportProcess) reRegister(conn *Connector, additionalPrincipals []string, dnsNames []string, rotation types.Rotation, systemRoles []types.SystemRole, assertionID string) (*state.Identity, error) {
	id := conn.clientState.Load().identity.ID
	if id.NodeName == "" {
		id.NodeName = process.Config.Hostname
	}
	if id.Role == types.RoleAdmin || id.Role == types.RoleAuth {
		return auth.GenerateIdentity(process.localAuth, id, additionalPrincipals, dnsNames)
	}

	var clt auth.ReRegisterClient = conn.Client
	var remoteAddr string
	if srv := process.getLocalAuth(); srv != nil {
		clt = srv
		// auth server typically extracts remote addr from conn. since we're using the local auth
		// directly we must supply a reasonable remote addr value. preferably the advertise IP, but
		// otherwise localhost. this behavior must be kept consistent with the equivalent behavior
		// in LocalRegister.
		remoteAddr = cmp.Or(process.Config.AdvertiseIP, defaults.Localhost)
	}

	ctx, cancel := context.WithTimeout(process.ExitContext(), apidefaults.DefaultIOTimeout)
	defer cancel()
	identity, err := auth.ReRegister(ctx, auth.ReRegisterParams{
		Client:                clt,
		ID:                    id,
		AdditionalPrincipals:  additionalPrincipals,
		DNSNames:              dnsNames,
		RemoteAddr:            remoteAddr,
		Rotation:              rotation,
		SystemRoles:           systemRoles,
		SystemRoleAssertionID: assertionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

func (process *TeleportProcess) firstTimeConnect(role types.SystemRole) (*Connector, error) {
	id := state.IdentityID{
		Role:     role,
		HostUUID: process.Config.HostUUID,
		NodeName: process.Config.Hostname,
	}
	additionalPrincipals, dnsNames, err := process.getAdditionalPrincipals(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var identity *state.Identity
	if process.getLocalAuth() != nil {
		// Auth service is on the same host, no need to go though the invitation
		// procedure.
		process.logger.DebugContext(process.ExitContext(), "This server has local Auth server started, using it to add role to the cluster.")
		var systemRoles []types.SystemRole
		if role == types.RoleInstance {
			// normally this is taken from the join token, but if we're dealing with a local auth server, we
			// need to determine the roles for the instance cert ourselves.
			systemRoles = process.getInstanceRoles()
		}

		identity, err = auth.LocalRegister(id, process.getLocalAuth(), additionalPrincipals, dnsNames, process.Config.AdvertiseIP, systemRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Auth server is remote, so we need a provisioning token.
		if !process.Config.HasToken() {
			return nil, trace.BadParameter("%v must join a cluster and needs a provisioning token", role)
		}

		process.logger.InfoContext(process.ExitContext(), "Joining the cluster with a secure token.")
		token, err := process.Config.Token()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dataDir := defaults.DataDir
		if process.Config.DataDir != "" {
			dataDir = process.Config.DataDir
		}

		registerParams := join.RegisterParams{
			Token:                token,
			ID:                   id,
			AuthServers:          process.Config.AuthServerAddresses(),
			ProxyServer:          process.Config.ProxyServer,
			AdditionalPrincipals: additionalPrincipals,
			DNSNames:             dnsNames,
			CipherSuites:         process.Config.CipherSuites,
			CAPins:               process.Config.CAPins,
			CAPath:               filepath.Join(dataDir, defaults.CACertFile),
			GetHostCredentials:   client.HostCredentials,
			Clock:                process.Clock,
			JoinMethod:           process.Config.JoinMethod,
			// this circuit breaker is used for a client that only does a few
			// requests before closing
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
			FIPS:                 process.Config.FIPS,
			Insecure:             lib.IsInsecureDevMode(),
		}
		if registerParams.JoinMethod == types.JoinMethodAzure {
			registerParams.AzureParams = join.AzureParams{
				ClientID: process.Config.JoinParams.Azure.ClientID,
			}
		}

		result, err := join.Register(process.ExitContext(), registerParams)
		if err != nil {
			if utils.IsUntrustedCertErr(err) {
				return nil, trace.WrapWithMessage(err, utils.SelfSignedCertsMsg)
			}
			return nil, trace.Wrap(err)
		}

		privateKeyPEM, err := keys.MarshalPrivateKey(result.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity, err = state.ReadIdentityFromKeyPair(privateKeyPEM, result.Certs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	process.logger.InfoContext(process.ExitContext(), "Successfully obtained credentials to connect to the cluster.", "identity", role)
	var connector *Connector
	if role == types.RoleAdmin || role == types.RoleAuth {
		connector, err = newConnector(identity, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		connector, err = process.getConnector(identity, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Sync local rotation state to match the remote rotation state.
	ca, err := process.getCertAuthority(connector, types.CertAuthID{
		DomainName: connector.ClusterName(),
		Type:       types.HostCA,
	}, false)
	if err != nil {
		return nil, trace.NewAggregate(err, connector.Close())
	}

	if ca.GetRotation().Phase == types.RotationPhaseUpdateClients {
		process.logger.WarnContext(
			process.ExitContext(),
			updateClientsJoinWarning,
		)
	}

	if err := process.storage.WriteIdentity(state.IdentityCurrent, *identity); err != nil {
		process.logger.WarnContext(process.ExitContext(), "Failed to write identity to storage.", "identity", role, "error", err)
	}

	err = process.storage.WriteState(role, state.StateV2{
		Spec: state.StateSpecV2{
			Rotation:            ca.GetRotation(),
			InitialLocalVersion: teleport.Version,
		},
	})
	process.rotationCache.Remove(role)
	if err != nil {
		return nil, trace.NewAggregate(err, connector.Close())
	}
	process.logger.InfoContext(process.ExitContext(), "The process successfully wrote the credentials and state to the disk.", "identity", role)
	return connector, nil
}

func (process *TeleportProcess) initOpenSSH() {
	process.RegisterWithAuthServer(types.RoleNode, SSHIdentityEvent)
	process.SSHD = openssh.NewSSHD(
		process.Config.OpenSSH.RestartCommand,
		process.Config.OpenSSH.CheckCommand,
		process.Config.OpenSSH.SSHDConfigPath,
	)
	process.RegisterCriticalFunc("openssh.rotate", process.syncOpenSSHRotationState)
}

func (process *TeleportProcess) syncOpenSSHRotationState() error {
	if _, err := process.WaitForEvent(process.GracefulExitContext(), TeleportReadyEvent); err != nil {
		return trace.Wrap(err)
	}
	conn, err := process.WaitForConnector(SSHIdentityEvent, nil)
	if conn == nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	_, err = process.syncRotationState(conn)
	if err != nil {
		return trace.Wrap(err)
	}

	id, err := process.storage.ReadIdentity(state.IdentityCurrent, types.RoleNode)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := process.GracefulExitContext()
	cas, err := conn.Client.GetCertAuthorities(ctx, types.OpenSSHCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	keysDir := filepath.Join(process.Config.DataDir, openssh.SSHDKeysDir)
	if err := openssh.WriteKeys(keysDir, id, cas); err != nil {
		return trace.Wrap(err)
	}

	err = process.SSHD.UpdateConfig(openssh.SSHDConfigUpdate{
		SSHDConfigPath: process.Config.OpenSSH.SSHDConfigPath,
		DataDir:        process.Config.DataDir,
	}, process.Config.OpenSSH.RestartSSHD)
	if err != nil {
		return trace.Wrap(err)
	}

	state, err := process.storage.GetState(ctx, types.RoleNode)
	if err != nil {
		return trace.Wrap(err)
	}

	mostRecentRotation := state.Spec.Rotation.LastRotated
	if state.Spec.Rotation.State == types.RotationStateInProgress && state.Spec.Rotation.Started.After(mostRecentRotation) {
		mostRecentRotation = state.Spec.Rotation.Started
	}
	for _, ca := range cas {
		caRot := ca.GetRotation()
		if caRot.State == types.RotationStateInProgress && caRot.Started.After(mostRecentRotation) {
			mostRecentRotation = caRot.Started
		}

		if caRot.LastRotated.After(mostRecentRotation) {
			mostRecentRotation = caRot.LastRotated
		}
	}

	if err := registerServer(process.Config, ctx, conn.Client, mostRecentRotation); err != nil {
		return trace.Wrap(err)
	}

	// if any of the above exits with non nil error, the process is
	// shut down as it is run via RegisterCriticalFunction, so we
	// manually shut down here as we dont want teleport to remain
	// running after
	go func() {
		// run in a go routine as process.Shutdown waits until
		// all registered services/functions have finished and
		// this cant finish if its waiting on this function to
		// return
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		process.Shutdown(ctx)
	}()

	return nil
}

func registerServer(a *servicecfg.Config, ctx context.Context, client authclient.ClientI, lastRotation time.Time) error {
	server, err := types.NewServerWithLabels(
		a.HostUUID,
		types.KindNode,
		types.ServerSpecV2{
			Addr:     a.OpenSSH.InstanceAddr,
			Hostname: a.Hostname,
			Rotation: types.Rotation{
				LastRotated: lastRotation,
			},
		},
		a.OpenSSH.Labels,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	server.SetSubKind(types.SubKindOpenSSHNode)

	if _, err := client.UpsertNode(ctx, server); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// periodicSyncRotationState checks rotation state periodically and
// takes action if necessary
func (process *TeleportProcess) periodicSyncRotationState() error {
	// start rotation only after teleport process has started
	if _, err := process.WaitForEvent(process.GracefulExitContext(), TeleportReadyEvent); err != nil {
		return nil
	}

	maxRetryPeriod := defaults.MaxWatcherBackoff

	// non-control-plane agents should use a longer backoff to avoid creating excess
	// cluster load during times of instability.
	if !process.instanceRoleExpected(types.RoleAuth) && !process.instanceRoleExpected(types.RoleProxy) {
		maxRetryPeriod = defaults.MaxLongWatcherBackoff
	}

	process.logger.InfoContext(process.ExitContext(), "The new service has started successfully. Starting syncing rotation status.", "max_retry_period", maxRetryPeriod)

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(maxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(maxRetryPeriod / 16),
		Max:    maxRetryPeriod,
		Jitter: retryutils.HalfJitter,
		Clock:  process.Clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		err := process.syncRotationStateCycle(retry)
		if err == nil {
			return nil
		}

		process.logger.WarnContext(process.ExitContext(), "Sync rotation state cycle failed.", "retry_interval", process.Config.RotationConnectionInterval)

		startWait := process.Clock.Now()
		select {
		case t := <-retry.After():
			process.logger.InfoContext(process.ExitContext(), "Initiating new sync rotation state cycle after backoff.", "backoff_time", t.Sub(startWait))
			retry.Inc()
		case <-process.GracefulExitContext().Done():
			return nil
		}
	}
}

// syncRotationCycle executes a rotation cycle that returns:
//
// * nil whenever rotation state leads to teleport reload event
// * error whenever rotation cycle has to be restarted
//
// the function accepts extra delay timer extraDelay in case if parent
// function needs a
func (process *TeleportProcess) syncRotationStateCycle(retry retryutils.Retry) error {
	connectors := process.getConnectors()
	if len(connectors) == 0 {
		return trace.BadParameter("no connectors found")
	}
	// it is important to use the same view of the certificate authority
	// for all internal services at the same time, so that the same
	// procedure will be applied at the same time for multiple service process
	// and no internal services is left behind.
	conn := connectors[0]

	if err := process.syncRotationStateAndBroadcast(conn); err != nil {
		return trace.Wrap(err)
	}

	watcher, err := process.newWatcher(conn, types.Watch{Kinds: []types.WatchKind{{
		Kind: types.KindCertAuthority,
		Filter: types.CertAuthorityFilter{
			types.HostCA: conn.ClusterName(),
		}.IntoMap(),
	}}})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	periodic := interval.New(interval.Config{
		Duration:      process.Config.PollingPeriod,
		FirstDuration: retryutils.HalfJitter(process.Config.PollingPeriod),
		Jitter:        retryutils.SeventhJitter,
	})
	defer periodic.Stop()

	for {
		select {
		case event := <-watcher.Events():
			if event.Type == types.OpInit || event.Type == types.OpDelete {
				if event.Type == types.OpInit {
					retry.Reset()
				}
				continue
			}
			ca, ok := event.Resource.(types.CertAuthority)
			if !ok {
				continue
			}
			if ca.GetType() != types.HostCA || ca.GetClusterName() != conn.ClusterName() {
				continue
			}
			if err := process.syncRotationStateAndBroadcast(conn); err != nil {
				return trace.Wrap(err)
			}
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher has disconnected")
		case <-periodic.Next():
			if err := process.syncRotationStateAndBroadcast(conn); err != nil {
				return trace.Wrap(err)
			}
		case <-process.GracefulExitContext().Done():
			return nil
		}
	}
}

// syncRotationStateAndBroadcast syncs rotation state and broadcasts events
// when phase has been changed or reload happened
func (process *TeleportProcess) syncRotationStateAndBroadcast(conn *Connector) error {
	status, err := process.syncRotationState(conn)
	if err != nil {
		if trace.IsConnectionProblem(err) {
			process.logger.WarnContext(process.ExitContext(), "Connection problem: sync rotation state.", "error", err)
		} else {
			process.logger.WarnContext(process.ExitContext(), "Failed to sync rotation state.", "error", err)
		}
		return trace.Wrap(err)
	}

	if status.phaseChanged {
		process.logger.DebugContext(process.ExitContext(), "Sync rotation state detected CA rotation phase update.")
		process.BroadcastEvent(Event{Name: TeleportPhaseChangeEvent})
	}
	if status.credentialsUpdated {
		process.logger.InfoContext(process.ExitContext(), "Credentials reissued.")
		process.BroadcastEvent(Event{Name: TeleportCredentialsUpdatedEvent})
	}
	return nil
}

// syncRotationState compares cluster rotation state with the state of
// internal services and performs the rotation if necessary.
func (process *TeleportProcess) syncRotationState(conn *Connector) (rotationStatus, error) {
	connectors := process.getConnectors()
	ca, err := process.getCertAuthority(conn, types.CertAuthID{
		DomainName: conn.ClusterName(),
		Type:       types.HostCA,
	}, false)
	if err != nil {
		return rotationStatus{}, trace.Wrap(err)
	}
	var status rotationStatus
	for _, conn := range connectors {
		serviceStatus, err := process.syncServiceRotationState(ca, conn)
		if err != nil {
			return rotationStatus{}, trace.Wrap(err)
		}
		if serviceStatus.credentialsUpdated {
			status.credentialsUpdated = true
		}
		if serviceStatus.phaseChanged {
			status.phaseChanged = true
		}
	}
	return status, nil
}

// syncServiceRotationState syncs up rotation state for internal services (Auth, Proxy, Node) and
// if necessary, updates credentials. Returns true if the service will need to reload.
func (process *TeleportProcess) syncServiceRotationState(ca types.CertAuthority, conn *Connector) (*rotationStatus, error) {
	state, err := process.storage.GetState(context.TODO(), conn.Role())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return process.rotate(conn, *state, ca.GetRotation())
}

type rotationStatus struct {
	// credentialsUpdated means that credentials were updated in the
	// [Connector].
	credentialsUpdated bool
	// phaseChanged means that a CA rotation phase has been updated, but
	// credentials were not reissued.
	phaseChanged bool
}

// checkServerIdentity returns a boolean that indicates the host certificate
// needs to be regenerated.
func checkServerIdentity(ctx context.Context, conn *Connector, additionalPrincipals []string, dnsNames []string, logger *slog.Logger) bool {
	var principalsChanged bool
	var dnsNamesChanged bool

	// Remove 0.0.0.0 (meaning advertise_ip has not) if it exists in the list of
	// principals. The 0.0.0.0 values tells the auth server to "guess" the nodes
	// IP. If 0.0.0.0 is not removed, a check is performed if it exists in the
	// list of principals in the certificate. Since it never exists in the list
	// of principals (auth server will always remove it before issuing a
	// certificate) regeneration is always requested.
	principalsToCheck := utils.RemoveFromSlice(additionalPrincipals, defaults.AnyAddress)

	// If advertise_ip, public_addr, or listen_addr in file configuration were
	// updated, the list of principals (SSH) or DNS names (TLS) on the
	// certificate need to be updated.
	serverIdentity := conn.serverState.Load().identity
	if len(additionalPrincipals) != 0 && !serverIdentity.HasPrincipals(principalsToCheck) {
		principalsChanged = true
		logger.InfoContext(ctx, "Rotation in progress, updating SSH principals.", "identity", serverIdentity.ID.Role, "additional_principals", additionalPrincipals, "current_principals", serverIdentity.Cert.ValidPrincipals)
	}
	if len(dnsNames) != 0 && !serverIdentity.HasDNSNames(dnsNames) {
		dnsNamesChanged = true
		logger.InfoContext(ctx, "Rotation in progress, updating DNS names.", "identity", serverIdentity.ID.Role, "additional_dns_names", dnsNames, "current_dns_names", serverIdentity.XCert.DNSNames)
	}

	return principalsChanged || dnsNamesChanged
}

// rotate is called to check if rotation should be triggered.
func (process *TeleportProcess) rotate(conn *Connector, localState state.StateV2, remote types.Rotation) (*rotationStatus, error) {
	clientIdentity := conn.clientState.Load().identity
	id := clientIdentity.ID
	local := localState.Spec.Rotation

	additionalPrincipals, dnsNames, err := process.getAdditionalPrincipals(id.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var assertionID string
	var systemRoles []types.SystemRole
	var wantsSystemRoleRepair bool
	if id.Role == types.RoleInstance {
		var baseSystemRoles []types.SystemRole
		for _, baseRole := range clientIdentity.SystemRoles {
			baseSystemRoles = append(baseSystemRoles, types.SystemRole(baseRole))
		}
		var danglingSystemRoles []types.SystemRole
		for _, activeRole := range process.getInstanceRoles() {
			if slices.Contains(baseSystemRoles, activeRole) {
				continue
			}
			danglingSystemRoles = append(danglingSystemRoles, activeRole)
		}

		if len(danglingSystemRoles) != 0 {
			process.logger.WarnContext(process.ExitContext(), "rotation logic detected dangling system role(s), will attempt to self-repair", "existing", clientIdentity.SystemRoles, "dangling", danglingSystemRoles)
			// if auth is running locally, we can just generate ourselves a new instance cert with the correct
			// roles. otherwise, we need to prove to the remote auth server that we hold the necessary privileges
			// by performing system role assertions.
			if process.getLocalAuth() == nil {
				assertionID, err = process.assertSystemRoles(danglingSystemRoles)
				if err != nil && !trace.IsNotImplemented(err) {
					return nil, trace.Wrap(err)
				}

				if trace.IsNotImplemented(err) {
					danglingSystemRoles = nil
					assertionID = ""
					process.logger.WarnContext(process.ExitContext(), "auth server does not implement system role assertions, self-repair will be re-attempted later")
				}
			}
		}

		systemRoles = append(systemRoles, baseSystemRoles...)
		systemRoles = append(systemRoles, danglingSystemRoles...)
		wantsSystemRoleRepair = len(danglingSystemRoles) != 0
	}

	// Check if any of the SSH principals or TLS DNS names have changed and the
	// host credentials need to be regenerated.
	regenerateCertificate := checkServerIdentity(process.ExitContext(), conn, additionalPrincipals, dnsNames, process.logger) || wantsSystemRoleRepair

	// If the local state matches remote state and neither principals or DNS
	// names changed, nothing to do. CA is in sync.
	if local.Matches(remote) && !regenerateCertificate {
		return &rotationStatus{}, nil
	}

	storage := process.storage

	const outOfSync = "%v and cluster rotation state (%v) is out of sync with local (%v). Clear local state and re-register this %v."

	writeStateAndIdentity := func(name string, identity *state.Identity) error {
		err = storage.WriteIdentity(name, *identity)
		if err != nil {
			return trace.Wrap(err)
		}
		localState.Spec.Rotation = remote
		err = storage.WriteState(id.Role, localState)
		process.rotationCache.Remove(id.Role)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	switch remote.State {
	case "", types.RotationStateStandby:
		switch local.State {
		// There is nothing to do, it could happen
		// that the old node came up and missed the whole rotation
		// rollback cycle.
		case "", types.RotationStateStandby:
			if regenerateCertificate {
				process.logger.InfoContext(process.ExitContext(), "Service has updated principals and DNS Names, going to request new principals and update.", "identity", id.Role, "principals", additionalPrincipals, "dns_names", dnsNames)
				identity, err := process.reRegister(conn, additionalPrincipals, dnsNames, remote, systemRoles, assertionID)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				connState, err := newConnectorState(identity)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				err = storage.WriteIdentity(state.IdentityCurrent, *identity)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				conn.clientState.Store(connState)
				conn.serverState.Store(connState)
				return &rotationStatus{credentialsUpdated: true}, nil
			}
			return &rotationStatus{}, nil
		case types.RotationStateInProgress:
			// Rollback phase has been completed, all services
			// will receive new identities.
			if local.Phase != types.RotationPhaseRollback && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			identity, err := process.reRegister(conn, additionalPrincipals, dnsNames, remote, systemRoles, assertionID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			connState, err := newConnectorState(identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(state.IdentityCurrent, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			conn.clientState.Store(connState)
			conn.serverState.Store(connState)
			return &rotationStatus{credentialsUpdated: true}, nil
		default:
			return nil, trace.BadParameter("unsupported state: %q", localState)
		}
	case types.RotationStateInProgress:
		switch remote.Phase {
		case types.RotationPhaseStandby, "":
			// There is nothing to do.
			return &rotationStatus{}, nil
		case types.RotationPhaseInit:
			// Only allow transition in case if local rotation state is standby
			// so this server is in the "clean" state.
			if local.State != types.RotationStateStandby && local.State != "" {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			// only update local phase, there is no need to reload
			localState.Spec.Rotation = remote
			err = storage.WriteState(id.Role, localState)
			process.rotationCache.Remove(id.Role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &rotationStatus{phaseChanged: true}, nil
		case types.RotationPhaseUpdateClients:
			// Allow transition to this phase only if the previous
			// phase was "Init".
			if local.Phase != types.RotationPhaseInit && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			identity, err := process.reRegister(conn, additionalPrincipals, dnsNames, remote, systemRoles, assertionID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			connState, err := newConnectorState(identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			process.logger.DebugContext(process.ExitContext(), "Re-registered, received new identity.", "identity", logutils.StringerAttr(identity))
			err = writeStateAndIdentity(state.IdentityReplacement, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			conn.clientState.Store(connState)
			return &rotationStatus{credentialsUpdated: true}, nil
		case types.RotationPhaseUpdateServers:
			// Allow transition to this phase only if the previous
			// phase was "Update clients".
			if local.Phase != types.RotationPhaseUpdateClients && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			clientState := conn.clientState.Load()
			err = writeStateAndIdentity(state.IdentityCurrent, clientState.identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			conn.serverState.Store(clientState)
			return &rotationStatus{credentialsUpdated: true}, nil
		case types.RotationPhaseRollback:
			// Allow transition to this phase from any other local phase
			// because it will be widely used to recover cluster state to
			// the previously valid state, client will re-register to receive
			// credentials signed by the "old" CA.
			identity, err := process.reRegister(conn, additionalPrincipals, dnsNames, remote, systemRoles, assertionID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			connState, err := newConnectorState(identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(state.IdentityCurrent, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			conn.clientState.Store(connState)
			conn.serverState.Store(connState)
			return &rotationStatus{credentialsUpdated: true}, nil
		default:
			return nil, trace.BadParameter("unsupported phase: %q", remote.Phase)
		}
	default:
		return nil, trace.BadParameter("unsupported state: %q", remote.State)
	}
}

// getConnector gets an appropriate [Connector] for the given identity. The returned [Connector] is backed by the
// instance client is reused if appropriate, otherwise a new client is created.
func (process *TeleportProcess) getConnector(clientIdentity, serverIdentity *state.Identity) (*Connector, error) {
	newConn, err := newConnector(clientIdentity, serverIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if clientIdentity.ID.Role != types.RoleInstance {
		// non-instance roles should wait to see if the instance client can be reused
		// before acquiring their own client.
		if instanceConn := process.waitForInstanceConnector(); instanceConn != nil && instanceConn.Client != nil {
			instanceClientIdentity := instanceConn.clientState.Load().identity
			if instanceClientIdentity.HasSystemRole(clientIdentity.ID.Role) {
				process.logger.InfoContext(process.ExitContext(), "Reusing Instance client.", "identity", clientIdentity.ID.Role, "additional_system_roles", instanceClientIdentity.SystemRoles)
				newConn.Client = instanceConn.Client
				newConn.ReusedClient = true
				return newConn, nil
			} else {
				process.logger.WarnContext(process.ExitContext(), "Unable to reuse Instance client.", "identity", clientIdentity.ID.Role, "additional_system_roles", instanceClientIdentity.SystemRoles)
			}
		} else {
			process.logger.WarnContext(process.ExitContext(), "Instance client not available for reuse.", "identity", clientIdentity.ID.Role)
		}
	}

	clt, pingResponse, err := process.newClient(newConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return a helpful message and don't retry if ping was successful,
	// but the auth server is too old. Auth is not going to get any younger.
	if err := process.authServerTooOld(pingResponse); err != nil {
		return nil, trace.NewAggregate(err, clt.Close())
	}

	// Set cluster features and return successfully with a working connector.
	// TODO(michellescripts) remove clone & compatibility check in v18
	cloned := apiutils.CloneProtoMsg(pingResponse.GetServerFeatures())
	entitlements.BackfillFeatures(cloned)
	process.setClusterFeatures(cloned)
	process.setAuthSubjectiveAddr(pingResponse.RemoteAddr)
	process.logger.InfoContext(process.ExitContext(), "features loaded from auth server", "identity", clientIdentity.ID.Role, "features", pingResponse.GetServerFeatures())

	newConn.Client = clt
	return newConn, nil
}

// newClient attempts to connect to either the proxy server or auth server
// For config v3 and onwards, it will only connect to either the proxy (via tunnel) or the auth server (direct),
// depending on what was specified in the config.
// For config v1 and v2, it will attempt to direct dial the auth server, and fallback to trying to tunnel
// to the Auth Server through the proxy.
func (process *TeleportProcess) newClient(connector *Connector) (*authclient.Client, *proto.PingResponse, error) {
	tlsConfig := utils.TLSConfig(process.Config.CipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := connector.ClientGetCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.ServerName = apiutils.EncodeClusterName(connector.ClusterName())
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(connector.ClientGetPool)

	sshClientConfig, err := connector.clientSSHClientConfig(process.Config.FIPS)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	authServers := process.Config.AuthServerAddresses()
	connectToAuthServer := func(logger *slog.Logger) (*authclient.Client, *proto.PingResponse, error) {
		logger.DebugContext(process.ExitContext(), "Attempting to connect to Auth Server directly.")
		clt, pingResponse, err := process.newClientDirect(authServers, tlsConfig, connector.Role())
		if err != nil {
			logger.DebugContext(process.ExitContext(), "Failed to connect to Auth Server directly.")
			return nil, nil, err
		}

		logger.DebugContext(process.ExitContext(), "Connected to Auth Server with direct connection.")
		return clt, pingResponse, nil
	}

	switch process.Config.Version {
	// for config v1 and v2, attempt to directly connect to the auth server and fall back to tunneling
	case defaults.TeleportConfigVersionV1, defaults.TeleportConfigVersionV2:
		// if we don't have a proxy address, try to connect to the auth server directly
		logger := process.logger.With("auth_addrs", utils.NetAddrsToStrings(authServers))

		directClient, resp, directErr := connectToAuthServer(logger)
		if directErr == nil {
			return directClient, resp, nil
		}

		// Don't attempt to connect through a tunnel as a proxy or auth server.
		if connector.Role() == types.RoleAuth || connector.Role() == types.RoleProxy {
			return nil, nil, trace.Wrap(directErr)
		}

		// if that fails, attempt to connect to the auth server through a tunnel

		logger.DebugContext(process.ExitContext(), "Attempting to discover reverse tunnel address.")
		logger.DebugContext(process.ExitContext(), "Attempting to connect to Auth Server through tunnel.")

		tunnelClient, pingResponse, err := process.newClientThroughTunnel(tlsConfig, sshClientConfig, connector.Role(), connector.ClientGetPool)
		if err != nil {
			process.logger.ErrorContext(process.ExitContext(), "Node failed to establish connection to Teleport Proxy. We have tried the following endpoints:")
			process.logger.ErrorContext(process.ExitContext(), "- connecting to auth server directly", "error", directErr)
			if trace.IsConnectionProblem(err) && strings.Contains(err.Error(), "connection refused") {
				err = trace.Wrap(err, "This is the alternative port we tried and it's not configured.")
			}
			process.logger.ErrorContext(process.ExitContext(), "- connecting to auth server through tunnel", "error", err)
			collectedErrs := trace.NewAggregate(directErr, err)
			if utils.IsUntrustedCertErr(collectedErrs) {
				collectedErrs = trace.Wrap(collectedErrs, utils.SelfSignedCertsMsg)
			}
			return nil, nil, trace.Wrap(collectedErrs, "Failed to connect to Auth Server directly or over tunnel, no methods remaining.")
		}

		logger.DebugContext(process.ExitContext(), "Connected to Auth Server through tunnel.")
		return tunnelClient, pingResponse, nil

	// for config v3, either tunnel to the given proxy server or directly connect to the given auth server
	case defaults.TeleportConfigVersionV3:
		proxyServer := process.Config.ProxyServer
		if !proxyServer.IsEmpty() {
			logger := process.logger.With("proxy_server", proxyServer.String())
			logger.DebugContext(process.ExitContext(), "Attempting to connect to Auth Server through tunnel.")
			tunnelClient, pingResponse, err := process.newClientThroughTunnel(tlsConfig, sshClientConfig, connector.Role(), connector.ClientGetPool)
			if err != nil {
				return nil, nil, trace.Errorf("Failed to connect to Proxy Server through tunnel: %v", err)
			}

			logger.DebugContext(process.ExitContext(), "Connected to Auth Server through tunnel.")

			return tunnelClient, pingResponse, nil
		}

		// if we don't have a proxy address, try to connect to the auth server directly
		logger := process.logger.With("auth_server", utils.NetAddrsToStrings(authServers))

		return connectToAuthServer(logger)
	}

	return nil, nil, trace.NotImplemented("could not find connection strategy for config version %s", process.Config.Version)
}

func (process *TeleportProcess) breakerConfigForRole(role types.SystemRole) breaker.Config {
	// Disable circuit breaking for proxies. A proxy often times forwards
	// requests to auth on behalf of agents(during joining) or unauthenticated
	// users(webapi/ping) and any errors that may be encountered during forwarded
	// requests could trip the breaker eventhough auth is healthy. Since the number
	// of agents in a cluster should far outnumber the proxies this shouldn't
	// have much impact.
	if role == types.RoleProxy || process.instanceRoleExpected(types.RoleProxy) {
		return breaker.NoopBreakerConfig()
	}
	return servicebreaker.InstrumentBreakerForConnector(role, process.Config.CircuitBreakerConfig)
}

func (process *TeleportProcess) newClientThroughTunnel(tlsConfig *tls.Config, sshConfig *ssh.ClientConfig, role types.SystemRole, getClusterCAs func() (*x509.CertPool, error)) (*authclient.Client, *proto.PingResponse, error) {
	dialer, err := reversetunnelclient.NewTunnelAuthDialer(reversetunnelclient.TunnelAuthDialerConfig{
		Resolver:              process.resolver,
		ClientConfig:          sshConfig,
		Log:                   process.logger,
		InsecureSkipTLSVerify: lib.IsInsecureDevMode(),
		GetClusterCAs: func(context.Context) (*x509.CertPool, error) {
			return getClusterCAs()
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clt, err := authclient.NewClient(apiclient.Config{
		Context: process.ExitContext(),
		Dialer:  dialer,
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: process.breakerConfigForRole(role),
		DialTimeout:          process.Config.Testing.ClientTimeout,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// If connected, make sure the connector's client works by using
	// a call that should succeed at all times (Ping).
	ctx, cancel := context.WithTimeout(process.ExitContext(), apidefaults.DefaultIOTimeout)
	defer cancel()
	resp, err := clt.Ping(ctx)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, clt.Close())
	}

	return clt, &resp, nil
}

func (process *TeleportProcess) newClientDirect(authServers []utils.NetAddr, tlsConfig *tls.Config, role types.SystemRole) (*authclient.Client, *proto.PingResponse, error) {
	var cltParams []roundtrip.ClientParam
	if process.Config.Testing.ClientTimeout != 0 {
		cltParams = []roundtrip.ClientParam{
			authclient.ClientParamIdleConnTimeout(process.Config.Testing.ClientTimeout),
			authclient.ClientParamResponseHeaderTimeout(process.Config.Testing.ClientTimeout),
		}
	}

	var dialOpts []grpc.DialOption
	if role == types.RoleProxy {
		grpcMetrics := metrics.CreateGRPCClientMetrics(process.Config.Metrics.GRPCClientLatency, prometheus.Labels{teleport.TagClient: "teleport-proxy"})
		if err := metrics.RegisterPrometheusCollectors(grpcMetrics); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		dialOpts = append(dialOpts, []grpc.DialOption{
			grpc.WithUnaryInterceptor(grpcMetrics.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(grpcMetrics.StreamClientInterceptor()),
		}...)
	}

	clt, err := authclient.NewClient(apiclient.Config{
		Context: process.ExitContext(),
		Addrs:   utils.NetAddrsToStrings(authServers),
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
		DialTimeout:          process.Config.Testing.ClientTimeout,
		CircuitBreakerConfig: process.breakerConfigForRole(role),
		DialOpts:             dialOpts,
	}, cltParams...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// If connected, make sure the connector's client works by using
	// a call that should succeed at all times (Ping).
	ctx, cancel := context.WithTimeout(process.ExitContext(), apidefaults.DefaultIOTimeout)
	defer cancel()
	resp, err := clt.Ping(ctx)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, clt.Close())
	}

	return clt, &resp, nil
}
