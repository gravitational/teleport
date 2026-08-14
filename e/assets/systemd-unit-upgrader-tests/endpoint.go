package unit

import (
	"fmt"
	"net/http"
	"os"
	"sync"
)

const defaultEndpointPath = "/v1/stable/cloud"

// UpgradeEndpoint is a helper for mocking the upgrade endpoint.
type UpgradeEndpoint struct {
	*http.Server

	mu       sync.Mutex
	path     string
	version  string
	critical string
}

// NewUpgradeEndpoint sets up a new upgrade endpoint.
func NewUpgradeEndpoint(path string) *UpgradeEndpoint {
	srv := &http.Server{}
	if path == "" {
		path = defaultEndpointPath
	}

	e := &UpgradeEndpoint{
		Server: srv,
		path:   path,
	}

	srv.Handler = e

	return e
}

func (e *UpgradeEndpoint) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(os.Stderr, "---> %s %s\n", req.Method, req.URL.Path)

	if req.Method != http.MethodGet {
		rsp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rsp, "unsupported method: %q", req.Method)
		return
	}

	switch req.URL.Path {
	case fmt.Sprintf("%s/version", e.GetPath()):
		fmt.Fprintf(rsp, "%s\n", e.GetVersion())

	case fmt.Sprintf("%s/critical", e.GetPath()):
		fmt.Fprintf(rsp, "%s\n", e.GetCritical())

	default:
		rsp.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rsp, "unknown path: %q", req.URL.Path)
	}
}

func (e *UpgradeEndpoint) SetPath(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.path = s
}

func (e *UpgradeEndpoint) GetPath() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

func (e *UpgradeEndpoint) SetVersion(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.version = s
}

func (e *UpgradeEndpoint) GetVersion() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.version
}

func (e *UpgradeEndpoint) SetCritical(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.critical = s
}

func (e *UpgradeEndpoint) GetCritical() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.critical
}
