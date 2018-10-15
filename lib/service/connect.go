package service

import (
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
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

func (process *TeleportProcess) connect(role teleport.Role) (conn *Connector, err error) {
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
	// TODO(klizhentas): REMOVE IN 3.1
	// this is a migration clutch, used to re-register
	// in case if identity of the auth server does not have the wildcard cert
	if role == teleport.RoleAdmin || role == teleport.RoleAuth {
		if !identity.HasPrincipals([]string{"*." + teleport.APIDomain}) {
			process.Debugf("Detected Auth server certificate without wildcard, regenerating.")
			return process.firstTimeConnect(role)
		}
	}

	rotation := state.Spec.Rotation

	switch rotation.State {
	// rotation is on standby, so just use whatever is current
	case "", services.RotationStateStandby:
		// The roles of admin and auth are treated in a special way, as in this case
		// the process does not need TLS clients and can use local auth directly.
		if role == teleport.RoleAdmin || role == teleport.RoleAuth {
			return &Connector{
				ClientIdentity: identity,
				ServerIdentity: identity,
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

// KeyPair is a private/public key pair
type KeyPair struct {
	// PrivateKey is a private key in PEM format
	PrivateKey []byte
	// PublicSSHKey is a public key in SSH format
	PublicSSHKey []byte
	// PublicTLSKey is a public key in X509 format
	PublicTLSKey []byte
}

func (process *TeleportProcess) deleteKeyPair(role teleport.Role, reason string) {
	process.keyMutex.Lock()
	defer process.keyMutex.Unlock()
	process.Debugf("Deleted generated key pair %v %v.", role, reason)
	delete(process.keyPairs, keyPairKey{role: role, reason: reason})
}

func (process *TeleportProcess) generateKeyPair(role teleport.Role, reason string) (*KeyPair, error) {
	process.keyMutex.Lock()
	defer process.keyMutex.Unlock()

	mapKey := keyPairKey{role: role, reason: reason}
	keyPair, ok := process.keyPairs[mapKey]
	if ok {
		process.Debugf("Returning existing key pair for %v %v.", role, reason)
		return &keyPair, nil
	}
	process.Debugf("Generating new key pair for %v %v.", role, reason)
	privPEM, pubSSH, err := process.Config.Keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := ssh.ParseRawPrivateKey(privPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubTLS, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPair = KeyPair{PrivateKey: privPEM, PublicSSHKey: pubSSH, PublicTLSKey: pubTLS}
	process.keyPairs[mapKey] = keyPair

	return &keyPair, nil
}

// getCertAuthority returns cert authority by ID.
// In case if auth servers, the role is 'TeleportAdmin' and instead of using
// TLS client this method uses the local auth server.
func (process *TeleportProcess) getCertAuthority(conn *Connector, id services.CertAuthID, loadPrivateKeys bool) (services.CertAuthority, error) {
	if conn.ClientIdentity.ID.Role == teleport.RoleAdmin || conn.ClientIdentity.ID.Role == teleport.RoleAuth {
		return process.localAuth.GetCertAuthority(id, loadPrivateKeys)
	}
	return conn.Client.GetCertAuthority(id, loadPrivateKeys)
}

// reRegister receives new identity credentials for proxy, node and auth.
// In case if auth servers, the role is 'TeleportAdmin' and instead of using
// TLS client this method uses the local auth server.
func (process *TeleportProcess) reRegister(conn *Connector, additionalPrincipals []string) (*auth.Identity, error) {
	if conn.ClientIdentity.ID.Role == teleport.RoleAdmin || conn.ClientIdentity.ID.Role == teleport.RoleAuth {
		return auth.GenerateIdentity(process.localAuth, conn.ClientIdentity.ID, additionalPrincipals)
	}
	const reason = "re-register"
	keyPair, err := process.generateKeyPair(conn.ClientIdentity.ID.Role, reason)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := auth.ReRegister(auth.ReRegisterParams{
		Client:               conn.Client,
		ID:                   conn.ClientIdentity.ID,
		AdditionalPrincipals: additionalPrincipals,
		PrivateKey:           keyPair.PrivateKey,
		PublicTLSKey:         keyPair.PublicTLSKey,
		PublicSSHKey:         keyPair.PublicSSHKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.deleteKeyPair(conn.ClientIdentity.ID.Role, reason)
	return identity, nil
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
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Auth server is remote, so we need a provisioning token.
		if process.Config.Token == "" {
			return nil, trace.BadParameter("%v must join a cluster and needs a provisioning token", role)
		}
		process.Infof("Joining the cluster with a secure token.")
		const reason = "first-time-connect"
		keyPair, err := process.generateKeyPair(role, reason)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity, err = auth.Register(auth.RegisterParams{
			DataDir:              process.Config.DataDir,
			Token:                process.Config.Token,
			ID:                   id,
			Servers:              process.Config.AuthServers,
			AdditionalPrincipals: additionalPrincipals,
			PrivateKey:           keyPair.PrivateKey,
			PublicTLSKey:         keyPair.PublicTLSKey,
			PublicSSHKey:         keyPair.PublicSSHKey,
			CipherSuites:         process.Config.CipherSuites,
			CAPin:                process.Config.CAPin,
			CAPath:               filepath.Join(defaults.DataDir, defaults.CACertFile),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		process.deleteKeyPair(role, reason)
	}

	log.Infof("%v has successfully registered with the cluster.", role)
	var connector *Connector
	if role == teleport.RoleAdmin || role == teleport.RoleAuth {
		connector = &Connector{
			ClientIdentity: identity,
			ServerIdentity: identity,
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
	ca, err := process.getCertAuthority(connector, services.CertAuthID{
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
	ca, err := process.getCertAuthority(conn, services.CertAuthID{
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
				identity, err := process.reRegister(conn, additionalPrincipals)
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
			identity, err := process.reRegister(conn, additionalPrincipals)
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
			identity, err := process.reRegister(conn, additionalPrincipals)
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
			identity, err := process.reRegister(conn, additionalPrincipals)
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
	tlsConfig, err := identity.TLSConfig(process.Config.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if process.Config.ClientTimeout != 0 {
		return auth.NewTLSClient(authServers, tlsConfig, auth.ClientTimeout(process.Config.ClientTimeout))
	}
	return auth.NewTLSClient(authServers, tlsConfig)
}
