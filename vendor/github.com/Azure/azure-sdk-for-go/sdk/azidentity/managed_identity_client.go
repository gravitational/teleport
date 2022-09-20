// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
)

const (
	imdsEndpoint = "http://169.254.169.254/metadata/identity/oauth2/token"
)

const (
	arcIMDSEndpoint          = "IMDS_ENDPOINT"
	identityEndpoint         = "IDENTITY_ENDPOINT"
	identityHeader           = "IDENTITY_HEADER"
	identityServerThumbprint = "IDENTITY_SERVER_THUMBPRINT"
	msiEndpoint              = "MSI_ENDPOINT"
	msiSecret                = "MSI_SECRET"
	imdsAPIVersion           = "2018-02-01"
	azureArcAPIVersion       = "2019-08-15"
	serviceFabricAPIVersion  = "2019-07-01-preview"
)

type msiType int

const (
	msiTypeUnknown             msiType = 0
	msiTypeIMDS                msiType = 1
	msiTypeAppServiceV20170901 msiType = 2
	msiTypeCloudShell          msiType = 3
	msiTypeUnavailable         msiType = 4
	msiTypeAppServiceV20190801 msiType = 5
	msiTypeAzureArc            msiType = 6
	msiTypeServiceFabric       msiType = 7
)

// managedIdentityClient provides the base for authenticating in managed identity environments
// This type includes an runtime.Pipeline and TokenCredentialOptions.
type managedIdentityClient struct {
	pipeline             runtime.Pipeline
	imdsAPIVersion       string
	imdsAvailableTimeout time.Duration
	msiType              msiType
	endpoint             string
	id                   ManagedIdentityIDKind
	unavailableMessage   string
}

type wrappedNumber json.Number

func (n *wrappedNumber) UnmarshalJSON(b []byte) error {
	c := string(b)
	if c == "\"\"" {
		return nil
	}
	return json.Unmarshal(b, (*json.Number)(n))
}

// newManagedIdentityClient creates a new instance of the ManagedIdentityClient with the ManagedIdentityCredentialOptions
// that are passed into it along with a default pipeline.
// options: ManagedIdentityCredentialOptions configure policies for the pipeline and the authority host that
// will be used to retrieve tokens and authenticate
func newManagedIdentityClient(options *ManagedIdentityCredentialOptions) *managedIdentityClient {
	logEnvVars()
	return &managedIdentityClient{
		id:                   options.ID,
		pipeline:             newDefaultMSIPipeline(*options), // a pipeline that includes the specific requirements for MSI authentication, such as custom retry policy options
		imdsAPIVersion:       imdsAPIVersion,                  // this field will be set to whatever value exists in the constant and is used when creating requests to IMDS
		imdsAvailableTimeout: 500 * time.Millisecond,          // we allow a timeout of 500 ms since the endpoint might be slow to respond
		msiType:              msiTypeUnknown,                  // when creating a new managedIdentityClient, the current MSI type is unknown and will be tested for and replaced once authenticate() is called from GetToken on the credential side
	}
}

// authenticate creates an authentication request for a Managed Identity and returns the resulting Access Token if successful.
// ctx: The current context for controlling the request lifetime.
// clientID: The client (application) ID of the service principal.
// scopes: The scopes required for the token.
func (c *managedIdentityClient) authenticate(ctx context.Context, clientID string, scopes []string) (*azcore.AccessToken, error) {
	if len(c.unavailableMessage) > 0 {
		return nil, &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: c.unavailableMessage}
	}

	msg, err := c.createAuthRequest(ctx, clientID, scopes)
	if err != nil {
		return nil, err
	}

	resp, err := c.pipeline.Do(msg)
	if err != nil {
		return nil, err
	}

	if runtime.HasStatusCode(resp, successStatusCodes[:]...) {
		return c.createAccessToken(resp)
	}

	if c.msiType == msiTypeIMDS && resp.StatusCode == 400 {
		if len(clientID) > 0 {
			return nil, &AuthenticationFailedError{msg: "The requested identity isn't assigned to this resource."}
		}
		c.unavailableMessage = "No default identity is assigned to this resource."
		return nil, &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: c.unavailableMessage}
	}

	return nil, &AuthenticationFailedError{inner: newAADAuthenticationFailedError(resp)}
}

