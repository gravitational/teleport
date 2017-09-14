/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package auth

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// UpsertTrustedCluster creates or toggles a Trusted Cluster relationship.
func (a *AuthServer) UpsertTrustedCluster(trustedCluster services.TrustedCluster) error {
	var exists bool
	existingCluster, err := a.Presence.GetTrustedCluster(trustedCluster.GetName())
	if err == nil {
		exists = true
	}
	enable := trustedCluster.GetEnabled()

	// if the trusted cluster already exists in the backend, make sure it's a
	// valid state change we are trying to make
	if exists == true {
		err := existingCluster.CanChangeStateTo(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// change state
	switch {
	case exists == true && enable == true:
		log.Debugf("[TRUSTED CLUSTER] Enabling existing Trusted Cluster relationship.")

		err := a.activateCertAuthority(trustedCluster)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("enable only supported for Trusted Clusters created with Teleport 2.3 and above")
			}
			return trace.Wrap(err)
		}

		err = a.createReverseTunnel(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	case exists == true && enable == false:
		log.Debugf("[TRUSTED CLUSTER] Disabling existing Trusted Cluster relationship.")

		err := a.deactivateCertAuthority(trustedCluster)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.BadParameter("enable only supported for Trusted Clusters created with Teleport 2.3 and above")
			}
			return trace.Wrap(err)
		}

		err = a.DeleteReverseTunnel(trustedCluster.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	case exists == false && enable == true:
		log.Debugf("[TRUSTED CLUSTER] Creating enabled Trusted Cluster relationship.")

		remoteCAs, err := a.establishTrust(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}

		err = a.addCertAuthorities(trustedCluster, remoteCAs)
		if err != nil {
			return trace.Wrap(err)
		}

		err = a.createReverseTunnel(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	case exists == false && enable == false:
		log.Debugf("[TRUSTED CLUSTER] Creating disabled Trusted Cluster relationship.")

		remoteCAs, err := a.establishTrust(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}

		err = a.addCertAuthorities(trustedCluster, remoteCAs)
		if err != nil {
			return trace.Wrap(err)
		}

		err = a.deactivateCertAuthority(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = a.Presence.UpsertTrustedCluster(trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil

}

// DeleteTrustedCluster removes services.CertAuthority, services.ReverseTunnel,
// and services.TrustedCluster resources.
func (a *AuthServer) DeleteTrustedCluster(name string) error {
	err := a.DeleteCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: name})
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	err = a.DeleteCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: name})
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	err = a.DeleteReverseTunnel(name)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	err = a.Presence.DeleteTrustedCluster(name)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *AuthServer) establishTrust(trustedCluster services.TrustedCluster) ([]services.CertAuthority, error) {
	var localCertAuthorities []services.CertAuthority

	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get a list of certificate authorities for this auth server
	allLocalCAs, err := a.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, lca := range allLocalCAs {
		if lca.GetClusterName() == domainName {
			localCertAuthorities = append(localCertAuthorities, lca)
		}
	}

	// create a request to validate a trusted cluster (token and local certificate authorities)
	validateRequest := ValidateTrustedClusterRequest{
		Token: trustedCluster.GetToken(),
		CAs:   localCertAuthorities,
	}

	// log the local certificate authorities that we are sending
	log.Debugf("[TRUSTED CLUSTER] Sending validate request; token=%v, CAs=%v", validateRequest.Token, validateRequest.CAs)

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
	log.Debugf("[TRUSTED CLUSTER] Received validate response; CAs=%v", validateResponse.CAs)

	return validateResponse.CAs, nil
}

