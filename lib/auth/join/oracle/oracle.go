// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oracle

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const teleportUserAgent = "teleport/" + api.Version

const (
	// DateHeader is the header containing the date to send to Oracle.
	DateHeader = "x-date"
	// ChallengeHeader is the header containing the Teleport-generated challenge
	// string to send to Oracle.
	ChallengeHeader = "x-teleport-challenge"
)

const (
	tenancyClaim     = "opc-tenant"
	compartmentClaim = "opc-compartment"
	instanceClaim    = "opc-instance"
)

type authenticateClientDetails struct {
	RequestHeaders http.Header `json:"requestHeaders"`
}

type authenticateClientRequest struct {
	Date      string                    `contributesTo:"header" name:"x-date"`
	Challenge string                    `contributesTo:"header" name:"x-teleport-challenge"`
	UserAgent string                    `contributesTo:"header" name:"User-Agent"`
	Details   authenticateClientDetails `contributesTo:"body"`
}

// Claims are the claims returned by the authenticateClient endpoint.
type Claims struct {
	// TenancyID is the ID of the instance's tenant.
	TenancyID string `json:"tenant_id"`
	// CompartmentID is the ID of the instance's compartment.
	CompartmentID string `json:"compartment_id"`
	// InstanceID is the instance's ID.
	InstanceID string `json:"-"`
}

// Region extracts the region from an instance's claims.
func (c Claims) Region() string {
	region, err := ParseRegionFromOCID(c.InstanceID)
	if err != nil {
		return ""
	}
	return region
}

type claim struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type principal struct {
	Claims []claim `json:"claims"`
}

func (p principal) getClaims() Claims {
	var claims Claims
	for _, claim := range p.Claims {
		switch claim.Key {
		case tenancyClaim:
			claims.TenancyID = claim.Value
		case compartmentClaim:
			claims.CompartmentID = claim.Value
		case instanceClaim:
			claims.InstanceID = claim.Value
		}
	}
	return claims
}

type authenticateClientResult struct {
	ErrorMessage string    `json:"errorMessage,omitempty"`
	Principal    principal `json:"principal,omitempty"`
}

type authenticateClientResponse struct {
	Result authenticateClientResult `presentIn:"body"`
}

func newAuthenticateClientRequest(time time.Time, challenge string, headers http.Header) authenticateClientRequest {
	req := authenticateClientRequest{
		Date:      time.UTC().Format(http.TimeFormat),
		Challenge: challenge,
		UserAgent: teleportUserAgent,
		Details: authenticateClientDetails{
			RequestHeaders: headers,
		},
	}
	// Avoid a null request body.
	if len(headers) == 0 {
		req.Details.RequestHeaders = http.Header{}
	}
	return req
}

