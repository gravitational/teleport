package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
)

const baseURL = "https://graph.microsoft.com/v1.0"

type UnsupportedGroupMember struct {
	Type string
}

func (u *UnsupportedGroupMember) Error() string {
	return fmt.Sprintf("Unsupported group member: %q", u.Type)
}

var scopes = []string{"https://graph.microsoft.com/.default"}

type azureTokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

type Config struct {
	TokenProvider azureTokenProvider
	HTTPClient    *http.Client
}

func (cfg *Config) SetDefaults() {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
}

func (cfg *Config) Validate() error {
	if cfg.TokenProvider == nil {
		return trace.BadParameter("TokenProvider must be set")
	}
	return nil
}

type client struct {
	httpClient    *http.Client
	tokenProvider azureTokenProvider
	baseURL       *url.URL
}

func NewClient(cfg Config) (Client, error) {
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	uri, err := url.Parse(baseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &client{
		httpClient:    cfg.HTTPClient,
		tokenProvider: cfg.TokenProvider,
		baseURL:       uri,
	}, nil
}

func (c *client) request(ctx context.Context, method string, uri string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, uri, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := c.tokenProvider.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: scopes,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to get azure authentication token")
	}
	req.Header.Add("Authorization", "Bearer "+token.Token)

	const defaultRetry = 1 * time.Second // TODO: RetryV2
	const maxRetries = 5
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		retryAfter := defaultRetry
		if i != 0 {
			time.Sleep(retryAfter) // TODO: clockwork.Clock
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, trace.Wrap(err) // hard I/O error, bail
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return resp, trace.Wrap(err)
		}

		resp.Body.Close()
		lastErr = trace.BadParameter("status %s", resp.Status)
		if !isRetriable(resp.StatusCode) {
			return nil, lastErr
		}
		// TODO: parse Retry-After or fall back to RetryV2
	}
	return nil, lastErr
}

func (c *client) iterate(ctx context.Context, endpoint string, f func(json.RawMessage) bool) error {
	const limit = 2 // TODO: make configurable, bump the default
	uri := *c.baseURL
	uri.Path = path.Join(uri.Path, endpoint)
	uri.RawQuery = url.Values{"$top": {strconv.Itoa(limit)}}.Encode()
	uriString := uri.String()
	for uriString != "" {
		resp, err := c.request(ctx, http.MethodGet, uriString)
		if err != nil {
			return trace.Wrap(err)
		}
		defer resp.Body.Close()

		var page oDataPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return trace.Wrap(err)
		}
		uriString = page.NextLink
		if !f(page.Value) {
			break
		}
	}

	return nil
}

// CreateFederatedIdentityCredential implements Client.
func (c *client) CreateFederatedIdentityCredential(ctx context.Context, cred *FederatedIdentityCredential) error {
	panic("unimplemented")
}

// CreateServicePrincipalTokenSigningCertificate implements Client.
func (c *client) CreateServicePrincipalTokenSigningCertificate(ctx context.Context, spID string, displayName string) (*SelfSignedCertificate, error) {
	panic("unimplemented")
}

// GetServicePrincipalsByAppId implements Client.
func (c *client) GetServicePrincipalsByAppId(ctx context.Context, appID string) ([]*ServicePrincipal, error) {
	panic("unimplemented")
}

// GetServicePrincipalsByDisplayName implements Client.
func (c *client) GetServicePrincipalsByDisplayName(ctx context.Context, displayName string) ([]*ServicePrincipal, error) {
	panic("unimplemented")
}

// InstantiateApplicationTemplate implements Client.
func (c *client) InstantiateApplicationTemplate(ctx context.Context, appTemplateID string, displayName string) (*ApplicationServicePrincipal, error) {
	panic("unimplemented")
}

// IterateApplications implements Client.
func (c *client) IterateApplications(ctx context.Context, f func(*Application) bool) error {
	return iterateSimple(c, ctx, "applications", f)
}

func decodeGroupMember(msg json.RawMessage) (GroupMember, error) {
	var temp struct {
		Type string `json:"@odata.type"`
	}

	if err := json.Unmarshal(msg, &temp); err != nil {
		return nil, trace.Wrap(err)
	}

	var err error
	var member GroupMember
	switch temp.Type {
	case "#microsoft.graph.user":
		var u *User
		err = json.Unmarshal(msg, &u)
		member = u
	default:
		err = trace.BadParameter("unsupported group member type: %s", temp.Type)
	}

	return member, trace.Wrap(err)
}

// IterateGroupMembers implements Client.
func (c *client) IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool) error {
	var err error
	itErr := c.iterate(ctx, path.Join("groups", groupID, "members"), func(msg json.RawMessage) bool {
		var page []json.RawMessage
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, entry := range page {
			var member GroupMember
			member, err = decodeGroupMember(entry)
			if err != nil {
				var gmErr *UnsupportedGroupMember
				if errors.As(err, &gmErr) {
					slog.Debug("unsupported group member", "type", gmErr.Type)
					err = nil // Reset so that we do not return the error up if this is the last entry
					continue
				} else {
					return false
				}
			}
			if !f(member) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

// IterateGroups implements Client.
func (c *client) IterateGroups(ctx context.Context, f func(*Group) bool) error {
	return iterateSimple(c, ctx, "groups", f)
}

// IterateUsers implements Client.
func (c *client) IterateUsers(ctx context.Context, f func(*User) bool) error {
	return iterateSimple(c, ctx, "users", f)
}

// iterateSimple implements pagination for "simple" object lists, where additional logic isn't needed
func iterateSimple[T any](c *client, ctx context.Context, endpoint string, f func(*T) bool) error {
	var err error
	itErr := c.iterate(ctx, endpoint, func(msg json.RawMessage) bool {
		var page []T
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, item := range page {
			if !f(&item) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

func isRetriable(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable
}
