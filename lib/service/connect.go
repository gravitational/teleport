package service

import (
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// connectToAuthService attempts to login into the auth servers specified in the
// configuration and receive credentials.
func (process *TeleportProcess) connectToAuthService(role teleport.Role) (*Connector, error) {
	connector, err := process.connect(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.Debugf("Connected client: %v", connector.ClientIdentity)
	process.Debugf("Connected server: %v", connector.ServerIdentity)
	process.addConnector(connector)
	return connector, nil
}

func (process *TeleportProcess) connect(role teleport.Role) (*Connector, error) {
	state, err := process.storage.GetState(role)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// no state recorded - this is the first connect
		// process will try to connect with the security token.
		return process.firstTimeConnect(role)
	}
	process.Debugf("Connected state: %v.", state.Spec.Rotation.String())

	identity, err := process.GetIdentity(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotation := state.Spec.Rotation

	switch rotation.State {
	// rotation is on standby, so just use whatever is current
	case "", services.RotationStateStandby:
		// The roles of admin and auth are treaded in a special way, as in this case
		// the process does not need TLS clients and can use local auth directly.
		if role == teleport.RoleAdmin || role == teleport.RoleAuth {
			return &Connector{
				ClientIdentity: identity,
				ServerIdentity: identity,
				AuthServer:     process.getLocalAuth(),
			}, nil
		}
		log.Infof("Connecting to the cluster %v with TLS client certificate.", identity.ClusterName)
		client, err := process.newClient(process.Config.AuthServers, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Connector{Client: client, ClientIdentity: identity, ServerIdentity: identity}, nil
	case services.RotationStateInProgress:
		switch rotation.Phase {
		case services.RotationPhaseInit:
			// Both clients and servers are using old credentials,
			// this phase exists for remote clusters to propagate information about the new CA
			if role == teleport.RoleAdmin || role == teleport.RoleAuth {
				return &Connector{
					ClientIdentity: identity,
					ServerIdentity: identity,
					AuthServer:     process.getLocalAuth(),
				}, nil
			}
			client, err := process.newClient(process.Config.AuthServers, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &Connector{
				Client:         client,
				ClientIdentity: identity,
				ServerIdentity: identity,
			}, nil
		case services.RotationPhaseUpdateClients:
			// Clients should use updated credentials,
			// while servers should use old credentials to answer auth requests.
			newIdentity, err := process.storage.ReadIdentity(auth.IdentityReplacement, role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if role == teleport.RoleAdmin || role == teleport.RoleAuth {
				return &Connector{
					ClientIdentity: newIdentity,
					ServerIdentity: identity,
					AuthServer:     process.getLocalAuth(),
				}, nil
			}
			client, err := process.newClient(process.Config.AuthServers, newIdentity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &Connector{
				Client:         client,
				ClientIdentity: newIdentity,
				ServerIdentity: identity,
			}, nil
		case services.RotationPhaseUpdateServers:
			// Servers and clients are using new identity credentials, but the
			// identity is still set up to trust the old certificate authority certificates.
			newIdentity, err := process.storage.ReadIdentity(auth.IdentityReplacement, role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if role == teleport.RoleAdmin || role == teleport.RoleAuth {
				return &Connector{
					ClientIdentity: newIdentity,
					ServerIdentity: newIdentity,
					AuthServer:     process.getLocalAuth(),
				}, nil
			}
			client, err := process.newClient(process.Config.AuthServers, newIdentity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &Connector{
				Client:         client,
				ClientIdentity: newIdentity,
				ServerIdentity: newIdentity,
			}, nil
		case services.RotationPhaseRollback:
			// In rollback phase, clients and servers should switch back
			// to the old certificate authority-issued credentials,
			// but the new certificate authority should be trusted
			// because not all clients can update at the same time.
			if role == teleport.RoleAdmin || role == teleport.RoleAuth {
				return &Connector{
					ClientIdentity: identity,
					ServerIdentity: identity,
					AuthServer:     process.getLocalAuth(),
				}, nil
			}
			client, err := process.newClient(process.Config.AuthServers, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &Connector{
				Client:         client,
				ClientIdentity: identity,
				ServerIdentity: identity,
			}, nil
		default:
			return nil, trace.BadParameter("unsupported rotation phase: %q", rotation.Phase)
		}
	default:
		return nil, trace.BadParameter("unsupported rotation state: %q", rotation.State)
	}
}

func (process *TeleportProcess) firstTimeConnect(role teleport.Role) (*Connector, error) {
	id := auth.IdentityID{
		Role:     role,
		HostUUID: process.Config.HostUUID,
		NodeName: process.Config.Hostname,
	}
	additionalPrincipals, err := process.getAdditionalPrincipals(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var identity *auth.Identity
	if process.getLocalAuth() != nil {
		// Auth service is on the same host, no need to go though the invitation
		// procedure.
		process.Debugf("This server has local Auth server started, using it to add role to the cluster.")
		identity, err = auth.LocalRegister(id, process.getLocalAuth(), additionalPrincipals)
	} else {
		// Auth server is remote, so we need a provisioning token.
		if process.Config.Token == "" {
			return nil, trace.BadParameter("%v must join a cluster and needs a provisioning token", role)
		}
		process.Infof("Joining the cluster with a token %v.", process.Config.Token)
		identity, err = auth.Register(process.Config.DataDir, process.Config.Token, id, process.Config.AuthServers, additionalPrincipals)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%v has successfully registered with the cluster.", role)
	var connector *Connector
	if role == teleport.RoleAdmin || role == teleport.RoleAuth {
		connector = &Connector{
			ClientIdentity: identity,
			ServerIdentity: identity,
			AuthServer:     process.getLocalAuth(),
		}
	} else {
		client, err := process.newClient(process.Config.AuthServers, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connector = &Connector{
			ClientIdentity: identity,
			ServerIdentity: identity,
			Client:         client,
		}
	}

	// Sync local rotation state to match the remote rotation state.
	ca, err := connector.GetCertAuthority(services.CertAuthID{
		DomainName: connector.ClientIdentity.ClusterName,
		Type:       services.HostCA,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = process.storage.WriteIdentity(auth.IdentityCurrent, *identity)
	if err != nil {
		process.Warningf("Failed to write %v identity: %v.", role, err)
	}

	err = process.storage.WriteState(role, auth.StateV2{
		Spec: auth.StateSpecV2{
			Rotation: ca.GetRotation(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.Infof("The process has successfully wrote credentials and state of %v to disk.", role)
	return connector, nil
}

// periodicSyncRotationState checks rotation state periodically and
// takes action if necessary
func (process *TeleportProcess) periodicSyncRotationState() error {
	// start rotation only after teleport process has started
	eventC := make(chan Event, 1)
	process.WaitForEvent(process.ExitContext(), TeleportReadyEvent, eventC)
	select {
	case <-eventC:
		process.Infof("The new service has started successfully. Starting syncing rotation status with period %v.", process.Config.PollingPeriod)
	case <-process.ExitContext().Done():
		process.Infof("Periodic rotation sync has exited.")
		return nil
	}

	t := time.NewTicker(process.Config.PollingPeriod)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			status, err := process.syncRotationState()
			if err != nil {
				if trace.IsConnectionProblem(err) {
					process.Warningf("Connection problem: sync rotation state: %v.", err)
				} else {
					process.Warningf("Failed to sync rotation state: %v.", err)
				}
			} else {
				if status.phaseChanged || status.needsReload {
					process.Debugf("Sync rotation state detected cert authority reload phase update.")
				}
				if status.phaseChanged {
					process.BroadcastEvent(Event{Name: TeleportPhaseChangeEvent})
				}
				if status.needsReload {
					process.Debugf("Triggering reload process.")
					process.BroadcastEvent(Event{Name: TeleportReloadEvent})
					return nil
				}
			}
		case <-process.ExitContext().Done():
			process.Infof("Periodic rotation sync has exited because the process is shutting down.")
			return nil
		}
	}
}

// syncRotationState compares cluster rotation state with the state of
// internal services and performs the rotation if necessary.
func (process *TeleportProcess) syncRotationState() (*rotationStatus, error) {
	connectors := process.getConnectors()
	if len(connectors) == 0 {
		return nil, trace.BadParameter("no connectors found")
	}
	// it is important to use the same view of the certificate authority
	// for all internal services at the same time, so that the same
	// procedure will be applied at the same time for multiple service process
	// and no internal services is left behind.
	conn := connectors[0]
	ca, err := conn.GetCertAuthority(services.CertAuthID{
		DomainName: conn.ClientIdentity.ClusterName,
		Type:       services.HostCA,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var status rotationStatus
	for _, conn := range connectors {
		serviceStatus, err := process.syncServiceRotationState(ca, conn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if serviceStatus.needsReload {
			status.needsReload = true
		}
		if serviceStatus.phaseChanged {
			status.phaseChanged = true
		}
	}
	return &status, nil
}

// syncServiceRotationState syncs up rotation state for internal services (Auth, Proxy, Node) and
// if necessary, updates credentials. Returns true if the service will need to reload.
func (process *TeleportProcess) syncServiceRotationState(ca services.CertAuthority, conn *Connector) (*rotationStatus, error) {
	state, err := process.storage.GetState(conn.ClientIdentity.ID.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return process.rotate(conn, *state, ca.GetRotation())
}

type rotationStatus struct {
	// needsReload means that phase has been updated
	// and teleport process has to reload
	needsReload bool
	// phaseChanged means that teleport phase has been updated,
	// but teleport does not need reload
	phaseChanged bool
}

// rotate is called to check if rotation should be triggered.
func (process *TeleportProcess) rotate(conn *Connector, localState auth.StateV2, remote services.Rotation) (*rotationStatus, error) {
	id := conn.ClientIdentity.ID
	local := localState.Spec.Rotation

	additionalPrincipals, err := process.getAdditionalPrincipals(id.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	principalsChanged := len(additionalPrincipals) != 0 && !conn.ServerIdentity.HasPrincipals(additionalPrincipals)

	if local.Matches(remote) && !principalsChanged {
		// nothing to do, local state and rotation state are in sync
		return &rotationStatus{}, nil
	}

	storage := process.storage

	const outOfSync = "%v and cluster rotation state (%v) is out of sync with local (%v). Clear local state and re-register this %v."

	writeStateAndIdentity := func(name string, identity *auth.Identity) error {
		err = storage.WriteIdentity(name, *identity)
		if err != nil {
			return trace.Wrap(err)
		}
		localState.Spec.Rotation = remote
		err = storage.WriteState(id.Role, localState)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	switch remote.State {
	case "", services.RotationStateStandby:
		switch local.State {
		// There is nothing to do, it could happen
		// that the old node came up and missed the whole rotation
		// rollback cycle.
		case "", services.RotationStateStandby:
			if principalsChanged {
				process.Infof("Service %v has updated principals to %q, going to request new principals and update.", id.Role, additionalPrincipals)
				identity, err := conn.ReRegister(additionalPrincipals)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				err = storage.WriteIdentity(auth.IdentityCurrent, *identity)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return &rotationStatus{needsReload: true}, nil
			}
			return &rotationStatus{}, nil
		case services.RotationStateInProgress:
			// Rollback phase has been completed, all services
			// will receive new identities.
			if local.Phase != services.RotationPhaseRollback && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			identity, err := conn.ReRegister(additionalPrincipals)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(auth.IdentityCurrent, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &rotationStatus{needsReload: true}, nil
		default:
			return nil, trace.BadParameter("unsupported state: %q", localState)
		}
	case services.RotationStateInProgress:
		switch remote.Phase {
		case services.RotationPhaseStandby, "":
			// There is nothing to do.
			return &rotationStatus{}, nil
		case services.RotationPhaseInit:
			// Only allow transition in case if local rotation state is standby
			// so this server is in the "clean" state.
			if local.State != services.RotationStateStandby && local.State != "" {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			// only update local phase, there is no need to reload
			localState.Spec.Rotation = remote
			err = storage.WriteState(id.Role, localState)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &rotationStatus{phaseChanged: true}, nil
		case services.RotationPhaseUpdateClients:
			// Allow transition to this phase only if the previous
			// phase was "Init".
			if local.Phase != services.RotationPhaseInit && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			identity, err := conn.ReRegister(additionalPrincipals)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(auth.IdentityReplacement, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Require reload of teleport process to update client and servers.
			return &rotationStatus{needsReload: true}, nil
		case services.RotationPhaseUpdateServers:
			// Allow transition to this phase only if the previous
			// phase was "Update clients".
			if local.Phase != services.RotationPhaseUpdateClients && local.CurrentID != remote.CurrentID {
				return nil, trace.CompareFailed(outOfSync, id.Role, remote, local, id.Role)
			}
			// Write the replacement identity as a current identity and reload the server.
			replacement, err := storage.ReadIdentity(auth.IdentityReplacement, id.Role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(auth.IdentityCurrent, replacement)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Require reload of teleport process to update servers.
			return &rotationStatus{needsReload: true}, nil
		case services.RotationPhaseRollback:
			// Allow transition to this phase from any other local phase
			// because it will be widely used to recover cluster state to
			// the previously valid state, client will re-register to receive
			// credentials signed by the "old" CA.
			identity, err := conn.ReRegister(additionalPrincipals)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			err = writeStateAndIdentity(auth.IdentityCurrent, identity)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Require reload of teleport process to update servers.
			return &rotationStatus{needsReload: true}, nil
		default:
			return nil, trace.BadParameter("unsupported phase: %q", remote.Phase)
		}
	default:
		return nil, trace.BadParameter("unsupported state: %q", remote.State)
	}
}

func (process *TeleportProcess) newClient(authServers []utils.NetAddr, identity *auth.Identity) (*auth.Client, error) {
	tlsConfig, err := identity.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if process.Config.ClientTimeout != 0 {
		return auth.NewTLSClient(authServers, tlsConfig, auth.ClientTimeout(process.Config.ClientTimeout))
	}
	return auth.NewTLSClient(authServers, tlsConfig)
}