func (c *managedIdentityClient) createAccessToken(res *http.Response) (*azcore.AccessToken, error) {
	value := struct {
		// these are the only fields that we use
		Token        string        `json:"access_token,omitempty"`
		RefreshToken string        `json:"refresh_token,omitempty"`
		ExpiresIn    wrappedNumber `json:"expires_in,omitempty"` // this field should always return the number of seconds for which a token is valid
		ExpiresOn    interface{}   `json:"expires_on,omitempty"` // the value returned in this field varies between a number and a date string
	}{}
	if err := runtime.UnmarshalAsJSON(res, &value); err != nil {
		return nil, fmt.Errorf("internal AccessToken: %w", err)
	}
	if value.ExpiresIn != "" {
		expiresIn, err := json.Number(value.ExpiresIn).Int64()
		if err != nil {
			return nil, err
		}
		return &azcore.AccessToken{Token: value.Token, ExpiresOn: time.Now().Add(time.Second * time.Duration(expiresIn)).UTC()}, nil
	}
	switch v := value.ExpiresOn.(type) {
	case float64:
		return &azcore.AccessToken{Token: value.Token, ExpiresOn: time.Unix(int64(v), 0).UTC()}, nil
	case string:
		if expiresOn, err := strconv.Atoi(v); err == nil {
			return &azcore.AccessToken{Token: value.Token, ExpiresOn: time.Unix(int64(expiresOn), 0).UTC()}, nil
		}
		// this is the case when expires_on is a time string
		// this is the format of the string coming from the service
		if expiresOn, err := time.Parse("1/2/2006 15:04:05 PM +00:00", v); err == nil { // the date string specified is for Windows OS
			eo := expiresOn.UTC()
			return &azcore.AccessToken{Token: value.Token, ExpiresOn: eo}, nil
		} else if expiresOn, err := time.Parse("1/2/2006 15:04:05 +00:00", v); err == nil { // the date string specified is for Linux OS
			eo := expiresOn.UTC()
			return &azcore.AccessToken{Token: value.Token, ExpiresOn: eo}, nil
		} else {
			return nil, err
		}
	default:
		return nil, &AuthenticationFailedError{msg: fmt.Sprintf("unsupported type received in expires_on: %T, %v", v, v)}
	}
}

func (c *managedIdentityClient) createAuthRequest(ctx context.Context, clientID string, scopes []string) (*policy.Request, error) {
	switch c.msiType {
	case msiTypeIMDS:
		return c.createIMDSAuthRequest(ctx, clientID, scopes)
	case msiTypeAppServiceV20170901, msiTypeAppServiceV20190801:
		return c.createAppServiceAuthRequest(ctx, clientID, scopes)
	case msiTypeAzureArc:
		// need to perform preliminary request to retreive the secret key challenge provided by the HIMDS service
		key, err := c.getAzureArcSecretKey(ctx, scopes)
		if err != nil {
			return nil, &AuthenticationFailedError{inner: err, msg: "Failed to retreive secret key from the identity endpoint."}
		}
		return c.createAzureArcAuthRequest(ctx, key, scopes)
	case msiTypeServiceFabric:
		return c.createServiceFabricAuthRequest(ctx, clientID, scopes)
	case msiTypeCloudShell:
		return c.createCloudShellAuthRequest(ctx, clientID, scopes)
	default:
		errorMsg := ""
		switch c.msiType {
		case msiTypeUnavailable:
			errorMsg = "unavailable"
		default:
			errorMsg = "unknown"
		}
		c.unavailableMessage = "Make sure you are running in a valid Managed Identity environment. Status: " + errorMsg
		return nil, &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: c.unavailableMessage}
	}
}

