/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package authz authorizes App access and answers requests that fail it. It's
// separate from lib/srv/app so that the Proxy can share it with the
// Application Service. Callers import it as appauthz, since they also import
// lib/authz.
package authz

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrUnsupportedApplicationType means that an application can't be served with
// a forwarded client identity.
var ErrUnsupportedApplicationType = &trace.AccessDeniedError{
	Message: "application can't be served with a forwarded client identity",
}

// Authorizer authorizes App access for the service that holds it. Both fields
// are required. When it hides why access was denied from the caller, it logs
// the reason with that service's logger.
type Authorizer struct {
	Logger               *slog.Logger
	AuthPreferenceGetter services.AuthPreferenceGetter
}

// Authorize checks that the identity in a client certificate that routes to app
// may access it. It returns the checker of the role that granted access, which
// holds that role's session controls.
func (a Authorizer) Authorize(ctx context.Context, authContext *authz.ScopedContext, app types.Application) (*services.AppAccessChecker, error) {
	state, err := a.accessState(ctx, authContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := a.authorizeWithState(ctx, authContext, app, state)
	return checker, trace.Wrap(err)
}

// AuthorizeForwarded checks that a client identity a Proxy forwards may access
// app. The identity must already route to app, since the checks read its
// route. It's shared so that a Proxy admitting a connection runs the same
// checks, in the same order, as the Application Service does for each request.
// Discovery comes first, so only an identity that may discover the application
// can get an error specific to it.
func (a Authorizer) AuthorizeForwarded(ctx context.Context, authContext *authz.ScopedContext, app types.Application) (*services.AppAccessChecker, error) {
	if _, err := authContext.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return trace.Wrap(checker.App().CanAccessApp(app))
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	if !componentfeatures.SupportsAppForwardedIdentityV1(app) {
		return nil, trace.Wrap(ErrUnsupportedApplicationType)
	}
	state, err := a.accessState(ctx, authContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// A forwarded identity comes from a certificate that routes to no
	// application, so MFA evidence in it would satisfy per-session MFA for
	// every application the user can access. Only the access state ignores it.
	// Locks, audit events and hardware key enforcement still read the
	// identity's fields.
	state.MFAVerified = false
	checker, err := a.authorizeWithState(ctx, authContext, app, state)
	return checker, trace.Wrap(err)
}

// ValidateForwardedIdentity checks that an identity a Proxy forwards has the
// shape of an App certificate for clusterName. The Proxy checks each identity
// it builds and the Application Service each one it receives, so the two share
// one definition. authz.Middleware only rejects a forwarded identity with a
// primary system role. It skips the scope pin and usage checks that it applies
// to a client certificate, so this function repeats them.
func ValidateForwardedIdentity(user authz.IdentityGetter, clusterName string) error {
	localUser, ok := user.(authz.LocalUser)
	if !ok {
		return trace.AccessDenied("forwarded identity must be a local user, got %T", user)
	}
	identity := localUser.Identity

	switch {
	case len(identity.SystemRoles) != 0:
		return trace.AccessDenied("forwarded identity has system roles")
	case identity.ScopePin != nil && identity.ScopePin.GetKind() != scopesv1.PinKind_PIN_KIND_USER:
		return trace.AccessDenied("forwarded identity has a %v scope pin", identity.ScopePin.GetKind())
	case len(identity.Usage) != 0 && !slices.Equal(identity.Usage, []string{teleport.UsageAppsOnly}):
		return trace.AccessDenied("forwarded identity is restricted to %v", identity.Usage)
	case identity.RouteToCluster != "" && identity.RouteToCluster != clusterName:
		return trace.AccessDenied("forwarded identity routes to cluster %q", identity.RouteToCluster)
	case identity.KubernetesCluster != "" || !identity.RouteToDatabase.Empty() ||
		identity.RouteToGit != (tlsca.RouteToGit{}):
		return trace.AccessDenied("forwarded identity routes to another service")
	}

	route := identity.RouteToApp
	switch {
	case route.SessionID == "" || route.Name == "" || route.PublicAddr == "":
		return trace.AccessDenied("forwarded identity has an incomplete app route")
	case route.ClusterName != clusterName:
		return trace.AccessDenied("forwarded identity routes to an app in cluster %q", route.ClusterName)
	case route.AWSRoleARN != "" || route.AzureIdentity != "" || route.GCPServiceAccount != "" ||
		route.TargetPort != 0 || route.AWSCredentialProcessCredentials != "":
		// These fields select an upstream identity or port for app types that
		// requestIdentityHandler does not serve.
		return trace.AccessDenied("forwarded identity has an app route for an unsupported app type")
	}
	return nil
}

// accessState returns the access state of authContext's identity, counting the
// MFA evidence it carries.
func (a Authorizer) accessState(ctx context.Context, authContext *authz.ScopedContext) (services.AccessState, error) {
	identity := authContext.Identity.GetIdentity()
	state, err := authContext.CheckerContext.AccessStateFromTLSIdentity(ctx, &identity, a.AuthPreferenceGetter)
	return state, trace.Wrap(err)
}

// authorizeWithState checks that authContext may access app in state, and
// returns the app checker of the role that granted access.
func (a Authorizer) authorizeWithState(ctx context.Context, authContext *authz.ScopedContext, app types.Application, state services.AccessState) (*services.AppAccessChecker, error) {
	identity := authContext.Identity.GetIdentity()

	if identity.RouteToApp.Scope != app.GetScope() {
		return nil, trace.AccessDenied("certificate app scope %q does not match application scope %q",
			identity.RouteToApp.Scope, app.GetScope())
	}

	// When accessing AWS management console, check permissions to assume
	// requested IAM role as well.
	var matchers []services.RoleMatcher
	if app.IsAWSConsole() {
		matchers = append(matchers, &services.AWSRoleARNMatcher{
			RoleARN: identity.RouteToApp.AWSRoleARN,
		})
	}

	// When accessing Azure API, check permissions to assume
	// requested Azure identity as well.
	if app.IsAzureCloud() {
		matchers = append(matchers, &services.AzureIdentityMatcher{
			Identity: identity.RouteToApp.AzureIdentity,
		})
	}

	// When accessing GCP API, check permissions to assume
	// requested GCP service account as well.
	if app.IsGCP() {
		matchers = append(matchers, &services.GCPServiceAccountMatcher{
			ServiceAccount: identity.RouteToApp.GCPServiceAccount,
		})
	}

	// Identity Center account apps are currently not supported for scoped
	// applications. TODO (williamo/scopes) - potentially look into adding
	// account_assignments into scoped roles.
	if _, isUnscoped := authContext.UnscopedContext(); !isUnscoped && app.GetSubKind() == types.KindIdentityCenterAccount {
		return nil, trace.AccessDenied("identity center account apps are not supported for scoped identities")
	}

	checker, err := authContext.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return trace.Wrap(checker.App().CheckAccessToApp(app, state, matchers...))
	})
	switch {
	case errors.Is(err, services.ErrTrustedDeviceRequired) || errors.Is(err, services.ErrSessionMFARequired):
		// When access is denied due to trusted device or session MFA
		// requirements, these specific errors are returned directly to provide
		// clarity to the client about the additional authentication
		// steps needed.
		return nil, trace.Wrap(err)
	case err != nil:
		// Other access denial errors are wrapped and obfuscated to prevent
		// leaking sensitive details.
		a.Logger.WarnContext(ctx, "Access denied to application.",
			"app", app.GetName(),
			"error", err,
		)
		return nil, utils.OpaqueAccessDenied(err)
	}

	return checker.App(), nil
}

