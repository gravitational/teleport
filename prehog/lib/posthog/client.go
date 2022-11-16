package posthog

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type (
	EventName      string
	EventProperty  string
	PersonProperty string
)

// Events with special meaning in PostHog.
const (
	CreateAliasEvent EventName = "$create_alias"
)

// Properties with special meaning in PostHog.
const (
	AliasProperty EventProperty = "alias"
)

type Event struct {
	// APIKey is a PostHog write-only API key. Set by the client if empty.
	APIKey string `json:"api_key,omitempty"`

	DistinctID string                 `json:"distinct_id"`
	Event      EventName              `json:"event"`
	Timestamp  time.Time              `json:"timestamp"`
	Properties map[EventProperty]any  `json:"properties,omitempty"`
	Set        map[PersonProperty]any `json:"$set,omitempty"`
	SetOnce    map[PersonProperty]any `json:"$set_once,omitempty"`
}

func (e *Event) AddProperty(name EventProperty, value any) {
	if e.Properties == nil {
		e.Properties = map[EventProperty]any{
			name: value,
		}
	} else {
		e.Properties[name] = value
	}
}

func (e *Event) AddSet(name PersonProperty, value any) {
	if e.Set == nil {
		e.Set = map[PersonProperty]any{
			name: value,
		}
	} else {
		e.Set[name] = value
	}
}

func (e *Event) AddSetOnce(name PersonProperty, value any) {
	if e.SetOnce == nil {
		e.SetOnce = map[PersonProperty]any{
			name: value,
		}
	} else {
		e.SetOnce[name] = value
	}
}

type Batch struct {
	// APIKey is a PostHog write-only API key. Set by the client if empty.
	APIKey string `json:"api_key"`

	Batch []Event `json:"batch"`
}

func NewClient(posthogURL *url.URL, apiKey string, clientCert *tls.Certificate) *Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
	if clientCert != nil {
		tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return clientCert, nil
		}
	}

	captureURL := posthogURL.JoinPath("capture")
	decideURL := posthogURL.JoinPath("decide")
	q := decideURL.Query()
	q.Set("v", "2")
	decideURL.RawQuery = q.Encode()

	c := &Client{
		apiKey:     apiKey,
		captureURL: captureURL.String(),
		decideURL:  decideURL.String(),
		httpClient: http.Client{
			Transport: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				TLSClientConfig:   tlsConfig,
				ForceAttemptHTTP2: true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("no redirects expected")
			},
			Timeout: 5 * time.Second,
		},
	}

	return c
}

type Client struct {
	// apiKey is a PostHog write-only API key, seems to begin with `phc_`.
	apiKey string

	// captureURL is the URL of the /capture endpoint.
	captureURL string
	// decideURL is the URL of the /decide?v=2 endpoint.
	decideURL string

	httpClient http.Client
}

func (c *Client) Emit(ctx context.Context, event *Event) (time.Duration, error) {
	if event.APIKey == "" {
		event.APIKey = c.apiKey
	}

	j, err := json.Marshal(event)
	if err != nil {
		return 0, err
	}

	dur, err := c.postJSON(ctx, c.captureURL, j)
	if dur > 0 {
		EmitTotal.Inc()
		EmitDuration.Observe(dur.Seconds())
		if err != nil {
			EmitErrorTotal.Inc()
		}
	}

	return dur, err
}

func (c *Client) Check(ctx context.Context) error {
	// we use the /decide endpoint (for feature flags) to check if our API key
	// is valid, asking for the features enabled for a randomly generated
	// distinct_id
	j, err := json.Marshal(map[string]string{
		"api_key":     c.apiKey,
		"distinct_id": uuid.NewString(),
	})
	if err != nil {
		return err
	}

	_, err = c.postJSON(ctx, c.decideURL, j)
	return err
}

func (c *Client) postJSON(ctx context.Context, u string, j []byte) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(j))
	if err != nil {
		return 0, err
	}
	req.Header.Set("content-type", "application/json")

	t0 := time.Now()
	resp, err := c.httpClient.Do(req)
	dur := time.Since(t0)
	if err != nil {
		return dur, err
	}
	resp.Body = http.MaxBytesReader(nil, resp.Body, 65536)

	if resp.StatusCode == http.StatusOK {
		go discardBody(resp.Body)
		return dur, nil
	}

	errData, _ := io.ReadAll(resp.Body)
	go discardBody(resp.Body)

	return dur, fmt.Errorf("%v %+q", resp.StatusCode, errData)
}

func discardBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	body.Close()
}
