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

package auth

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

// RotateRequest is a request to start rotation of the certificate authority.

// rotationReq is an internal rotation request
type rotationReq struct {
	// clock implements test or real wall clock
	clock clockwork.Clock
	// ca is a certificate authority to rotate
	ca types.CertAuthority
	// targetPhase is a target rotation phase to set
	targetPhase string
	// mode is a rotation mode
	mode string
	// gracePeriod is a rotation grace period
	gracePeriod time.Duration
	// schedule is a schedule to set
	schedule types.RotationSchedule
	// privateKey is passed by tests to supply private key for cert authorities
	// instead of generating them on each iteration
	privateKey []byte
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
//
// Rotation procedure is based on the state machine approach.
//
// Here are the supported rotation states:
//
//   - Standby - the cluster is in standby mode and ready to take action.
//   - In-progress - cluster CA rotation is in progress.
//
// In-progress state is split into multiple phases and the cluster
// can traverse between phases using supported transitions.
//
// Here are the supported phases:
//
// * Standby - no action is taken.
//
// * Init - New CAs are issued, but all internal system clients
// and servers are still using the old certificates. New CAs are trusted,
// but are not used. New components that are joining the cluster
// are issued certificates signed by "old" CAs.
//
// This phase is necessary for remote clusters to fetch new certificate authorities,
// otherwise remote clusters will be locked out, because they won't have a chance
// to discover the new certificate authorities to be issued.
//
// * Update Clients - All internal system clients
// have to reconnect and receive the new credentials, but all servers
// TLS, SSH and Proxies will still use old credentials.
// Certs from old CA and new CA are trusted within the system.
// This phase is necessary because old clients should receive new credentials
// from the auth servers. If this phase did not exist, old clients could not
// trust servers serving new credentials, because old clients did not receive
// new information yet. It is possible to transition from this phase to phase
// "Update servers" or "Rollback".
//
// * Update Servers - triggers all internal system components to reload and use
// new credentials both in the internal clients and servers, however
// old CA issued credentials are still trusted. This is done to make it possible
// for old components to be trusted within the system, to make rollback possible.
// It is possible to transition from this phase to "Rollback" or "Standby".
// When transitioning to "Standby" phase, the rotation is considered completed,
// old CA is removed from the system and components reload again,
// but this time they don't trust old CA any more.
//
// * Rollback phase is used to revert any changes. When going to rollback phase
// the newly issued CA is no longer used, but set up as trusted,
// so components can reload and receive credentials issued by "old" CA back.
// This phase is useful when administrator makes a mistake, or there are some
// offline components that will lose the connection in case if rotation
// completes. It is only possible to transition from this phase to "Standby".
// When transitioning to "Standby" phase from "Rollback" phase, all components
// reload again, but the "new" CA is discarded and is no longer trusted,
// cluster goes back to the original state.
//
// # Rotation modes
//
// There are two rotation modes supported - manual or automatic.
//
// * Manual mode allows administrators to transition between
// phases explicitly setting a phase on every request.
//
// * Automatic mode performs automatic transition between phases
// on a given schedule. Schedule is a time table
// that specifies exact date when the next phase should take place. If automatic
// transition between any phase fails, the rotation switches back to the manual
// mode and stops execution phases on the schedule. If schedule is not specified,
// it will be auto generated based on the "grace period" duration parameter,
// and time between all phases will be evenly split over the grace period duration.
//
// It is possible to switch from automatic to manual by setting the phase
// to the rollback phase.
func (a *Server) RotateCertAuthority(ctx context.Context, req types.RotateRequest) error {
	if err := req.CheckAndSetDefaults(a.clock); err != nil {
		return trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	existing, err := a.Services.GetCertAuthority(ctx, types.CertAuthID{
		Type:       req.Type,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return trace.Wrap(err)
	}

	rotated, err := a.processRotationRequest(ctx, rotationReq{
		ca:          existing,
		clock:       a.clock,
		targetPhase: req.TargetPhase,
		schedule:    *req.Schedule,
		gracePeriod: *req.GracePeriod,
		mode:        req.Mode,
		privateKey:  a.privateKey,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := a.UpdateCertAuthority(ctx, rotated); err != nil {
		return trace.Wrap(err)
	}
	rotation := rotated.GetRotation()
	switch rotation.State {
	case types.RotationStateInProgress:
		log.WithFields(logrus.Fields{"type": req.Type}).Infof("Updated rotation state, set current phase to: %q.", rotation.Phase)
	case types.RotationStateStandby:
		log.WithFields(logrus.Fields{"type": req.Type}).Infof("Updated and completed rotation.")
	}

	return nil
}

// autoRotateCertAuthorities automatically rotates cert authorities,
// does nothing if no rotation parameters were set up
// or it is too early to rotate per schedule
func (a *Server) autoRotateCertAuthorities(ctx context.Context) error {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	usableKeysResults := make(map[types.CertAuthType]*keystore.UsableKeysResult)
	for _, caType := range types.CertAuthTypes {
		ca, err := a.Services.GetCertAuthority(ctx, types.CertAuthID{
			Type:       caType,
			DomainName: clusterName.GetClusterName(),
		}, true)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := a.autoRotate(ctx, ca); err != nil {
			return trace.Wrap(err)
		}
		// make sure there are local AdditionalKeys during init phase of rotation
		if ca.GetRotation().Phase == types.RotationPhaseInit {
			if err := a.ensureLocalAdditionalKeys(ctx, ca); err != nil {
				return trace.Wrap(err)
			}
		}
		usableKeysResults[caType], err = a.keyStore.HasUsableActiveKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err := a.syncUsableKeysAlert(ctx, usableKeysResults); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (a *Server) autoRotate(ctx context.Context, ca types.CertAuthority) error {
	rotation := ca.GetRotation()
	// rotation mode is not automatic, nothing to do
	if rotation.Mode != types.RotationModeAuto {
		return nil
	}
	// rotation is not in progress, there is nothing to do
	if rotation.State != types.RotationStateInProgress {
		return nil
	}
	logger := log.WithFields(logrus.Fields{"type": ca.GetType()})
	var req *rotationReq
	switch rotation.Phase {
	case types.RotationPhaseInit:
		if rotation.Schedule.UpdateClients.After(a.clock.Now()) {
			return nil
		}
		req = &rotationReq{
			clock:       a.clock,
			ca:          ca,
			targetPhase: types.RotationPhaseUpdateClients,
			mode:        types.RotationModeAuto,
			gracePeriod: rotation.GracePeriod.Duration(),
			schedule:    rotation.Schedule,
		}
	case types.RotationPhaseUpdateClients:
		if rotation.Schedule.UpdateServers.After(a.clock.Now()) {
			return nil
		}
		req = &rotationReq{
			clock:       a.clock,
			ca:          ca,
			targetPhase: types.RotationPhaseUpdateServers,
			mode:        types.RotationModeAuto,
			gracePeriod: rotation.GracePeriod.Duration(),
			schedule:    rotation.Schedule,
		}
	case types.RotationPhaseUpdateServers:
		if rotation.Schedule.Standby.After(a.clock.Now()) {
			return nil
		}
		req = &rotationReq{
			clock:       a.clock,
			ca:          ca,
			targetPhase: types.RotationPhaseStandby,
			mode:        types.RotationModeAuto,
			gracePeriod: rotation.GracePeriod.Duration(),
			schedule:    rotation.Schedule,
		}
	default:
		return trace.BadParameter("phase is not supported: %q", rotation.Phase)
	}
	logger.Infof("Setting rotation phase to %q", req.targetPhase)
	rotated, err := a.processRotationRequest(ctx, *req)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := a.UpdateCertAuthority(ctx, rotated); err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("Cert authority rotation request is completed")
	return nil
}

// processRotationRequest processes rotation request based on the target and
// current phase and state.
func (a *Server) processRotationRequest(ctx context.Context, req rotationReq) (types.CertAuthority, error) {
	rotation := req.ca.GetRotation()
	ca := req.ca.Clone()

	switch req.targetPhase {
	case types.RotationPhaseInit:
		// This is the first stage of the rotation - new certificate authorities
		// are being generated, but no components are using them yet
		switch rotation.State {
		case types.RotationStateStandby, "":
		default:
			return nil, trace.BadParameter("can not initiate rotation while another is in progress")
		}
		if err := a.startNewRotation(ctx, req, ca); err != nil {
			return nil, trace.Wrap(err)
		}
		return ca, nil
	case types.RotationPhaseUpdateClients:
		// Update client phase clients will start using new credentials
		// and servers will use the existing credentials, but will trust clients
		// with both old and new credentials.
		if rotation.Phase != types.RotationPhaseInit {
			return nil, trace.BadParameter(
				"can only switch to phase %v from %v, current phase is %v",
				types.RotationPhaseUpdateClients,
				types.RotationPhaseInit,
				rotation.Phase)
		}
		if err := updateClients(ca, req.mode); err != nil {
			return nil, trace.Wrap(err)
		}
		return ca, nil
	case types.RotationPhaseUpdateServers:
		// Update server phase uses the new credentials both for servers
		// and clients, but still trusts clients with old credentials.
		if rotation.Phase != types.RotationPhaseUpdateClients {
			return nil, trace.BadParameter(
				"can only switch to phase %v from %v, current phase is %v",
				types.RotationPhaseUpdateServers,
				types.RotationPhaseUpdateClients,
				rotation.Phase)
		}
		// Signal nodes to restart and start serving new signatures
		// by updating the phase.
		rotation.Phase = req.targetPhase
		rotation.Mode = req.mode
		ca.SetRotation(rotation)
		return ca, nil
	case types.RotationPhaseRollback:
		// Rollback moves back both clients and servers to use the old credentials
		// but will trust new credentials.
		switch rotation.Phase {
		case types.RotationPhaseInit, types.RotationPhaseUpdateClients, types.RotationPhaseUpdateServers:
			if err := startRollingBackRotation(ca); err != nil {
				return nil, trace.Wrap(err)
			}
			return ca, nil
		default:
			return nil, trace.BadParameter("can not transition to phase %q from %q phase.", req.targetPhase, rotation.Phase)
		}
	case types.RotationPhaseStandby:
		// Transition to the standby phase moves rotation process
		// to standby, servers will only trust one certificate authority.
		switch rotation.Phase {
		case types.RotationPhaseUpdateServers, types.RotationPhaseRollback:
			completeRotation(req.clock, ca)
			return ca, nil
		default:
			return nil, trace.BadParameter("can not transition to phase %q from %q phase.", req.targetPhase, rotation.Phase)
		}
	default:
		return nil, trace.BadParameter("unsupported phase: %q", req.targetPhase)
	}
}

// startNewRotation starts new rotation. In this phase requests will continue
// to be signed by the old CAKeySet, but a new CAKeySet will be added. This new
// CA can be used to verify requests.
func (a *Server) startNewRotation(ctx context.Context, req rotationReq, ca types.CertAuthority) error {
	clock := req.clock
	gracePeriod := req.gracePeriod

	rotation := ca.GetRotation()
	id := uuid.New().String()

	rotation.Mode = req.mode
	rotation.Schedule = req.schedule

	activeKeys := ca.GetActiveKeys()
	additionalKeys := ca.GetAdditionalTrustedKeys()
	var newKeys types.CAKeySet

	// generate keys and certificates:
	if len(req.privateKey) != 0 {
		log.Infof("Generating CA, using pregenerated test private key.")

		signer, err := keys.ParsePrivateKey(req.privateKey)
		if err != nil {
			return trace.Wrap(err)
		}

		if len(activeKeys.SSH) > 0 {
			sshSigner, err := ssh.NewSignerFromKey(signer)
			if err != nil {
				return trace.Wrap(err)
			}
			sshPublicKey := ssh.MarshalAuthorizedKey(sshSigner.PublicKey())
			newKeys.SSH = append(newKeys.SSH, &types.SSHKeyPair{
				PublicKey:      sshPublicKey,
				PrivateKey:     req.privateKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
			})
		}

		if len(activeKeys.TLS) > 0 {
			tlsCert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
				Signer: signer,
				Entity: pkix.Name{
					CommonName:   ca.GetClusterName(),
					Organization: []string{ca.GetClusterName()},
				},
				TTL:   defaults.CATTL,
				Clock: req.clock,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			newKeys.TLS = append(newKeys.TLS, &types.TLSKeyPair{
				Cert:    tlsCert,
				Key:     req.privateKey,
				KeyType: types.PrivateKeyType_RAW,
			})
		}

		if len(activeKeys.JWT) > 0 {
			jwtPublicKey, err := keys.MarshalPublicKey(signer.Public())
			if err != nil {
				return trace.Wrap(err)
			}
			jwtPrivateKey, err := keys.MarshalPrivateKey(signer)
			if err != nil {
				return trace.Wrap(err)
			}
			newKeys.JWT = append(newKeys.JWT, &types.JWTKeyPair{
				PublicKey:      jwtPublicKey,
				PrivateKey:     jwtPrivateKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
			})
		}
	} else {
		if !additionalKeys.Empty() {
			// Special case where a new HSM auth server is coming up and has
			// already added local AdditionalTrustedKeys during the standby
			// phase. Keep the existing AdditionalTrustedKeys to avoid
			// invalidating the current Admin identity.
			newKeys = additionalKeys.Clone()
		}
		usableKeysResult, err := a.keyStore.HasUsableAdditionalKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
		if !usableKeysResult.CAHasPreferredKeyType {
			// There are no AdditionalTrustedKeys in this CA that match the
			// configured key type of this auth server.
			// This is one of 2 cases:
			// 1. There are no AdditionalTrustedKeys at all.
			// 2. There are AdditionalTrustedKeys which were added by a
			//    different HSM-enabled auth server or one using a different KMS.
			// In either case, we need to add newly generated keys.
			newLocalKeys, err := newKeySet(ctx, a.keyStore, ca.GetID())
			if err != nil {
				return trace.Wrap(err)
			}
			newKeys = mergeKeySets(newLocalKeys, newKeys)
		}
	}

	rotation.Started = clock.Now().UTC()
	rotation.GracePeriod = types.NewDuration(gracePeriod)
	rotation.CurrentID = id

	// If no grace period was set, drop old certificate authority without keeping
	// it as trusted.
	//
	// If a grace period was set, in the initial phase of rotation keeps old CAs
	// as primary signing key pairs, and generates new CAs that are trusted, but
	// not used in the cluster.
	if gracePeriod == 0 {
		if err := ca.SetActiveKeys(newKeys); err != nil {
			return trace.Wrap(err)
		}
		// In case of forced rotation, rotation has been started and completed
		// in the same step moving it to standby state.
		rotation.State = types.RotationStateStandby
		rotation.Phase = types.RotationPhaseStandby
	} else {
		if err := ca.SetAdditionalTrustedKeys(newKeys); err != nil {
			return trace.Wrap(err)
		}
		rotation.State = types.RotationStateInProgress
		rotation.Phase = types.RotationPhaseInit
	}

	ca.SetRotation(rotation)

	return nil
}

// updateClients swaps old and new CA key sets.
//
//   - Old CAs continue to be trusted, but are no longer used for signing.
//   - New CAs are used for signing.
//   - Remote components will reload with new certificates used for client
//     connections.
func updateClients(ca types.CertAuthority, mode string) error {
	oldActive, oldTrusted := ca.GetActiveKeys(), ca.GetAdditionalTrustedKeys()
	if err := ca.SetActiveKeys(oldTrusted); err != nil {
		return trace.Wrap(err)
	}
	if err := ca.SetAdditionalTrustedKeys(oldActive); err != nil {
		return trace.Wrap(err)
	}

	rotation := ca.GetRotation()
	rotation.State = types.RotationStateInProgress
	rotation.Phase = types.RotationPhaseUpdateClients
	rotation.Mode = mode
	ca.SetRotation(rotation)
	return nil
}

// startRollingBackRotation starts roll back to the original state. Will move
// old CAKeySet back as active.
//
// Will keep the new CAKeySet around as trusted during rollback phase, both
// types of clients may be present in the cluster.
func startRollingBackRotation(ca types.CertAuthority) error {
	rotation := ca.GetRotation()

	// if rolling back from the init phase, active and trusted keys have not yet
	// been swapped
	if rotation.Phase != types.RotationPhaseInit {
		oldActive, oldTrusted := ca.GetActiveKeys(), ca.GetAdditionalTrustedKeys()
		if err := ca.SetActiveKeys(oldTrusted); err != nil {
			return trace.Wrap(err)
		}
		if err := ca.SetAdditionalTrustedKeys(oldActive); err != nil {
			return trace.Wrap(err)
		}
	}

	// Rollback always sets rotation to manual mode.
	rotation.Mode = types.RotationModeManual
	rotation.State = types.RotationStateInProgress
	rotation.Phase = types.RotationPhaseRollback
	ca.SetRotation(rotation)
	return nil
}

// completeRotation completes the certificate authority rotation by removing
// the new CA as trusted.
func completeRotation(clock clockwork.Clock, ca types.CertAuthority) {
	ca.SetAdditionalTrustedKeys(types.CAKeySet{})

	rotation := ca.GetRotation()
	rotation.Started = time.Time{}
	rotation.State = types.RotationStateStandby
	rotation.Phase = types.RotationPhaseStandby
	rotation.LastRotated = clock.Now().UTC()
	rotation.Mode = ""
	rotation.Schedule = types.RotationSchedule{}
	ca.SetRotation(rotation)
}

// syncUsableKeysAlert creates a cluster alert if any of the stored CAs do not
// contain keys matching the type of key (HSM, KMS, software) this auth server
// is configured to use. The [usableKeysResults] arguments is expected to
// contain the results of [keystore.(*Manager).HasUsableActiveKeys] for all CA
// types.
func (a *Server) syncUsableKeysAlert(ctx context.Context, usableKeysResults map[types.CertAuthType]*keystore.UsableKeysResult) error {
	// Alert ID contains server ID because multiple auth servers can be
	// configured differently and may be able to use different key types.
	// If the auth servers are ephemeral, the alert will expire.
	alertID := "ca-key-types/" + a.ServerID
	var casWithoutPreferredKeyType []types.CertAuthType
	unableToSign := false
	var preferredKeyType string
	for caType, usableKeysResult := range usableKeysResults {
		if !usableKeysResult.CAHasPreferredKeyType {
			casWithoutPreferredKeyType = append(casWithoutPreferredKeyType, caType)
		}
		if !usableKeysResult.CAHasUsableKeys {
			unableToSign = true
		}
		// Should be identical for all results, just take any one.
		preferredKeyType = usableKeysResult.PreferredKeyType
	}

	if len(casWithoutPreferredKeyType) == 0 {
		// Every CA contains keys matching the preferred type, delete the alert
		// if it exists.
		if err := a.DeleteClusterAlert(ctx, alertID); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil
	}

	alertOptions := []types.AlertOption{
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		// This is called by a.runPeriodicOperations via
		// a.autoRotateCertAuthorities on a random period between 1-2x
		// defaults.HighResPollingPeriod, the alert will be renewed before it
		// expires if it's still relevant.
		types.WithAlertExpires(a.clock.Now().Add(defaults.HighResPollingPeriod * 3)),
	}
	msg := fmt.Sprintf(
		"Auth Service %s is configured to use %s, but the following CAs do not contain any keys of that type: %v. ",
		a.ServerID,
		preferredKeyType,
		casWithoutPreferredKeyType)
	if unableToSign {
		alertOptions = append(alertOptions,
			types.WithAlertSeverity(types.AlertSeverity_HIGH),
			types.WithAlertLabel(types.AlertPermitAll, "yes"))
		msg += "The Auth Service is currently unable to sign certificates and degraded service is expected. "
	} else {
		if modules.GetModules().Features().Cloud {
			// Don't create this alert on Cloud. This avoids alerting all
			// customers if Cloud ends up enabling an HSM/KMS by default in
			// existing configurations. It's fine to never rotate in this case
			// and continue using software keys. But if this is an on-prem
			// cluster where the admin manually configured an HSM or KMS, they
			// probably want to use it, so hopefully they'll appreciate the
			// alert reminding them to rotate the CAs.
			return nil
		}
		alertOptions = append(alertOptions,
			types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
			types.WithAlertLabel(types.AlertVerbPermit,
				fmt.Sprintf("%s:%s", types.KindCertAuthority, types.VerbUpdate)))
		msg += "The Auth Service will continue signing certificates with raw software keys. "
	}
	msg += "These CAs must be rotated to begin using the configured key type. " +
		"See https://goteleport.com/docs/management/operations/ca-rotation/"

	alert, err := types.NewClusterAlert("ca-key-types/"+a.ServerID, msg, alertOptions...)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.UpsertClusterAlert(ctx, alert); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
