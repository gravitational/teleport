package authn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"

	"github.com/gravitational/prehog/lib/license"
)

const (
	noClientCertMsg   = "no client certificate"
	invalidLicenseMsg = "invalid license"
	unauthorizedMsg   = "unauthorized"
)

type authnState struct {
	isCA    bool
	license *license.License
}

// ConnContext returns a ConnContext function to be used by http.Server, that
// will add validated data to a connection's context regarding the client
// certificate being a valid Teleport license file, or it being a known CA.
func ConnContext(trusted *x509.CertPool) func(context.Context, net.Conn) context.Context {
	return func(ctx context.Context, c net.Conn) context.Context {
		t, ok := c.(*tls.Conn)
		if !ok {
			return ctx
		}

		state := t.ConnectionState()
		if !state.HandshakeComplete {
			// same behavior as (*net/http.conn).serve
			dl := time.Now().Add(time.Second)
			_ = t.SetReadDeadline(dl)
			_ = t.SetWriteDeadline(dl)
			err := t.HandshakeContext(ctx)
			_ = t.SetReadDeadline(time.Time{})
			_ = t.SetWriteDeadline(time.Time{})
			if err != nil {
				// the http server will catch and handle the same error immediately after this
				return ctx
			}
			state = t.ConnectionState()
		}

		if len(state.VerifiedChains) < 1 {
			return ctx
		}

		var a authnState
		for _, chain := range state.VerifiedChains {
			if len(chain) == 1 {
				a.isCA = true
				break
			}
		}

		l, err := license.LicenseFromCert(state.PeerCertificates[0])
		if err == nil {
			a.license = &l
		}

		return context.WithValue(ctx, (*authnState)(nil), a)
	}
}

func stateFromContext(ctx context.Context) (authnState, bool) {
	a, ok := ctx.Value((*authnState)(nil)).(authnState)
	return a, ok
}

// RequireLicense is a http.Handler middleware that requires that the client has
// a client certificate with a valid Teleport license payload.
func RequireLicense(next http.Handler) http.Handler {
	// TODO(espadolini): gracefully accept events from unauthenticated clients, log bad or weird licenses
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := stateFromContext(r.Context())
		if !ok {
			sendError(w, r, noClientCertMsg, connect.CodeUnauthenticated, http.StatusUnauthorized)
			return
		}

		if a.license == nil {
			sendError(w, r, invalidLicenseMsg, connect.CodeUnauthenticated, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LicenseFromContext returns the License associated to the context's
// connection, if any.
func LicenseFromContext(ctx context.Context) (license.License, bool) {
	a, _ := stateFromContext(ctx)
	if a.license == nil {
		return license.License{}, false
	}
	return *a.license, true
}

// RequireCA is a http.Handler middleware that requires that the client has a
// client certificate that's a known CA.
func RequireCA(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := stateFromContext(r.Context())
		if !ok {
			sendError(w, r, noClientCertMsg, connect.CodeUnauthenticated, http.StatusUnauthorized)
			return
		}

		if !a.isCA {
			sendError(w, r, unauthorizedMsg, connect.CodeUnauthenticated, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IsCAFromContext returns true if the context is from a connection with a
// client certificate that's a known CA.
func IsCAFromContext(ctx context.Context) bool {
	a, _ := stateFromContext(ctx)
	return a.isCA
}

// IsValidCertFromContext returns true if the context is from a connection that
// has a valid client certificate.
func IsValidCertFromContext(ctx context.Context) bool {
	_, ok := stateFromContext(ctx)
	return ok
}

func sendError(
	w http.ResponseWriter, r *http.Request,
	message string, connectCode connect.Code, httpCode int,
) {
	ew := connect.NewErrorWriter()
	if ew.IsSupported(r) {
		_ = ew.Write(w, r, connect.NewError(connectCode, errors.New(message)))
	} else {
		http.Error(w, message, httpCode)
	}
}
