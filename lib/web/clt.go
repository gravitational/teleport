package web

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport/lib/httplib"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

func newInsecureClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func newWebClient(url string, opts ...roundtrip.ClientParam) (*webClient, error) {
	clt, err := roundtrip.NewClient(url, Version, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &webClient{clt}, nil
}

// webClient is a package local lightweight client used
// in tests and some functions to handle errors properly
type webClient struct {
	*roundtrip.Client
}

func (w *webClient) PostJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PostJSON(endpoint, val))
}

func (w *webClient) PutJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.PutJSON(endpoint, val))
}

func (w *webClient) Get(endpoint string, val url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Get(endpoint, val))
}

func (w *webClient) Delete(endpoint string) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(w.Client.Delete(endpoint))
}