func createAuthHTTPRequest(region string, auth authenticateClientRequest) (*http.Request, error) {
	req, err := common.MakeDefaultHTTPRequestWithTaggedStruct(
		http.MethodPost,
		"",
		auth,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpointURL, err := url.Parse(fmt.Sprintf("https://auth.%s.oraclecloud.com/v1/authentication/authenticateClient", region))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Manually set the host header so it will be sent as part of the grpc.
	req.Header.Set("Host", endpointURL.Host)
	req.Host = endpointURL.Host
	req.URL = endpointURL

	// If no headers were provided, this is the inner header payload and we need
	// to explicitly include (request-target).
	if len(auth.Details.RequestHeaders) == 0 {
		req.Header.Set("(request-target)", strings.ToLower(http.MethodPost)+" "+endpointURL.RequestURI())
	}
	return &req, nil
}

type oracleLogger struct {
	logger *slog.Logger
}

// LogLevel returns the log level of sdkLogger (always debug).
func (l *oracleLogger) LogLevel() int {
	// Oracle SDK encoding of debug level.
	return 2
}

// Log logs v with the provided format if the current log level is loglevel
func (l *oracleLogger) Log(logLevel int, format string, v ...interface{}) error {
	if !l.logger.Handler().Enabled(context.Background(), slog.LevelDebug) {
		return nil
	}

	//nolint:sloglint // msg cannot be constant
	l.logger.DebugContext(context.Background(), fmt.Sprintf(format, v...))

	return nil
}

func init() {
	common.SetSDKLogger(&oracleLogger{
		logger: logutils.NewPackageLogger().With(teleport.ComponentKey, "oracle"),
	})
}

// CreateSignedRequest creates a signed HTTP request to
// https://auth.<region>.oraclecloud.com/v1/authentication/authenticateClient
// with an instance principal config provider. The returned headers should be
// sent to an auth server as part of RegisterUsingOracleMethod.
func CreateSignedRequest(challenge string) (innerHeaders, outerHeaders http.Header, err error) {
	provider, err := auth.InstancePrincipalConfigurationProviderWithCustomClient(
		func(dispatcher common.HTTPRequestDispatcher) (common.HTTPRequestDispatcher, error) {
			client, err := defaults.HTTPClient()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			client.Timeout = 10 * time.Second
			return client, nil
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	inner, outer, err := CreateSignedRequestWithProvider(provider, challenge)
	return inner, outer, trace.Wrap(err)
}

// CreateSignedRequestWithProvider creates a signed HTTP request to
// https://auth.<region>.oraclecloud.com/v1/authentication/authenticateClient
// using the given config provider. The returned headers should be sent to an
// auth server as part of RegisterUsingOracleMethod.
func CreateSignedRequestWithProvider(provider common.ConfigurationProvider, challenge string) (innerHeaders, outerHeaders http.Header, err error) {
	signedHeaders := append(common.DefaultGenericHeaders(), DateHeader, ChallengeHeader)
	signer := common.RequestSigner(provider, signedHeaders, common.DefaultBodyHeaders())
	region, err := provider.Region()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	now := time.Now().UTC()
	innerReq, err := createAuthHTTPRequest(region, newAuthenticateClientRequest(now, challenge, nil))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(innerReq)

	outerReq, err := createAuthHTTPRequest(region, newAuthenticateClientRequest(now, challenge, innerReq.Header))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(outerReq)
	return innerReq.Header, outerReq.Header, nil
}

// GetAuthorizationHeaderValues gets the key-value pairs encoded in the
// Authorization header as described in the [Oracle API docs].
//
// [Oracle API docs]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/signingrequests.htm#five
func GetAuthorizationHeaderValues(header http.Header) map[string]string {
	rawValues, isSignature := strings.CutPrefix(header.Get("Authorization"), "Signature ")
	if !isSignature {
		return nil
	}
	keyValuePairs := strings.Split(rawValues, ",")
	values := make(map[string]string, len(keyValuePairs))
	for _, pair := range keyValuePairs {
		k, v, isPair := strings.Cut(pair, "=")
		if !isPair {
			continue
		}
		values[k] = strings.Trim(v, "\"")
	}
	return values
}

// CreateRequestFromHeaders recreates an HTTP request to the authenticateClient
// endpoint from its inner and outer headers.
func CreateRequestFromHeaders(region string, innerHeaders, outerHeaders http.Header) (*http.Request, error) {
	req, err := createAuthHTTPRequest(region, authenticateClientRequest{
		Date:      outerHeaders.Get(DateHeader),
		Challenge: outerHeaders.Get(ChallengeHeader),
		UserAgent: teleportUserAgent,
		Details: authenticateClientDetails{
			RequestHeaders: innerHeaders,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header = outerHeaders
	return req, nil
}

// FetchOraclePrincipalClaims executes a request to authenticateClient and parses
// the response.
func FetchOraclePrincipalClaims(ctx context.Context, req *http.Request) (Claims, error) {
	client, err := defaults.HTTPClient()
	if err != nil {
		return Claims{}, trace.Wrap(err)
	}
	// Block redirects.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	authResp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return Claims{}, trace.Wrap(err)
	}
	defer authResp.Body.Close()
	var resp authenticateClientResponse
	unmarshalErr := common.UnmarshalResponse(authResp, &resp)
	if authResp.StatusCode >= 300 || resp.Result.ErrorMessage != "" {
		msg := resp.Result.ErrorMessage
		if msg == "" {
			msg = authResp.Status
		}
		return Claims{}, trace.AccessDenied("%v", msg)
	}
	if unmarshalErr != nil {
		return Claims{}, trace.Wrap(unmarshalErr)
	}
	return resp.Result.Principal.getClaims(), nil
}

// Hack: StringToRegion will lazily load regions from a config file if its
// input isn't in its hard-coded list, in a non-threadsafe way. Call it here
// to load the config ahead of time so future calls are threadsafe.
var _ = common.StringToRegion("")

// ParseRegion parses a string into a full (not abbreviated) Oracle Cloud
// region. It returns the empty string if the input is not a valid region.
func ParseRegion(rawRegion string) (region, realm string) {
	canonicalRegion := common.StringToRegion(rawRegion)
	realm, err := canonicalRegion.RealmID()
	if err != nil {
		return "", ""
	}
	return string(canonicalRegion), realm
}

var ociRealms = map[string]struct{}{
	"oc1": {}, "oc2": {}, "oc3": {}, "oc4": {}, "oc8": {}, "oc9": {},
	"oc10": {}, "oc14": {}, "oc15": {}, "oc19": {}, "oc20": {}, "oc21": {},
	"oc23": {}, "oc24": {}, "oc26": {}, "oc29": {}, "oc35": {},
}

// ParseRegionFromOCID parses an Oracle OCID and returns the embedded region.
// It returns an error if the input is not a valid OCID.
func ParseRegionFromOCID(ocid string) (string, error) {
	// OCID format: ocid1.<RESOURCE TYPE>.<REALM>.[REGION][.FUTURE USE].<UNIQUE ID>
	// Check format.
	ocidParts := strings.Split(ocid, ".")
	switch len(ocidParts) {
	case 5, 6:
	default:
		return "", trace.BadParameter("not an ocid")
	}
	// Check version.
	if ocidParts[0] != "ocid1" {
		return "", trace.BadParameter("invalid ocid version: %v", ocidParts[0])
	}
	// Check realm.
	if _, ok := ociRealms[ocidParts[2]]; !ok {
		return "", trace.BadParameter("invalid realm: %v", ocidParts[2])
	}
	resourceType := ocidParts[1]
	region, realm := ParseRegion(ocidParts[3])
	// Check type. Only instance OCIDs should have a region.
	switch resourceType {
	case "instance":
		if region == "" {
			return "", trace.BadParameter("invalid region: %v", region)
		}
		if realm != ocidParts[2] {
			return "", trace.BadParameter("invalid realm %q for region %q", ocidParts[2], region)
		}
	case "compartment", "tenancy":
		if ocidParts[3] != "" {
			return "", trace.BadParameter("resource type %v should not have a region", resourceType)
		}
	default:
		return "", trace.BadParameter("unsupported resource type: %v", resourceType)
	}
	return region, nil
}