func (c *managedIdentityClient) createIMDSAuthRequest(ctx context.Context, id string, scopes []string) (*policy.Request, error) {
	request, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return nil, err
	}
	request.Raw().Header.Set(headerMetadata, "true")
	q := request.Raw().URL.Query()
	q.Add("api-version", c.imdsAPIVersion)
	q.Add("resource", strings.Join(scopes, " "))
	if c.id == ResourceID {
		q.Add(qpResID, id)
	} else if id != "" {
		q.Add(qpClientID, id)
	}
	request.Raw().URL.RawQuery = q.Encode()
	return request, nil
}

func (c *managedIdentityClient) createAppServiceAuthRequest(ctx context.Context, id string, scopes []string) (*policy.Request, error) {
	request, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return nil, err
	}
	q := request.Raw().URL.Query()
	if c.msiType == msiTypeAppServiceV20170901 {
		request.Raw().Header.Set("secret", os.Getenv(msiSecret))
		q.Add("api-version", "2017-09-01")
		q.Add("resource", strings.Join(scopes, " "))
		if c.id == ResourceID {
			q.Add(qpResID, id)
		} else if id != "" {
			// the legacy 2017 API version specifically specifies "clientid" and not "client_id" as a query param
			q.Add("clientid", id)
		}
	} else if c.msiType == msiTypeAppServiceV20190801 {
		request.Raw().Header.Set("X-IDENTITY-HEADER", os.Getenv(identityHeader))
		q.Add("api-version", "2019-08-01")
		q.Add("resource", scopes[0])
		if c.id == ResourceID {
			q.Add(qpResID, id)
		} else if id != "" {
			q.Add(qpClientID, id)
		}
	}

	request.Raw().URL.RawQuery = q.Encode()
	return request, nil
}

func (c *managedIdentityClient) createServiceFabricAuthRequest(ctx context.Context, id string, scopes []string) (*policy.Request, error) {
	request, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return nil, err
	}
	q := request.Raw().URL.Query()
	request.Raw().Header.Set("Accept", "application/json")
	request.Raw().Header.Set("Secret", os.Getenv(identityHeader))
	q.Add("api-version", serviceFabricAPIVersion)
	q.Add("resource", strings.Join(scopes, " "))
	if id != "" {
		q.Add(qpClientID, id)
	}
	request.Raw().URL.RawQuery = q.Encode()
	return request, nil
}

func (c *managedIdentityClient) getAzureArcSecretKey(ctx context.Context, resources []string) (string, error) {
	// create the request to retreive the secret key challenge provided by the HIMDS service
	request, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return "", err
	}
	request.Raw().Header.Set(headerMetadata, "true")
	q := request.Raw().URL.Query()
	q.Add("api-version", azureArcAPIVersion)
	q.Add("resource", strings.Join(resources, " "))
	request.Raw().URL.RawQuery = q.Encode()
	// send the initial request to get the short-lived secret key
	response, err := c.pipeline.Do(request)
	if err != nil {
		return "", err
	}
	// the endpoint is expected to return a 401 with the WWW-Authenticte header set to the location
	// of the secret key file. Any other status code indicates an error in the request.
	if response.StatusCode != 401 {
		return "", &AuthenticationFailedError{inner: newAADAuthenticationFailedError(response), msg: fmt.Sprintf("Expected a 401 Unauthorized response, received: %d", response.StatusCode)}
	}
	header := response.Header.Get("WWW-Authenticate")
	if len(header) == 0 {
		return "", errors.New("did not receive a value from WWW-Authenticate header")
	}
	// the WWW-Authenticate header is expected in the following format: Basic realm=/some/file/path.key
	pos := strings.LastIndex(header, "=")
	if pos == -1 {
		return "", fmt.Errorf("did not receive a correct value from WWW-Authenticate header: %s", header)
	}
	key, err := ioutil.ReadFile(header[pos+1:])
	if err != nil {
		return "", fmt.Errorf("could not read file (%s) contents: %w", header[pos+1:], err)
	}
	return string(key), nil
}

