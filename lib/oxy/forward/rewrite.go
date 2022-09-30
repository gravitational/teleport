package forward

import (
	"net"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/oxy/utils"
)

// Rewriter is responsible for removing hop-by-hop headers and setting forwarding headers
type HeaderRewriter struct {
	TrustForwardHeader bool
	Hostname           string
}

func (rw *HeaderRewriter) Rewrite(req *http.Request) {
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if rw.TrustForwardHeader {
			if prior, ok := req.Header[XForwardedFor]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
		}
		req.Header.Set(XForwardedFor, clientIP)
	}

	port := ""
	if xfp := req.Header.Get(XForwardedProto); xfp != "" && rw.TrustForwardHeader {
		if xfp == "https" {
			port = "443"
		} else {
			port = "80"
		}
		req.Header.Set(XForwardedProto, xfp)
	} else if req.TLS != nil {
		req.Header.Set(XForwardedProto, "https")
		port = "443"
	} else {
		req.Header.Set(XForwardedProto, "http")
		port = "80"
	}

	if xfh := req.Header.Get(XForwardedHost); xfh != "" && rw.TrustForwardHeader {
		req.Header.Set(XForwardedHost, xfh)
	} else if req.Host != "" {
		req.Header.Set(XForwardedHost, req.Host)
	}

	if rw.Hostname != "" {
		req.Header.Set(XForwardedServer, rw.Hostname)
	}

	if req.TLS != nil {
		req.Header.Set(XForwardedSSL, "on")
	} else {
		req.Header.Set(XForwardedSSL, "off")
	}

	if req.URL.Port() != "" {
		req.Header.Set(XForwardedPort, req.URL.Port())
	} else {
		req.Header.Set(XForwardedPort, port)
	}

	// Remove hop-by-hop headers to the backend.  Especially important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	utils.RemoveHeaders(req.Header, HopHeaders...)
}