// WriteForwardedError answers a request on the forwarded path that failed
// authorization with a 403 and a [common.AppErrorCode], so a client can tell it
// apart from the application's responses. Any NotFound or AccessDenied error,
// from whichever step, is access-denied, so a response never reveals whether
// an app the client may not discover exists. It writes nothing and returns
// false if err isn't an authorization failure.
func WriteForwardedError(w http.ResponseWriter, r *http.Request, err error) (common.AppErrorCode, bool) {
	code, ok := ForwardedErrorCode(err)
	if !ok {
		return "", false
	}

	w.Header().Set(common.TeleportAppErrorCodeHeader, string(code))
	if errors.Is(err, services.ErrTrustedDeviceRequired) {
		WriteTrustedDeviceRequired(w, r, http.StatusForbidden)
		return code, true
	}
	message := http.StatusText(http.StatusForbidden)
	switch code {
	case common.AppErrorCodeUnsupportedApplicationType:
		message = "This application requires a certificate for the application, such as one from `tsh app login`."
	case common.AppErrorCodeMFARequired:
		message = "This application requires per-session MFA, which needs a certificate for the application."
	}
	http.Error(w, message, http.StatusForbidden)
	return code, true
}

// ForwardedErrorCode returns the [common.AppErrorCode] that WriteForwardedError
// answers err with, or false if err isn't an authorization failure.
func ForwardedErrorCode(err error) (common.AppErrorCode, bool) {
	switch {
	case errors.Is(err, ErrUnsupportedApplicationType):
		return common.AppErrorCodeUnsupportedApplicationType, true
	case errors.Is(err, services.ErrSessionMFARequired):
		return common.AppErrorCodeMFARequired, true
	case errors.Is(err, authz.ErrUserNotFound), errors.Is(err, authz.ErrUserSessionRoleNotFound):
		return common.AppErrorCodeSessionInvalid, true
	// An unmet private key policy is a BadParameter whose message tells tsh to
	// log in again with a hardware key, which a client of this path can't
	// act on.
	case errors.Is(err, services.ErrTrustedDeviceRequired), trace.IsAccessDenied(err), trace.IsNotFound(err),
		keys.IsPrivateKeyPolicyError(err):
		return common.AppErrorCodeAccessDenied, true
	default:
		return "", false
	}
}