func (c *managedIdentityClient) createAzureArcAuthRequest(ctx context.Context, key string, resources []string) (*policy.Request, error) {
	request, err := runtime.NewRequest(ctx, http.MethodGet, c.endpoint)
	if err != nil {
		return nil, err
	}
	request.Raw().Header.Set(headerMetadata, "true")
	request.Raw().Header.Set(headerAuthorization, fmt.Sprintf("Basic %s", key))
	q := request.Raw().URL.Query()
	q.Add("api-version", azureArcAPIVersion)
	q.Add("resource", strings.Join(resources, " "))
	request.Raw().URL.RawQuery = q.Encode()
	return request, nil
}

func (c *managedIdentityClient) createCloudShellAuthRequest(ctx context.Context, clientID string, scopes []string) (*policy.Request, error) {
	request, err := runtime.NewRequest(ctx, http.MethodPost, c.endpoint)
	if err != nil {
		return nil, err
	}
	request.Raw().Header.Set(headerMetadata, "true")
	data := url.Values{}
	data.Set("resource", strings.Join(scopes, " "))
	if clientID != "" {
		data.Set(qpClientID, clientID)
	}
	dataEncoded := data.Encode()
	body := streaming.NopCloser(strings.NewReader(dataEncoded))
	if err := request.SetBody(body, headerURLEncoded); err != nil {
		return nil, err
	}
	return request, nil
}

func (c *managedIdentityClient) getMSIType() (msiType, error) {
	if c.msiType == msiTypeUnknown { // if we haven't already determined the msiType
		if endpointEnvVar := os.Getenv(msiEndpoint); endpointEnvVar != "" { // if the env var MSI_ENDPOINT is set
			c.endpoint = endpointEnvVar
			if secretEnvVar := os.Getenv(msiSecret); secretEnvVar != "" { // if BOTH the env vars MSI_ENDPOINT and MSI_SECRET are set the msiType is AppService
				c.msiType = msiTypeAppServiceV20170901
			} else { // if ONLY the env var MSI_ENDPOINT is set the msiType is CloudShell
				c.msiType = msiTypeCloudShell
			}
		} else if endpointEnvVar := os.Getenv(identityEndpoint); endpointEnvVar != "" { // check for IDENTITY_ENDPOINT
			c.endpoint = endpointEnvVar
			if header := os.Getenv(identityHeader); header != "" { // if BOTH the env vars IDENTITY_ENDPOINT and IDENTITY_HEADER are set the msiType is AppService
				c.msiType = msiTypeAppServiceV20190801
				if thumbprint := os.Getenv(identityServerThumbprint); thumbprint != "" { // if IDENTITY_SERVER_THUMBPRINT is set the environment is Service Fabric
					c.msiType = msiTypeServiceFabric
				}
			} else if arcIMDS := os.Getenv(arcIMDSEndpoint); arcIMDS != "" {
				c.msiType = msiTypeAzureArc
			} else {
				c.msiType = msiTypeUnavailable
				return c.msiType, &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: "This Managed Identity Environment is not supported yet"}
			}
		} else if c.imdsAvailable() { // if MSI_ENDPOINT is NOT set AND the IMDS endpoint is available the msiType is IMDS. This will timeout after 500 milliseconds
			c.endpoint = imdsEndpoint
			c.msiType = msiTypeIMDS
		} else { // if MSI_ENDPOINT is NOT set and IMDS endpoint is not available Managed Identity is not available
			c.msiType = msiTypeUnavailable
			return c.msiType, &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: "Make sure you are running in a valid Managed Identity Environment"}
		}
	}
	return c.msiType, nil
}

// performs an I/O request that has a timeout of 500 milliseconds
func (c *managedIdentityClient) imdsAvailable() bool {
	tempCtx, cancel := context.WithTimeout(context.Background(), c.imdsAvailableTimeout)
	defer cancel()
	// this should never fail
	request, _ := runtime.NewRequest(tempCtx, http.MethodGet, imdsEndpoint)
	q := request.Raw().URL.Query()
	q.Add("api-version", c.imdsAPIVersion)
	request.Raw().URL.RawQuery = q.Encode()
	resp, err := c.pipeline.Do(request)
	if err == nil {
		runtime.Drain(resp)
	}
	return err == nil
}
