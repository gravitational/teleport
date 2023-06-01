package jamf

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

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

	Username, Password string
}

// NewClient creates a new Jamf API client.
func NewClient(opts ClientOpts) (*Client, error) {
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

	return &Client{
		clock:      clock,
		logger:     logger,
		httpClient: opts.HTTPClient,
		baseURL:    baseURL.String(),
		username:   opts.Username,
		password:   opts.Password,
	}, nil
}

// UsePlainHTTP makes the client use "http" instead of "https".
// Don't do this in production, credentials and tokens will be exchanged in
// plaintext when using "http".
func (c *Client) UsePlainHTTP() {
	c.baseURL = strings.Replace(c.baseURL, "https://", "http://", 1)
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := resp.Body.Close(); err != nil {
		c.logger.WithError(err).Warn("Failed to close http.Response body")
	}

	if resp.StatusCode != 200 {
		return &APIError{
			StatusCode: resp.StatusCode,
			RawBody:    string(body),
		}
	}

	return trace.Wrap(json.Unmarshal(body, jsonResp))
}
