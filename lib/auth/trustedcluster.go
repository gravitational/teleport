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
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// UpsertTrustedCluster creates or toggles a Trusted Cluster relationship.
func (a *Server) UpsertTrustedCluster(ctx context.Context, trustedCluster types.TrustedCluster) (newTrustedCluster types.TrustedCluster, returnErr error) {
	// It is recommended to omit trusted cluster name because the trusted cluster name
	// is updated to the roots cluster name during the handshake with the root cluster.
	var existingCluster types.TrustedCluster
	if trustedCluster.GetName() != "" {
		var err error
		existingCluster, err = a.GetTrustedCluster(ctx, trustedCluster.GetName())
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	enable := trustedCluster.GetEnabled()

	// If the trusted cluster already exists in the backend, make sure it's a
	// valid state change client is trying to make.
	if existingCluster != nil {
		if err := existingCluster.CanChangeStateTo(trustedCluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	logger := log.WithField("trusted_cluster", trustedCluster.GetName())

	// change state
	if err := a.checkLocalRoles(ctx, trustedCluster.GetRoleMap()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Update role map
	if existingCluster != nil && !cmp.Equal(existingCluster.GetRoleMap(), trustedCluster.GetRoleMap()) {
		if err := a.UpdateUserCARoleMap(ctx, existingCluster.GetName(), trustedCluster.GetRoleMap(),
			existingCluster.GetEnabled()); err != nil {
			return nil, trace.Wrap(err)
		}

		// Reset previous UserCA role map if this func fails later on
		defer func() {
			if returnErr != nil {
				if err := a.UpdateUserCARoleMap(ctx, trustedCluster.GetName(), existingCluster.GetRoleMap(),
					trustedCluster.GetEnabled()); err != nil {
					returnErr = trace.NewAggregate(err, returnErr)
				}
			}
		}()
	}
	// Create or update state
	switch {
	case existingCluster != nil && enable == true:
		if existingCluster.GetEnabled() {
			break
		}
		log.Debugf("Enabling existing Trusted Cluster relationship.")

		if err := a.activateCertAuthority(ctx, trustedCluster); err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.BadParameter("enable only supported for Trusted Clusters created with Teleport 2.3 and above")
			}
			return nil, trace.Wrap(err)
		}

		if err := a.createReverseTunnel(trustedCluster); err != nil {
			return nil, trace.Wrap(err)
		}
	case existingCluster != nil && enable == false:
		if !existingCluster.GetEnabled() {
			break
		}
		log.Debugf("Disabling existing Trusted Cluster relationship.")

		if err := a.deactivateCertAuthority(ctx, trustedCluster); err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.BadParameter("enable only supported for Trusted Clusters created with Teleport 2.3 and above")
			}
			return nil, trace.Wrap(err)
		}

		if err := a.DeleteReverseTunnel(trustedCluster.GetName()); err != nil {
			return nil, trace.Wrap(err)
		}
	case existingCluster == nil && enable == true:
		logger.Info("Creating enabled Trusted Cluster relationship.")

		remoteCAs, err := a.establishTrust(ctx, trustedCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Force name of the trusted cluster resource
		// to be equal to the name of the remote cluster it is connecting to.
		trustedCluster.SetName(remoteCAs[0].GetClusterName())

		if err := a.addCertAuthorities(ctx, trustedCluster, remoteCAs); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := a.createReverseTunnel(trustedCluster); err != nil {
			return nil, trace.Wrap(err)
		}

	case existingCluster == nil && enable == false:
		logger.Info("Creating disabled Trusted Cluster relationship.")

		remoteCAs, err := a.establishTrust(ctx, trustedCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Force name to the name of the trusted cluster.
		trustedCluster.SetName(remoteCAs[0].GetClusterName())

		if err := a.addCertAuthorities(ctx, trustedCluster, remoteCAs); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := a.deactivateCertAuthority(ctx, trustedCluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tc, err := a.Services.UpsertTrustedCluster(ctx, trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.TrustedClusterCreate{
		Metadata: apievents.Metadata{
			Type: events.TrustedClusterCreateEvent,
			Code: events.TrustedClusterCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: trustedCluster.GetName(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		logger.WithError(err).Warn("Failed to emit trusted cluster create event.")
	}

	return tc, nil
}

func (a *Server) checkLocalRoles(ctx context.Context, roleMap types.RoleMap) error {
	for _, mapping := range roleMap {
		for _, localRole := range mapping.Local {
			// expansion means dynamic mapping is in place,
			// so local role is undefined
			if utils.ContainsExpansion(localRole) {
				continue
			}
			_, err := a.GetRole(ctx, localRole)
			if err != nil {
				if trace.IsNotFound(err) {
					return trace.NotFound("a role %q referenced in a mapping %v:%v is not defined", localRole, mapping.Remote, mapping.Local)
				}
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// DeleteTrustedCluster removes types.CertAuthority, services.ReverseTunnel,
// and services.TrustedCluster resources.
func (a *Server) DeleteTrustedCluster(ctx context.Context, name string) error {
	cn, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	// This check ensures users are not deleting their root/own cluster.
	if cn.GetClusterName() == name {
		return trace.BadParameter("trusted cluster %q is the name of this root cluster and cannot be removed.", name)
	}

	// err on the safe side and delete all possible CA types.
	var ids []types.CertAuthID
	for _, caType := range types.CertAuthTypes {
		ids = append(ids, types.CertAuthID{
			Type:       caType,
			DomainName: name,
		})
	}

	if err := a.DeleteCertAuthorities(ctx, ids...); err != nil {
		return trace.Wrap(err)
	}

	if err := a.DeleteReverseTunnel(name); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	if err := a.Services.DeleteTrustedCluster(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.TrustedClusterDelete{
		Metadata: apievents.Metadata{
			Type: events.TrustedClusterDeleteEvent,
			Code: events.TrustedClusterDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit trusted cluster delete event.")
	}

	return nil
}

func (a *Server) establishTrust(ctx context.Context, trustedCluster types.TrustedCluster) ([]types.CertAuthority, error) {
	var localCertAuthorities []types.CertAuthority

	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get a list of certificate authorities for this auth server
	allLocalCAs, err := a.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, lca := range allLocalCAs {
		if lca.GetClusterName() == domainName {
			localCertAuthorities = append(localCertAuthorities, lca)
		}
	}

	// create a request to validate a trusted cluster (token and local certificate authorities)
	validateRequest := authclient.ValidateTrustedClusterRequest{
		Token:           trustedCluster.GetToken(),
		CAs:             localCertAuthorities,
		TeleportVersion: teleport.Version,
	}

	// log the local certificate authorities that we are sending
	log.Infof("Sending validate request; token=%s, CAs=%v", backend.MaskKeyName(validateRequest.Token), validateRequest.CAs)

	// send the request to the remote auth server via the proxy
	validateResponse, err := a.sendValidateRequestToProxy(trustedCluster.GetProxyAddress(), &validateRequest)
	if err != nil {
		log.Error(err)
		if strings.Contains(err.Error(), "x509") {
			return nil, trace.AccessDenied("the trusted cluster uses misconfigured HTTP/TLS certificate.")
		}
		return nil, trace.Wrap(err)
	}

	// log the remote certificate authorities we are adding
	log.Infof("Received validate response; CAs=%v", validateResponse.CAs)

	for _, ca := range validateResponse.CAs {
		for _, keyPair := range ca.GetActiveKeys().TLS {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			remoteClusterName, err := tlsca.ClusterName(cert.Subject)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if remoteClusterName == domainName {
				return nil, trace.BadParameter("remote cluster name can not be the same as local cluster name")
			}
			// TODO(klizhentas) in 2.5.0 prohibit adding trusted cluster resource name
			// different from cluster name (we had no way of checking this before x509,
			// because SSH CA was a public key, not a cert with metadata)
		}
	}

	return validateResponse.CAs, nil
}

func (a *Server) addCertAuthorities(ctx context.Context, trustedCluster types.TrustedCluster, remoteCAs []types.CertAuthority) error {
	// the remote auth server has verified our token. add the
	// remote certificate authority to our backend
	for _, remoteCertAuthority := range remoteCAs {
		// change the name of the remote ca to the name of the trusted cluster
		remoteCertAuthority.SetName(trustedCluster.GetName())

		// wipe out roles sent from the remote cluster and set roles from the trusted cluster
		remoteCertAuthority.SetRoles(nil)
		if remoteCertAuthority.GetType() == types.UserCA {
			for _, r := range trustedCluster.GetRoles() {
				remoteCertAuthority.AddRole(r)
			}
			remoteCertAuthority.SetRoleMap(trustedCluster.GetRoleMap())
		}
	}

	// we use create here instead of upsert to prevent people from wiping out
	// their own ca if it has the same name as the remote ca
	_, err := a.CreateCertAuthorities(ctx, remoteCAs...)
	return trace.Wrap(err)
}

// DeleteRemoteCluster deletes remote cluster resource, all certificate authorities
// associated with it
func (a *Server) DeleteRemoteCluster(ctx context.Context, clusterName string) error {
	// To make sure remote cluster exists - to protect against random
	// clusterName requests (e.g. when clusterName is set to local cluster name)
	if _, err := a.GetRemoteCluster(clusterName); err != nil {
		return trace.Wrap(err)
	}

	// we only expect host CAs to be present for remote clusters, but it doesn't hurt
	// to err on the side of paranoia and delete all CA types.
	var ids []types.CertAuthID
	for _, caType := range types.CertAuthTypes {
		ids = append(ids, types.CertAuthID{
			Type:       caType,
			DomainName: clusterName,
		})
	}

	// delete cert authorities associated with the cluster
	if err := a.DeleteCertAuthorities(ctx, ids...); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.Services.DeleteRemoteCluster(ctx, clusterName))
}

// GetRemoteCluster returns remote cluster by name
func (a *Server) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	// To make sure remote cluster exists - to protect against random
	// clusterName requests (e.g. when clusterName is set to local cluster name)
	remoteCluster, err := a.Services.GetRemoteCluster(clusterName)
	return remoteCluster, trace.Wrap(err)
}

// updateRemoteClusterStatus determines current connection status of remoteCluster and writes it to the backend
// if there are changes. Returns true if backend was updated or false if update wasn't necessary.
func (a *Server) updateRemoteClusterStatus(ctx context.Context, netConfig types.ClusterNetworkingConfig, remoteCluster types.RemoteCluster) (updated bool, err error) {
	keepAliveCountMax := netConfig.GetKeepAliveCountMax()
	keepAliveInterval := netConfig.GetKeepAliveInterval()

	// fetch tunnel connections for the cluster to update runtime status
	connections, err := a.GetTunnelConnections(remoteCluster.GetName())
	if err != nil {
		return false, trace.Wrap(err)
	}
	lastConn, err := services.LatestTunnelConnection(connections)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		// No tunnel connections are known, mark the cluster offline (if it
		// wasn't already).
		if remoteCluster.GetConnectionStatus() != teleport.RemoteClusterStatusOffline {
			remoteCluster.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
			if err := a.UpdateRemoteCluster(ctx, remoteCluster); err != nil {
				// if the cluster was concurrently updated, ignore the update.  either
				// the update was consistent with our view of the world, in which case
				// retrying would be pointless, or the update was not consistent, in which
				// case we should prioritize presenting our view in an internally-consistent
				// manner rather than competing with another task.
				if !trace.IsCompareFailed(err) {
					return false, trace.Wrap(err)
				}
			}
			return true, nil
		}
		return false, nil
	}

	offlineThreshold := time.Duration(keepAliveCountMax) * keepAliveInterval
	tunnelStatus := services.TunnelConnectionStatus(a.clock, lastConn, offlineThreshold)

	// Update remoteCluster based on lastConn. If anything changed, update it
	// in the backend too.
	prevConnectionStatus := remoteCluster.GetConnectionStatus()
	prevLastHeartbeat := remoteCluster.GetLastHeartbeat()
	remoteCluster.SetConnectionStatus(tunnelStatus)
	// Only bump LastHeartbeat if it's newer.
	if lastConn.GetLastHeartbeat().After(prevLastHeartbeat) {
		remoteCluster.SetLastHeartbeat(lastConn.GetLastHeartbeat().UTC())
	}
	if prevConnectionStatus != remoteCluster.GetConnectionStatus() || !prevLastHeartbeat.Equal(remoteCluster.GetLastHeartbeat()) {
		if err := a.UpdateRemoteCluster(ctx, remoteCluster); err != nil {
			// if the cluster was concurrently updated, ignore the update.  either
			// the update was consistent with our view of the world, in which case
			// retrying would be pointless, or the update was not consistent, in which
			// case we should prioritize presenting our view in an internally-consistent
			// manner rather than competing with another task.
			if !trace.IsCompareFailed(err) {
				return false, trace.Wrap(err)
			}
		}
		return true, nil
	}

	return false, nil
}

// GetRemoteClusters returns remote clusters with updated statuses
func (a *Server) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	// To make sure remote cluster exists - to protect against random
	// clusterName requests (e.g. when clusterName is set to local cluster name)
	remoteClusters, err := a.Services.GetRemoteClusters(opts...)
	return remoteClusters, trace.Wrap(err)
}

func (a *Server) validateTrustedCluster(ctx context.Context, validateRequest *authclient.ValidateTrustedClusterRequest) (resp *authclient.ValidateTrustedClusterResponse, err error) {
	defer func() {
		if err != nil {
			log.WithError(err).Info("Trusted cluster validation failed")
		}
	}()

	log.Debugf("Received validate request: token=%s, CAs=%v", backend.MaskKeyName(validateRequest.Token), validateRequest.CAs)

	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// validate that we generated the token
	tokenLabels, err := a.validateTrustedClusterToken(ctx, validateRequest.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(validateRequest.CAs) != 1 {
		return nil, trace.AccessDenied("expected exactly one certificate authority, received %v", len(validateRequest.CAs))
	}
	remoteCA := validateRequest.CAs[0]

	if err := services.CheckAndSetDefaults(remoteCA); err != nil {
		return nil, trace.Wrap(err)
	}

	if remoteCA.GetType() != types.HostCA {
		return nil, trace.AccessDenied("expected host certificate authority, received CA with type %q", remoteCA.GetType())
	}

	// a host CA shouldn't have a rolemap or roles in the first place
	remoteCA.SetRoleMap(nil)
	remoteCA.SetRoles(nil)

	remoteClusterName := remoteCA.GetName()
	if remoteClusterName == domainName {
		return nil, trace.AccessDenied("remote cluster has same name as this cluster: %v", domainName)
	}
	_, err = a.GetTrustedCluster(ctx, remoteClusterName)
	if err == nil {
		return nil, trace.AccessDenied("remote cluster has same name as trusted cluster: %v", remoteClusterName)
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	remoteCluster, err := types.NewRemoteCluster(remoteClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(tokenLabels) != 0 {
		meta := remoteCluster.GetMetadata()
		meta.Labels = utils.CopyStringsMap(tokenLabels)
		remoteCluster.SetMetadata(meta)
	}
	remoteCluster.SetConnectionStatus(teleport.RemoteClusterStatusOffline)

	err = a.CreateRemoteCluster(remoteCluster)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
	}

	err = a.UpsertCertAuthority(ctx, remoteCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// export local cluster certificate authority and return it to the cluster
	validateResponse := authclient.ValidateTrustedClusterResponse{}

	validateResponse.CAs, err = getLeafClusterCAs(ctx, a, domainName, validateRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// log the local certificate authorities we are sending
	log.Debugf("Sending validate response: CAs=%v", validateResponse.CAs)

	return &validateResponse, nil
}

// getLeafClusterCAs returns a slice with Cert Authorities that should be returned in response to ValidateTrustedClusterRequest.
func getLeafClusterCAs(ctx context.Context, srv *Server, domainName string, validateRequest *authclient.ValidateTrustedClusterRequest) ([]types.CertAuthority, error) {
	certTypes, err := getCATypesForLeaf(validateRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts := make([]types.CertAuthority, 0, len(certTypes))

	for _, caType := range certTypes {
		certAuthority, err := srv.GetCertAuthority(
			ctx,
			types.CertAuthID{Type: caType, DomainName: domainName},
			false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caCerts = append(caCerts, certAuthority)
	}

	return caCerts, nil
}

// getCATypesForLeaf returns the list of CA certificates that should be sync in response to ValidateTrustedClusterRequest.
func getCATypesForLeaf(validateRequest *authclient.ValidateTrustedClusterRequest) ([]types.CertAuthType, error) {
	var (
		err                error
		openSSHCASupported bool
	)

	if validateRequest.TeleportVersion != "" {
		// (*ValidateTrustedClusterRequest).TeleportVersion was added in Teleport 10.0. If the request comes from an older
		// cluster this field will be empty.
		openSSHCASupported, err = utils.MinVerWithoutPreRelease(validateRequest.TeleportVersion, constants.OpenSSHCAMinVersion)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse Teleport version: %q", validateRequest.TeleportVersion)
		}
	}

	certTypes := []types.CertAuthType{types.HostCA, types.UserCA, types.DatabaseCA}
	if openSSHCASupported {
		// OpenSSH CA was introduced in Teleport 12.0. Do not send it to older clusters
		// as they don't understand it.
		certTypes = append(certTypes, types.OpenSSHCA)
	}

	return certTypes, nil
}

func (a *Server) validateTrustedClusterToken(ctx context.Context, tokenName string) (map[string]string, error) {
	provisionToken, err := a.ValidateToken(ctx, tokenName)
	if err != nil {
		return nil, trace.AccessDenied("the remote server denied access: invalid cluster token")
	}

	if !provisionToken.GetRoles().Include(types.RoleTrustedCluster) {
		return nil, trace.AccessDenied("role does not match")
	}

	return provisionToken.GetMetadata().Labels, nil
}

func (a *Server) sendValidateRequestToProxy(host string, validateRequest *authclient.ValidateTrustedClusterRequest) (*authclient.ValidateTrustedClusterResponse, error) {
	proxyAddr := url.URL{
		Scheme: "https",
		Host:   host,
	}

	opts := []roundtrip.ClientParam{
		roundtrip.SanitizerEnabled(true),
	}

	if lib.IsInsecureDevMode() {
		log.Warn("The setting insecureSkipVerify is used to communicate with proxy. Make sure you intend to run Teleport in insecure mode!")

		// Get the default transport, this allows picking up proxy from the
		// environment.
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, trace.BadParameter("invalid transport type %T", http.DefaultTransport)
		}
		// Clone the transport to not modify the global instance.
		tr := defaultTransport.Clone()
		// Disable certificate checking while in debug mode.
		tlsConfig := utils.TLSConfig(a.cipherSuites)
		tlsConfig.InsecureSkipVerify = true
		tr.TLSClientConfig = tlsConfig

		insecureWebClient := &http.Client{
			Transport: tracehttp.NewTransport(tr),
		}
		opts = append(opts, roundtrip.HTTPClient(insecureWebClient))
	}

	clt, err := roundtrip.NewClient(proxyAddr.String(), teleport.WebAPIVersion, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateRequestRaw, err := validateRequest.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := httplib.ConvertResponse(clt.PostJSON(context.TODO(), clt.Endpoint("webapi", "trustedclusters", "validate"), validateRequestRaw))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var validateResponseRaw authclient.ValidateTrustedClusterResponseRaw
	err = json.Unmarshal(out.Bytes(), &validateResponseRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := validateResponseRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponse, nil
}

// activateCertAuthority will activate both the user and host certificate
// authority given in the services.TrustedCluster resource.
func (a *Server) activateCertAuthority(ctx context.Context, t types.TrustedCluster) error {
	return trace.Wrap(a.ActivateCertAuthorities(ctx, []types.CertAuthID{
		{
			Type:       types.UserCA,
			DomainName: t.GetName(),
		},
		{
			Type:       types.HostCA,
			DomainName: t.GetName(),
		},
	}...))
}

// deactivateCertAuthority will deactivate both the user and host certificate
// authority given in the services.TrustedCluster resource.
func (a *Server) deactivateCertAuthority(ctx context.Context, t types.TrustedCluster) error {
	return trace.Wrap(a.DeactivateCertAuthorities(ctx, []types.CertAuthID{
		{
			Type:       types.UserCA,
			DomainName: t.GetName(),
		},
		{
			Type:       types.HostCA,
			DomainName: t.GetName(),
		},
	}...))
}

// createReverseTunnel will create a services.ReverseTunnel givenin the
// services.TrustedCluster resource.
func (a *Server) createReverseTunnel(t types.TrustedCluster) error {
	reverseTunnel, err := types.NewReverseTunnel(
		t.GetName(),
		[]string{t.GetReverseTunnelAddress()},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(a.UpsertReverseTunnel(reverseTunnel))
}
