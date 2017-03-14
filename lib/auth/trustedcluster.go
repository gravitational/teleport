package auth

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
)

func (a *AuthServer) getTrustedCluster(name string) (services.TrustedCluster, error) {
	return a.GetTrustedCluster(name)
}

func (a *AuthServer) getTrustedClusters() ([]services.TrustedCluster, error) {
	return a.GetTrustedClusters()
}

func (a *AuthServer) upsertTrustedCluster(trustedCluster services.TrustedCluster) error {
	if trustedCluster.GetEnabled() {
		err := a.enableTrustedCluster(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err := a.disableTrustedCluster(trustedCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err := a.UpsertTrustedCluster(trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *AuthServer) enableTrustedCluster(trustedCluster services.TrustedCluster) error {
	// get the certificate authorities for this auth server
	localCertAuthorities, err := a.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	// create a reques to validate a trusted cluster (token and local certificate authorities)
	validateRequest := ValidateTrustedClusterRequest{
		Token: trustedCluster.GetToken(),
		CAs:   localCertAuthorities,
	}

	// send the request to the remote auth server via the proxy
	validateResponse, err := a.sendValidateRequestToProxy(trustedCluster.GetProxyAddress(), &validateRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	// the remote auth server has verified our token. add the
	// remote certificate authority to our backend
	for _, remoteCertAuthority := range validateResponse.CAs {

		// add roles into user certificates
		if remoteCertAuthority.GetType() == services.UserCA {
			for _, r := range trustedCluster.GetRoles() {
				remoteCertAuthority.AddRole(r)
			}
		}

		err = a.UpsertCertAuthority(remoteCertAuthority, backend.Forever)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// the remote auth server has verified our token. add the
	// reverse tunnel into our backend
	reverseTunnel := services.NewReverseTunnel(
		trustedCluster.GetName(),
		[]string{trustedCluster.GetReverseTunnelAddress()},
	)
	err = a.UpsertReverseTunnel(reverseTunnel, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *AuthServer) disableTrustedCluster(trustedCluster services.TrustedCluster) error {
	err := a.DeleteCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: trustedCluster.GetName()})
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	err = a.DeleteCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: trustedCluster.GetName()})
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	err = a.DeleteReverseTunnel(trustedCluster.GetName())
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *AuthServer) validateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	// validate that we generated the token
	err := a.validateTrustedClusterToken(validateRequest.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// token has been validated, upsert the given certificate authority
	for _, certAuthority := range validateRequest.CAs {
		err = a.UpsertCertAuthority(certAuthority, backend.Forever)
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
			validateResponse.CAs = append(validateResponse.CAs, certAuthority)
		}
	}

	return &validateResponse, nil
}

func (a *AuthServer) validateTrustedClusterToken(token string) error {
	roles, err := a.ValidateToken(token)
	if err != nil {
		return trace.AccessDenied("invalid token")
	}

	if !roles.Include(teleport.RoleTrustedCluster) {
		return trace.AccessDenied("role does not match")
	}

	if !a.checkTokenTTL(token) {
		return trace.AccessDenied("expired token")
	}

	return nil
}

func (a *AuthServer) deleteTrustedCluster(name string) error {
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

	err = a.DeleteTrustedCluster(name)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *AuthServer) sendValidateRequestToProxy(host string, validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	proxyAddr := url.URL{
		Scheme: "https",
		Host:   host,
	}

	var opts []roundtrip.ClientParam
	if s.DeveloperMode {
		log.Warn("InsecureSkipVerify used to communicate with proxy.")
		log.Warn("Make sure you intend to run Teleport in debug mode.")

		insecureWebClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
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
