package idp

import (
	"bytes"
	"compress/flate"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/beevik/etree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

// Config is a configuration for IDP.
type Config struct {
	// ProxyAddr is the address of the proxy.
	ProxyAddr string
	// Clock is a clock.
	Clock clockwork.Clock
	// Is the URL that initialize the IdP flow.
	// Where a client will be redirected to start the SAML flow.
	URL string
}

// New creates a new IDP.
func New(config Config) *IDP {
	return &IDP{
		Config: config,
	}
}

// IDP is a mocked identity provider.
type IDP struct {
	// Config is the configuration.
	Config
}

// Do perform client initialized SAML login.
func (s *IDP) Do(t *testing.T, userName string, groups []string) {
	samlRequestID := s.mustGetSAMLRequestID(t)
	request := mustGenerateRequest(t, samlRequestID, userName, groups, s.Clock)

	form := url.Values{}
	form.Add("SAMLResponse", base64.StdEncoding.EncodeToString([]byte(request)))

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/webapi/saml/acs/test", s.ProxyAddr), bytes.NewBufferString(form.Encode()))
	if !assert.NoError(t, err) {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()

	buff, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}

	redirectURL := extractURL(string(buff))
	u, err := url.Parse(redirectURL)
	if !assert.NoError(t, err) {
		return
	}

	req, err = http.NewRequest(http.MethodGet, redirectURL, strings.NewReader(u.Query().Encode()))
	if !assert.NoError(t, err) {
		return

	}
	req.PostForm = u.Query()

	resp, err = client.Do(req)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()
}

func (s *IDP) mustGetSAMLRequestID(t *testing.T) string {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	assert.NoError(t, err)
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	buff, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	requestID, err := DecodeSAMLResponse(string(buff))
	assert.NoError(t, err)
	return requestID
}

func DecodeSAMLResponse(response string) (string, error) {
	re := regexp.MustCompile(`SAMLRequest=([^&"]+)`)
	match := re.FindStringSubmatch(response)

	if len(match) != 2 {
		return "", fmt.Errorf("failed to decode SAML response")
	}

	payload, err := url.QueryUnescape(match[1])
	if err != nil {
		return "", trace.Wrap(err)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", trace.Wrap(err)
	}
	xmlPayload, err := io.ReadAll(flate.NewReader(bytes.NewReader(data)))
	if err != nil {
		return "", trace.Wrap(err)
	}
	doc := etree.NewDocument()
	err = doc.ReadFromBytes(xmlPayload)
	if err != nil {
		return "", trace.Wrap(err)
	}
	id := doc.Root().SelectAttr("ID")
	return id.Value, nil
}

func TestOktaSAMLConnector(orgURL string) string {
	return fmt.Sprintf(`
kind: saml
metadata:
  name: okta-pre-created-test
  revision: fe2ebe51-7ec8-42cb-9b16-b2cd7efed2d0
  labels:
    okta/org: %[1]s
    teleport.dev/origin: okta
spec:
  acs: https://sp.example.com/saml2/acs/test
  attributes_to_roles:
  - name: groups
    roles:
    - okta-requester
    value: Everyone
  audience: https://teleport.example.com:3080/v1/webapi/saml/acs/test
  cert: ""
  display: test
  entity_descriptor: |
    <EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2024-07-04T11:40:17.481Z" cacheDuration="PT48H" entityID="teleport.example.com/metadata">
     <IDPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <KeyDescriptor use="signing">
       <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data xmlns="http://www.w3.org/2000/09/xmldsig#">
         <X509Certificate xmlns="http://www.w3.org/2000/09/xmldsig#">MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQELBQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoXDTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABo1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAUH1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgRMbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbtRrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0msQ0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==</X509Certificate>
        </X509Data>
       </KeyInfo>
      </KeyDescriptor>
      <KeyDescriptor use="encryption">
       <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data xmlns="http://www.w3.org/2000/09/xmldsig#">
         <X509Certificate xmlns="http://www.w3.org/2000/09/xmldsig#">MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQELBQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoXDTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABo1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAUH1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgRMbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbtRrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0msQ0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==</X509Certificate>
        </X509Data>
       </KeyInfo>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"></EncryptionMethod>
      </KeyDescriptor>
      <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="teleport.example.com/logout"></SingleLogoutService>
      <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>
      <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%[1]s/app/example_test-okta-app-name/abcdefghijKlMnopR123/sso/saml"></SingleSignOnService>
      <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.go.saml.url.com/http/post"></SingleSignOnService>
     </IDPSSODescriptor>
    </EntityDescriptor>
  entity_descriptor_url: ""
  issuer: teleport.example.com/metadata
  service_provider_issuer: https://teleport.example.com:3080/v1/webapi/saml/acs/okta
  signing_key_pair:
    cert: |
      -----BEGIN CERTIFICATE-----
      MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQEL
      BQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoX
      DTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjAN
      BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6V
      RuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+c
      GmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP
      04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdye
      FVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/
      gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQAB
      o1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAU
      H1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsF
      AAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgR
      MbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/
      hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbt
      RrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0ms
      Q0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1
      Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==
      -----END CERTIFICATE-----
    private_key: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEpAIBAAKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9
      yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ
      4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPu
      fOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5t
      InWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2
      EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABAoIBAFWZwDTeESBdrLcT
      zHZe++cJLxE4AObn2LrWANEv5AeySYsyzjRBYObIN9IzrgTb8uJ900N/zVr5VkxH
      xUa5PKbOcowd2NMfBTw5EEnaNbILLm+coHdanrNzVu59I9TFpAFoPavrNt/e2hNo
      NMGPSdOkFi81LLl4xoadz/WR6O/7N2famM+0u7C2uBe+TrVwHyuqboYoidJDhO8M
      w4WlY9QgAUhkPyzZqrl+VfF1aDTGVf4LJgaVevfFCas8Ws6DQX5q4QdIoV6/0vXi
      B1M+aTnWjHuiIzjBMWhcYW2+I5zfwNWRXaxdlrYXRukGSdnyO+DH/FhHePJgmlkj
      NInADDkCgYEA6MEQFOFSCc/ELXYWgStsrtIlJUcsLdLBsy1ocyQa2lkVUw58TouW
      RciE6TjW9rp31pfQUnO2l6zOUC6LT9Jvlb9PSsyW+rvjtKB5PjJI6W0hjX41wEO6
      fshFELMJd9W+Ezao2AsP2hZJ8McCF8no9e00+G4xTAyxHsNI2AFTCQcCgYEA5cWZ
      JwNb4t7YeEajPt9xuYNUOQpjvQn1aGOV7KcwTx5ELP/Hzi723BxHs7GSdrLkkDmi
      Gpb+mfL4wxCt0fK0i8GFQsRn5eusyq9hLqP/bmjpHoXe/1uajFbE1fZQR+2LX05N
      3ATlKaH2hdfCJedFa4wf43+cl6Yhp6ZA0Yet1r8CgYEAwiu1j8W9G+RRA5/8/DtO
      yrUTOfsbFws4fpLGDTA0mq0whf6Soy/96C90+d9qLaC3srUpnG9eB0CpSOjbXXbv
      kdxseLkexwOR3bD2FHX8r4dUM2bzznZyEaxfOaQypN8SV5ME3l60Fbr8ajqLO288
      wlTmGM5Mn+YCqOg/T7wjGmcCgYBpzNfdl/VafOROVbBbhgXWtzsz3K3aYNiIjbp+
      MunStIwN8GUvcn6nEbqOaoiXcX4/TtpuxfJMLw4OvAJdtxUdeSmEee2heCijV6g3
      ErrOOy6EqH3rNWHvlxChuP50cFQJuYOueO6QggyCyruSOnDDuc0BM0SGq6+5g5s7
      H++S/wKBgQDIkqBtFr9UEf8d6JpkxS0RXDlhSMjkXmkQeKGFzdoJcYVFIwq8jTNB
      nJrVIGs3GcBkqGic+i7rTO1YPkquv4dUuiIn+vKZVoO6b54f+oPBXd4S0BnuEqFE
      rdKNuCZhiaE2XD9L/O9KP1fh5bfEcKwazQ23EvpJHBMm8BGC+/YZNw==
      -----END RSA PRIVATE KEY-----
  sso: %[1]s/app/example_test-okta-app-name/abcdefghijKlMnopR123/sso/saml
version: v2`, orgURL)
}

const (
	SAMLConnector = `
kind: saml
metadata:
  name: okta-pre-created-test
  revision: fe2ebe51-7ec8-42cb-9b16-b2cd7efed2d0
  labels:
    okta/org: https://trial-1234567.okta.com
    teleport.dev/origin: okta
spec:
  acs: https://sp.example.com/saml2/acs/test
  attributes_to_roles:
  - name: groups
    roles:
    - okta-requester
    value: Everyone
  audience: https://teleport.example.com:3080/v1/webapi/saml/acs/test
  cert: ""
  display: test
  entity_descriptor: |
    <EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2024-07-04T11:40:17.481Z" cacheDuration="PT48H" entityID="teleport.example.com/metadata">
     <IDPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <KeyDescriptor use="signing">
       <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data xmlns="http://www.w3.org/2000/09/xmldsig#">
         <X509Certificate xmlns="http://www.w3.org/2000/09/xmldsig#">MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQELBQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoXDTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABo1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAUH1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgRMbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbtRrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0msQ0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==</X509Certificate>
        </X509Data>
       </KeyInfo>
      </KeyDescriptor>
      <KeyDescriptor use="encryption">
       <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data xmlns="http://www.w3.org/2000/09/xmldsig#">
         <X509Certificate xmlns="http://www.w3.org/2000/09/xmldsig#">MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQELBQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoXDTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABo1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAUH1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgRMbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbtRrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0msQ0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==</X509Certificate>
        </X509Data>
       </KeyInfo>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes192-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes256-cbc"></EncryptionMethod>
       <EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"></EncryptionMethod>
      </KeyDescriptor>
      <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="teleport.example.com/logout"></SingleLogoutService>
      <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</NameIDFormat>
      <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://trial-1234567.okta.com/app/example_test-okta-app-name/exkmtd8mclmbcpx01697/sso/saml"></SingleSignOnService>
      <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.go.saml.url.com/http/post"></SingleSignOnService>
     </IDPSSODescriptor>
    </EntityDescriptor>
  entity_descriptor_url: ""
  issuer: teleport.example.com/metadata
  service_provider_issuer: https://teleport.example.com:3080/v1/webapi/saml/acs/okta
  signing_key_pair:
    cert: |
      -----BEGIN CERTIFICATE-----
      MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQEL
      BQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoX
      DTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjAN
      BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6V
      RuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+c
      GmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP
      04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdye
      FVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/
      gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQAB
      o1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAU
      H1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsF
      AAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgR
      MbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/
      hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbt
      RrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0ms
      Q0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1
      Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==
      -----END CERTIFICATE-----
    private_key: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEpAIBAAKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9
      yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ
      4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPu
      fOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5t
      InWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2
      EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABAoIBAFWZwDTeESBdrLcT
      zHZe++cJLxE4AObn2LrWANEv5AeySYsyzjRBYObIN9IzrgTb8uJ900N/zVr5VkxH
      xUa5PKbOcowd2NMfBTw5EEnaNbILLm+coHdanrNzVu59I9TFpAFoPavrNt/e2hNo
      NMGPSdOkFi81LLl4xoadz/WR6O/7N2famM+0u7C2uBe+TrVwHyuqboYoidJDhO8M
      w4WlY9QgAUhkPyzZqrl+VfF1aDTGVf4LJgaVevfFCas8Ws6DQX5q4QdIoV6/0vXi
      B1M+aTnWjHuiIzjBMWhcYW2+I5zfwNWRXaxdlrYXRukGSdnyO+DH/FhHePJgmlkj
      NInADDkCgYEA6MEQFOFSCc/ELXYWgStsrtIlJUcsLdLBsy1ocyQa2lkVUw58TouW
      RciE6TjW9rp31pfQUnO2l6zOUC6LT9Jvlb9PSsyW+rvjtKB5PjJI6W0hjX41wEO6
      fshFELMJd9W+Ezao2AsP2hZJ8McCF8no9e00+G4xTAyxHsNI2AFTCQcCgYEA5cWZ
      JwNb4t7YeEajPt9xuYNUOQpjvQn1aGOV7KcwTx5ELP/Hzi723BxHs7GSdrLkkDmi
      Gpb+mfL4wxCt0fK0i8GFQsRn5eusyq9hLqP/bmjpHoXe/1uajFbE1fZQR+2LX05N
      3ATlKaH2hdfCJedFa4wf43+cl6Yhp6ZA0Yet1r8CgYEAwiu1j8W9G+RRA5/8/DtO
      yrUTOfsbFws4fpLGDTA0mq0whf6Soy/96C90+d9qLaC3srUpnG9eB0CpSOjbXXbv
      kdxseLkexwOR3bD2FHX8r4dUM2bzznZyEaxfOaQypN8SV5ME3l60Fbr8ajqLO288
      wlTmGM5Mn+YCqOg/T7wjGmcCgYBpzNfdl/VafOROVbBbhgXWtzsz3K3aYNiIjbp+
      MunStIwN8GUvcn6nEbqOaoiXcX4/TtpuxfJMLw4OvAJdtxUdeSmEee2heCijV6g3
      ErrOOy6EqH3rNWHvlxChuP50cFQJuYOueO6QggyCyruSOnDDuc0BM0SGq6+5g5s7
      H++S/wKBgQDIkqBtFr9UEf8d6JpkxS0RXDlhSMjkXmkQeKGFzdoJcYVFIwq8jTNB
      nJrVIGs3GcBkqGic+i7rTO1YPkquv4dUuiIn+vKZVoO6b54f+oPBXd4S0BnuEqFE
      rdKNuCZhiaE2XD9L/O9KP1fh5bfEcKwazQ23EvpJHBMm8BGC+/YZNw==
      -----END RSA PRIVATE KEY-----
  sso: https://example.okta.com/app/example_test-okta-app-name/abcdefghijKlMnopR123/sso/saml
version: v2`

	idpKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9
yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ
4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPu
fOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5t
InWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2
EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABAoIBAFWZwDTeESBdrLcT
zHZe++cJLxE4AObn2LrWANEv5AeySYsyzjRBYObIN9IzrgTb8uJ900N/zVr5VkxH
xUa5PKbOcowd2NMfBTw5EEnaNbILLm+coHdanrNzVu59I9TFpAFoPavrNt/e2hNo
NMGPSdOkFi81LLl4xoadz/WR6O/7N2famM+0u7C2uBe+TrVwHyuqboYoidJDhO8M
w4WlY9QgAUhkPyzZqrl+VfF1aDTGVf4LJgaVevfFCas8Ws6DQX5q4QdIoV6/0vXi
B1M+aTnWjHuiIzjBMWhcYW2+I5zfwNWRXaxdlrYXRukGSdnyO+DH/FhHePJgmlkj
NInADDkCgYEA6MEQFOFSCc/ELXYWgStsrtIlJUcsLdLBsy1ocyQa2lkVUw58TouW
RciE6TjW9rp31pfQUnO2l6zOUC6LT9Jvlb9PSsyW+rvjtKB5PjJI6W0hjX41wEO6
fshFELMJd9W+Ezao2AsP2hZJ8McCF8no9e00+G4xTAyxHsNI2AFTCQcCgYEA5cWZ
JwNb4t7YeEajPt9xuYNUOQpjvQn1aGOV7KcwTx5ELP/Hzi723BxHs7GSdrLkkDmi
Gpb+mfL4wxCt0fK0i8GFQsRn5eusyq9hLqP/bmjpHoXe/1uajFbE1fZQR+2LX05N
3ATlKaH2hdfCJedFa4wf43+cl6Yhp6ZA0Yet1r8CgYEAwiu1j8W9G+RRA5/8/DtO
yrUTOfsbFws4fpLGDTA0mq0whf6Soy/96C90+d9qLaC3srUpnG9eB0CpSOjbXXbv
kdxseLkexwOR3bD2FHX8r4dUM2bzznZyEaxfOaQypN8SV5ME3l60Fbr8ajqLO288
wlTmGM5Mn+YCqOg/T7wjGmcCgYBpzNfdl/VafOROVbBbhgXWtzsz3K3aYNiIjbp+
MunStIwN8GUvcn6nEbqOaoiXcX4/TtpuxfJMLw4OvAJdtxUdeSmEee2heCijV6g3
ErrOOy6EqH3rNWHvlxChuP50cFQJuYOueO6QggyCyruSOnDDuc0BM0SGq6+5g5s7
H++S/wKBgQDIkqBtFr9UEf8d6JpkxS0RXDlhSMjkXmkQeKGFzdoJcYVFIwq8jTNB
nJrVIGs3GcBkqGic+i7rTO1YPkquv4dUuiIn+vKZVoO6b54f+oPBXd4S0BnuEqFE
rdKNuCZhiaE2XD9L/O9KP1fh5bfEcKwazQ23EvpJHBMm8BGC+/YZNw==
-----END RSA PRIVATE KEY-----`
	idpCert = `-----BEGIN CERTIFICATE-----
MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQEL
BQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoX
DTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6V
RuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+c
GmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP
04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdye
FVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/
gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQAB
o1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAU
H1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsF
AAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgR
MbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/
hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbt
RrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0ms
Q0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1
Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==
-----END CERTIFICATE-----`
)

func mustParsePrivateKey(t *testing.T) crypto.PrivateKey {
	b, _ := pem.Decode([]byte(idpKey))
	k, err := x509.ParsePKCS1PrivateKey(b.Bytes)
	assert.NoError(t, err)
	return k
}

func mustParseCert(t *testing.T) *x509.Certificate {
	b, _ := pem.Decode([]byte(idpCert))
	c, err := x509.ParseCertificate(b.Bytes)
	assert.NoError(t, err)
	return c
}

// EntraIDSAMLConnector generates a SAML connector spec
// of the given [name]. Connector spec may need some tuning to
// be usable in a real SSO login simulation.
func EntraIDSAMLConnector(name string) string {
	return fmt.Sprintf(entraIDSAMLConnector, name)
}

// entraIDSAMLConnector defines connector spec for
// the Entra ID SAML IdP.
const entraIDSAMLConnector = `
kind: saml
metadata:
  name: %s
spec:
  acs: https://teleport.example.com/v1/webapi/saml/acs/entra-id
  allow_idp_initiated: true
  attributes_to_roles:
  - name: http://schemas.microsoft.com/ws/2008/06/identity/claims/groups
    roles:
    - requester
    value: '*'
  audience: https://teleport.example.com/v1/webapi/saml/acs/entra-id
  cert: ""
  display: entra-id
  entity_descriptor: |
    <?xml version="1.0" encoding="utf-8"?>
    <EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" ID="_b936b9ca-abcd-abcd-abcd-4bc7415asd154" entityID="https://sts.windows.net/tenantid/">
    <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
    <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
    <X509Data>MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQELBQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoXDTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABo1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAUH1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgRMbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbtRrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0msQ0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==</X509Data>
    </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://login.microsoftonline.com/tenantid/saml2"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://login.microsoftonline.com/tenantid/saml2"/>
    </IDPSSODescriptor>
    </EntityDescriptor> 
  issuer: https://sts.windows.net/tenantid/
  service_provider_issuer: https://teleport.example.com/v1/webapi/saml/acs/entra-id
  signing_key_pair:
    cert: |
      -----BEGIN CERTIFICATE-----
      MIIDEjCCAfqgAwIBAgIUNjOYL6/mdV/Cic7q6nZ5gf90UI0wDQYJKoZIhvcNAQEL
      BQAwGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMB4XDTI1MTIzMTE2MTEwNFoX
      DTM1MTIyOTE2MTEwNFowGjEYMBYGA1UEAwwPd3d3LmV4YW1wbGUuY29tMIIBIjAN
      BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6V
      RuAs3nNibiuifwCGz6u9yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+c
      GmLNwPOOA0PaD5q5xKhQ4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP
      04Oc7Uymq2v09bGb6nPufOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdye
      FVU1JHiQe444xLoPul5tInWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/
      gwEFVwifIHihRE0Hazn2EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQAB
      o1AwTjAdBgNVHQ4EFgQUH1mSULU6uTbOXnrODTx6Luyz/6IwHwYDVR0jBBgwFoAU
      H1mSULU6uTbOXnrODTx6Luyz/6IwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsF
      AAOCAQEAXgWbXmuE+GvntmpCZmnXmAzdv7nE76YRCzJUqlC/XqYei0XilVB7imgR
      MbID9Y2qPY7IWyJchkp6G+jiKlUvctRo/6GMK3R7xzYYAFrXn/X7HXKg2iMstaY/
      hlD9Bc9inH6itHIcu801+r/UhT8EuMXKZDcuVD2SwlO4jplr2trQSq7+9rAfwMbt
      RrJsZdEjx+8kyXbbC40D0yhKjW7rXdZyCc+yutSSa/896wJcPHjn4rZEUsM0N0ms
      Q0UncXcRm2bqixLZKZ4QbvMqdLCIjni0/zzrcfw4idALhPpz8Z08Roj+bZaSWHy1
      Dj4gfN7WonlA3eDYhzcxzpc6/2eIgg==
      -----END CERTIFICATE-----
    private_key: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEpAIBAAKCAQEA0OhbMuizgtbFOfwbK7aURuXhZx6VRuAs3nNibiuifwCGz6u9
      yy7bOR0P+zqN0YkjxaokqFgra7rXKCdeABmoLqCC0U+cGmLNwPOOA0PaD5q5xKhQ
      4Me3rt/R9C4Ca6k3/OnkxnKwnogcsmdgs2l8liT3qVHP04Oc7Uymq2v09bGb6nPu
      fOrkXS9F6mSClxHG/q59AGOWsXK1xzIRV1eu8W2SNdyeFVU1JHiQe444xLoPul5t
      InWasKayFsPlJfWNc8EoU8COjNhfo/GovFTHVjh9oUR/gwEFVwifIHihRE0Hazn2
      EQSLaOr2LM0TsRsQroFjmwSGgI+X2bfbMTqWOQIDAQABAoIBAFWZwDTeESBdrLcT
      zHZe++cJLxE4AObn2LrWANEv5AeySYsyzjRBYObIN9IzrgTb8uJ900N/zVr5VkxH
      xUa5PKbOcowd2NMfBTw5EEnaNbILLm+coHdanrNzVu59I9TFpAFoPavrNt/e2hNo
      NMGPSdOkFi81LLl4xoadz/WR6O/7N2famM+0u7C2uBe+TrVwHyuqboYoidJDhO8M
      w4WlY9QgAUhkPyzZqrl+VfF1aDTGVf4LJgaVevfFCas8Ws6DQX5q4QdIoV6/0vXi
      B1M+aTnWjHuiIzjBMWhcYW2+I5zfwNWRXaxdlrYXRukGSdnyO+DH/FhHePJgmlkj
      NInADDkCgYEA6MEQFOFSCc/ELXYWgStsrtIlJUcsLdLBsy1ocyQa2lkVUw58TouW
      RciE6TjW9rp31pfQUnO2l6zOUC6LT9Jvlb9PSsyW+rvjtKB5PjJI6W0hjX41wEO6
      fshFELMJd9W+Ezao2AsP2hZJ8McCF8no9e00+G4xTAyxHsNI2AFTCQcCgYEA5cWZ
      JwNb4t7YeEajPt9xuYNUOQpjvQn1aGOV7KcwTx5ELP/Hzi723BxHs7GSdrLkkDmi
      Gpb+mfL4wxCt0fK0i8GFQsRn5eusyq9hLqP/bmjpHoXe/1uajFbE1fZQR+2LX05N
      3ATlKaH2hdfCJedFa4wf43+cl6Yhp6ZA0Yet1r8CgYEAwiu1j8W9G+RRA5/8/DtO
      yrUTOfsbFws4fpLGDTA0mq0whf6Soy/96C90+d9qLaC3srUpnG9eB0CpSOjbXXbv
      kdxseLkexwOR3bD2FHX8r4dUM2bzznZyEaxfOaQypN8SV5ME3l60Fbr8ajqLO288
      wlTmGM5Mn+YCqOg/T7wjGmcCgYBpzNfdl/VafOROVbBbhgXWtzsz3K3aYNiIjbp+
      MunStIwN8GUvcn6nEbqOaoiXcX4/TtpuxfJMLw4OvAJdtxUdeSmEee2heCijV6g3
      ErrOOy6EqH3rNWHvlxChuP50cFQJuYOueO6QggyCyruSOnDDuc0BM0SGq6+5g5s7
      H++S/wKBgQDIkqBtFr9UEf8d6JpkxS0RXDlhSMjkXmkQeKGFzdoJcYVFIwq8jTNB
      nJrVIGs3GcBkqGic+i7rTO1YPkquv4dUuiIn+vKZVoO6b54f+oPBXd4S0BnuEqFE
      rdKNuCZhiaE2XD9L/O9KP1fh5bfEcKwazQ23EvpJHBMm8BGC+/YZNw==
      -----END RSA PRIVATE KEY-----
  sso: https://login.microsoftonline.com/tenantid/saml2
version: v2`
