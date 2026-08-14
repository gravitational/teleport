package github

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v70/github"
	"github.com/gravitational/trace"
)

var errApplicationNotInstalled = &trace.NotFoundError{Message: "application not installed in the organization"}

type roundTripper struct {
	transport        http.RoundTripper
	token            *github.InstallationToken
	mu               sync.Mutex
	clientID         string
	organizationName string
	appInstallID     int64
	privateKeyData   []byte
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	token, err := r.getToken(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Authorization", "Bearer "+token.GetToken())
	return r.transport.RoundTrip(req)
}

func (r *roundTripper) getToken(ctx context.Context) (*github.InstallationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.token != nil && time.Now().Before(r.token.ExpiresAt.Add(-time.Minute)) {
		return r.token, nil
	}
	token, err := r.getInstallToken(ctx, r.clientID, r.privateKeyData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.token = token
	return token, nil
}

func (r *roundTripper) getInstallToken(ctx context.Context, clientID string, privKeyData []byte) (*github.InstallationToken, error) {
	// Parse the private key
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privKeyData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the JWT
	claims := jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute * 10).Unix(),
		"iss": clientID,
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := jwtToken.SignedString(privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a GitHub client with JWT authentication
	reqHeaders := map[string]string{
		"Authorization": "Bearer " + signedToken,
		"Accept":        "application/vnd.github+json",
	}
	client := github.NewClient(&http.Client{
		Transport: &jwtTransport{
			transport: http.DefaultTransport,
			headers:   reqHeaders,
		},
	})

	if r.appInstallID == 0 {
		// Get the installation ID for the app
		var err error
		r.appInstallID, err = r.getInstallation(ctx, client, r.organizationName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// Revoke the installation token to create a new one
	readPerm := "read"
	token, _, err := client.Apps.CreateInstallationToken(ctx, r.appInstallID, &github.InstallationTokenOptions{
		Permissions: &github.InstallationPermissions{
			OrganizationPersonalAccessTokens: &readPerm,
			Members:                          &readPerm,
			OrganizationCustomOrgRoles:       &readPerm,
			Metadata:                         &readPerm,
			OrganizationAdministration:       &readPerm,
		},
	})
	return token, trace.Wrap(err)
}

func (r *roundTripper) getInstallation(ctx context.Context, client *github.Client, orgName string) (int64, error) {
	var page int
	for {
		installations, rsp, err := client.Apps.ListInstallations(ctx, &github.ListOptions{
			Page: page,
		})
		if err != nil {
			return 0, trace.Wrap(err)
		}

		for _, installation := range installations {
			if installation.GetAccount().GetLogin() == orgName {
				return installation.GetID(), nil
			}
		}

		if rsp.NextPage == 0 {
			break
		}
		page = rsp.NextPage
	}
	return 0, trace.Wrap(errApplicationNotInstalled)
}

type jwtTransport struct {
	transport http.RoundTripper
	headers   map[string]string
}

func (t *jwtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())
	for key, value := range t.headers {
		reqClone.Header.Set(key, value)
	}
	return t.transport.RoundTrip(reqClone)
}
