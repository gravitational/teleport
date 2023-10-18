package jamf

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrJamfClientInvalidCredential is returned by Jamf client when the Jamf API credentials are invalid.
var ErrJamfClientInvalidCredential = errors.New("invalid Jamf API credentials")

// Client is the Jamf API client.
// It automatically manages authentication and refreshes existing bearer tokens,
// as appropriate.
type Client struct {
	clock      clockwork.Clock
	logger     log.FieldLogger
	httpClient *http.Client

	baseURL            string
	username, password string

	// mu guards the fields below it.
	mu                    sync.Mutex
	currentToken          *AuthToken
	repeatedAuthnFailures int
}

// ClientOpts are the creation options for the [Client].
type ClientOpts struct {
	Clock  clockwork.Clock
	Logger log.FieldLogger

	HTTPClient *http.Client

	// APIURL is the URL for the Jamf API, usually including the "/api" path.
	// Example: "https://yourtenant.jamfcloud.com/api".
	APIURL string
	// Username for the Jamf API.
	Username string
	// Password for the Jamf API.
	Password string
}

// NewClient creates a new Jamf API client.
// `ctx` is used to verify credentials against the Jamf API.
func NewClient(ctx context.Context, opts ClientOpts) (*Client, error) {
	switch {
	case opts.HTTPClient == nil:
		return nil, trace.BadParameter("param HTTPClient required")
	case opts.APIURL == "":
		return nil, trace.BadParameter("param APIURL required")
	case opts.Username == "":
		return nil, trace.BadParameter("param Username required")
	case opts.Password == "":
		return nil, trace.BadParameter("param Password required")
	}

	u, err := url.Parse(opts.APIURL)
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case u.Host == "":
		return nil, trace.BadParameter("jamf APIURL lacks host: %q", opts.APIURL)
	}
	baseURL := &url.URL{
		Scheme: "https",
		Host:   u.Host,
		Path:   strings.TrimSuffix(u.Path, "/"),
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.New()
	}

	clock := opts.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	// Forbid HTTP downgrades or changing from the base host.
	// https://github.com/gravitational/teleport-private/issues/916.
	httpClient := &http.Client{
		Transport: opts.HTTPClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			switch {
			// No downgrades, no host/port changes.
			// (Note that req.URL.Host already captures the port.)
			case req.URL.Scheme != "https" || req.URL.Host != baseURL.Host:
				return http.ErrUseLastResponse
			case opts.HTTPClient.CheckRedirect != nil:
				return opts.HTTPClient.CheckRedirect(req, via)
			default:
				return nil
			}
		},
		Jar:     opts.HTTPClient.Jar,
		Timeout: opts.HTTPClient.Timeout,
	}

	c := &Client{
		clock:      clock,
		logger:     logger,
		httpClient: httpClient,
		baseURL:    baseURL.String(),
		username:   opts.Username,
		password:   opts.Password,
	}
	if err := c.verifyCredentials(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

func (c *Client) verifyCredentials(ctx context.Context) error {
	_, err := c.GetComputersInventory(ctx, &GetComputersInventoryRequest{
		Page:     0,
		PageSize: 1,
	})
	if err == nil {
		c.logger.Debugf("Jamf API: Authentication against %q successful", c.baseURL)
		return nil // Success
	}

	// Return ignored on purpose, makes no difference in the logic below.
	apiError := &APIError{}
	_ = errors.As(err, &apiError)

	switch {
	case apiError.StatusCode == http.StatusUnauthorized:
		return trace.Wrap(ErrJamfClientInvalidCredential)
	case apiError.StatusCode == http.StatusNotFound && !strings.HasSuffix(c.baseURL, "/api"):
		c.baseURL += "/api"
		return c.verifyCredentials(ctx)
	default:
		return trace.Wrap(err, "connecting to Jamf API")
	}
}

func (c *Client) endpoint(path string) string {
	return c.baseURL + path
}

func (c *Client) nowUTC() time.Time {
	return c.clock.Now().UTC()
}

func (c *Client) doJSONRequest(req *http.Request, jsonResp any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// Always drain the body, regardless of the status code.
	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := resp.Body.Close(); err != nil {
		c.logger.WithError(err).Warn("Jamf API: Failed to close http.Response body")
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(log.Fields{
			"body":   string(body),
			"status": resp.StatusCode,
			"url":    req.URL,
		}).Debug("Jamf API: API request failed")
		return &APIError{
			StatusCode: resp.StatusCode,
		}
	}

	return trace.Wrap(json.Unmarshal(body, jsonResp))
}