func (a *AuthServer) addCertAuthorities(trustedCluster services.TrustedCluster, remoteCAs []services.CertAuthority) error {
	// the remote auth server has verified our token. add the
	// remote certificate authority to our backend
	for _, remoteCertAuthority := range remoteCAs {
		// change the name of the remote ca to the name of the trusted cluster
		remoteCertAuthority.SetName(trustedCluster.GetName())

		// wipe out roles sent from the remote cluster and set roles from the trusted cluster
		remoteCertAuthority.SetRoles(nil)
		if remoteCertAuthority.GetType() == services.UserCA {
			for _, r := range trustedCluster.GetRoles() {
				remoteCertAuthority.AddRole(r)
			}
			remoteCertAuthority.SetRoleMap(trustedCluster.GetRoleMap())
		}

		// we use create here instead of upsert to prevent people from wiping out
		// their own ca if it has the same name as the remote ca
		err := a.CreateCertAuthority(remoteCertAuthority)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *AuthServer) validateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// validate that we generated the token
	err = a.validateTrustedClusterToken(validateRequest.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// log the remote certificate authorities we are adding
	log.Debugf("[TRUSTED CLUSTER] Received validate request: token=%v, CAs=%v", validateRequest.Token, validateRequest.CAs)

	// token has been validated, upsert the given certificate authority
	for _, certAuthority := range validateRequest.CAs {
		// don't add a ca with the same as as your own
		if certAuthority.GetName() == domainName {
			return nil, trace.AccessDenied("remote certificate authority has same name as cluster certificate authority: %v", domainName)
		}
		err = a.UpsertCertAuthority(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// export our certificate authority and return it to the cluster
	validateResponse := ValidateTrustedClusterResponse{
		CAs: []services.CertAuthority{},
	}
	for _, caType := range []services.CertAuthType{services.HostCA, services.UserCA} {
		certAuthorities, err := a.GetCertAuthorities(caType, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, certAuthority := range certAuthorities {
			if certAuthority.GetClusterName() == domainName {
				validateResponse.CAs = append(validateResponse.CAs, certAuthority)
			}
		}
	}

	// log the local certificate authorities we are sending
	log.Debugf("[TRUSTED CLUSTER] Sending validate response: CAs=%v", validateResponse.CAs)

	return &validateResponse, nil
}

func (a *AuthServer) validateTrustedClusterToken(token string) error {
	roles, err := a.ValidateToken(token)
	if err != nil {
		return trace.AccessDenied("the remote server denied access: invalid cluster token")
	}

	if !roles.Include(teleport.RoleTrustedCluster) && !roles.Include(teleport.LegacyClusterTokenType) {
		return trace.AccessDenied("role does not match")
	}

	if !a.checkTokenTTL(token) {
		return trace.AccessDenied("expired token")
	}

	return nil
}

func (s *AuthServer) sendValidateRequestToProxy(host string, validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	proxyAddr := url.URL{
		Scheme: "https",
		Host:   host,
	}

	var opts []roundtrip.ClientParam

	if lib.IsInsecureDevMode() {
		log.Warn("insecureSkipVerify is used to communicate with proxy. make sure you intend to run Teleport in insecure mode!")

		// get the default transport (so we can get the proxy from environment)
		// but disable tls certificate checking.
		tr, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, trace.BadParameter("unable to get default transport")
		}
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		insecureWebClient := &http.Client{
			Transport: tr,
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

	out, err := httplib.ConvertResponse(clt.PostJSON(clt.Endpoint("webapi", "trustedclusters", "validate"), validateRequestRaw))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var validateResponseRaw *ValidateTrustedClusterResponseRaw
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

type ValidateTrustedClusterRequest struct {
	Token string                   `json:"token"`
	CAs   []services.CertAuthority `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterRequest) ToRaw() (*ValidateTrustedClusterRequestRaw, error) {
	cas := [][]byte{}

	for _, certAuthority := range v.CAs {
		data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, data)
	}

	return &ValidateTrustedClusterRequestRaw{
		Token: v.Token,
		CAs:   cas,
	}, nil
}

type ValidateTrustedClusterRequestRaw struct {
	Token string   `json:"token"`
	CAs   [][]byte `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterRequestRaw) ToNative() (*ValidateTrustedClusterRequest, error) {
	cas := []services.CertAuthority{}

	for _, rawCertAuthority := range v.CAs {
		certAuthority, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(rawCertAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, certAuthority)
	}

	return &ValidateTrustedClusterRequest{
		Token: v.Token,
		CAs:   cas,
	}, nil
}

type ValidateTrustedClusterResponse struct {
	CAs []services.CertAuthority `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterResponse) ToRaw() (*ValidateTrustedClusterResponseRaw, error) {
	cas := [][]byte{}

	for _, certAuthority := range v.CAs {
		data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(certAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, data)
	}

	return &ValidateTrustedClusterResponseRaw{
		CAs: cas,
	}, nil
}

type ValidateTrustedClusterResponseRaw struct {
	CAs [][]byte `json:"certificate_authorities"`
}

func (v *ValidateTrustedClusterResponseRaw) ToNative() (*ValidateTrustedClusterResponse, error) {
	cas := []services.CertAuthority{}

	for _, rawCertAuthority := range v.CAs {
		certAuthority, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(rawCertAuthority)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cas = append(cas, certAuthority)
	}

	return &ValidateTrustedClusterResponse{
		CAs: cas,
	}, nil
}

// activateCertAuthority will activate both the user and host certificate
// authority given in the services.TrustedCluster resource.
func (a *AuthServer) activateCertAuthority(t services.TrustedCluster) error {
	err := a.ActivateCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: t.GetName()})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.ActivateCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: t.GetName()}))
}

// deactivateCertAuthority will deactivate both the user and host certificate
// authority given in the services.TrustedCluster resource.
func (a *AuthServer) deactivateCertAuthority(t services.TrustedCluster) error {
	err := a.DeactivateCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: t.GetName()})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.DeactivateCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: t.GetName()}))
}

// createReverseTunnel will create a services.ReverseTunnel givenin the
// services.TrustedCluster resource.
func (a *AuthServer) createReverseTunnel(t services.TrustedCluster) error {
	reverseTunnel := services.NewReverseTunnel(
		t.GetName(),
		[]string{t.GetReverseTunnelAddress()},
	)
	return trace.Wrap(a.UpsertReverseTunnel(reverseTunnel))
}