const (
	trustedDeviceRequiredDocsURL          = "https://goteleport.com/docs/zero-trust-access/device-trust/device-management/#troubleshooting"
	trustedDeviceRequiredWebUIDocsURL     = "https://goteleport.com/docs/zero-trust-access/device-trust/device-management/#web-ui-fails-to-authenticate-trusted-device"
	trustedDeviceRequiredAppAccessDocsURL = "https://goteleport.com/docs/zero-trust-access/device-trust/device-management/#app-access-and-access-to-this-app-requires-a-trusted-device"
)

// WriteTrustedDeviceRequired writes the response body for a request that failed
// with [services.ErrTrustedDeviceRequired]. Browsers receive a small HTML page
// with clickable links to the docs, and every other client gets plain text.
func WriteTrustedDeviceRequired(w http.ResponseWriter, r *http.Request, code int) {
	if isBrowserUserAgent(r.UserAgent()) {
		const body = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Trusted device required</title></head>
<body>
<p>A trusted device is required to access this resource, but this session has not been authorized with Device Trust. Follow <a href="` + trustedDeviceRequiredWebUIDocsURL + `" target="_blank">the Web UI troubleshooting guide</a> to authorize the session with Device Trust.</p>
<p>If accessing the resource through VNet or a local proxy, make sure the device running Teleport Connect or tsh is registered and enrolled. See <a href="` + trustedDeviceRequiredAppAccessDocsURL + `" target="_blank">the app access troubleshooting guide</a> for help.</p>
</body>
</html>
`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
		return
	}

	http.Error(w, `A trusted device is required to access this resource but this device has not been registered as a trusted device; use 'tsh device enroll' to register as a trusted device.

See `+trustedDeviceRequiredDocsURL+` for help.
`, code)
}

// isBrowserUserAgent reports whether ua plausibly comes from a web browser, as
// opposed to a CLI (tsh, curl) or some SDK client. It relies on the historical
// quirk that essentially every browser UA begins with "Mozilla/" and contains
// a known engine token. Modern browsers (Chrome, Safari, Edge, Opera, mobile
// browsers) are all WebKit- or Blink-based and carry "AppleWebKit", and the
// Firefox family carries "Gecko/".
func isBrowserUserAgent(ua string) bool {
	lower := strings.ToLower(ua)
	return strings.HasPrefix(lower, "mozilla/") &&
		(strings.Contains(lower, "applewebkit") || strings.Contains(lower, "gecko/"))
}
