/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"cmp"
	"context"
	"crypto/x509"
	"fmt"
	"iter"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"golang.org/x/net/idna"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// AppGetter defines interface for fetching application resources.
type AppGetter interface {
	// GetApps returns all application resources.
	GetApps(context.Context) ([]types.Application, error)
	// ListApps returns a page of application resources.
	ListApps(ctx context.Context, limit int, startKey string) ([]types.Application, string, error)
	// Apps returns application resources within the range [start, end).
	Apps(ctx context.Context, start, end string) iter.Seq2[types.Application, error]
	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)
}

// Applications defines an interface for managing application resources.
type Applications interface {
	// AppGetter provides methods for fetching application resources.
	AppGetter
	// CreateApp creates a new application resource.
	CreateApp(context.Context, types.Application) error
	// UpdateApp updates an existing application resource.
	UpdateApp(context.Context, types.Application) error
	// DeleteApp removes the specified application resource.
	DeleteApp(ctx context.Context, name string) error
	// DeleteAllApps removes all database resources.
	DeleteAllApps(context.Context) error
}

// ApplicationsInternal extends the Access interface with auth-specific internal methods.
type ApplicationsInternal interface {
	Applications

	// AppendPutAppActions adds conditional actions to an atomic write to create
	// or update an application resource.
	AppendPutAppActions(
		actions []backend.ConditionalAction,
		app types.Application,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)

	// AppendDeleteAppActions adds conditional actions to an atomic write to
	// delete an application resource.
	AppendDeleteAppActions(
		actions []backend.ConditionalAction,
		name string,
		condition backend.Condition,
	) ([]backend.ConditionalAction, error)
}

// ValidateApp validates the Application resource.
func ValidateApp(app types.Application, proxyGetter ProxyGetter) error {
	if app.GetTLS() != nil {
		if err := validateAppTLS(app); err != nil {
			return trace.Wrap(err)
		}
	}

	// If no public address is set, there's nothing to validate.
	if app.GetPublicAddr() == "" {
		return nil
	}

	// The app's spec has already been validated in CheckAndSetDefaults, so we can assume the public address is a valid
	// address. The remainder of this function focuses on detecting conflicts with proxy public addresses because the
	// proxy addresses are not part of the app spec and need to be fetched separately.
	appAddr, err := utils.ParseAddr(app.GetPublicAddr())
	if err != nil {
		return trace.Wrap(err)
	}

	// Convert the application's public address hostname to its ASCII representation for comparison. Strip any trailing
	// dots to ensure consistent comparison.
	asciiAppHostname, err := idna.ToASCII(strings.TrimRight(appAddr.Host(), "."))
	if err != nil {
		return trace.Wrap(err, "app %q has an invalid IDN hostname %q", app.GetName(), appAddr.Host())
	}

	proxyServers, err := clientutils.CollectWithFallback(context.TODO(), proxyGetter.ListProxyServers, func(context.Context) ([]types.Server, error) {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		return proxyGetter.GetProxies()
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Prevent routing conflicts and session hijacking by ensuring the application's public address does not match the
	// public address of any proxy. If an application shares a public address with a proxy, requests intended for the
	// proxy could be misrouted to the application, compromising security.
	for _, proxyServer := range proxyServers {
		proxyAddrs, err := utils.ParseAddrs(proxyServer.GetPublicAddrs())
		if err != nil {
			return trace.Wrap(err)
		}

		for _, proxyAddr := range proxyAddrs {
			// Also convert the proxy's public address hostname to its ASCII representation for comparison and strip any
			// trailing dots.
			asciiProxyHostname, err := idna.ToASCII(strings.TrimRight(proxyAddr.Host(), "."))
			if err != nil {
				return trace.Wrap(err, "proxy %q has an invalid IDN hostname %q", proxyServer.GetName(), proxyAddr)
			}

			// Compare the ASCII-normalized hostnames for equality, ignoring case.
			if strings.EqualFold(asciiProxyHostname, asciiAppHostname) {
				return trace.BadParameter(
					"Application %q public address %q conflicts with the Teleport Proxy public address. "+
						"Configure the application to use a unique public address that does not match the proxy's public addresses. "+
						"Refer to https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#customize-public-address.",
					app.GetName(),
					app.GetPublicAddr(),
				)
			}
		}
	}

	return nil
}

// validateAppTLS validates application TLS options.
func validateAppTLS(a types.Application) error {
	if !types.AppSupportsTLSConfig(a.GetURI()) {
		return trace.BadParameter(
			"App %q can only specify 'tls' settings for URI schemes that use upstream TLS. Supported schemes are: %s",
			a.GetName(),
			quoteAndJoin(types.AppSchemesWithTLSSupport),
		)
	}

	tls := a.GetTLS()
	var mode types.AppTLSMode
	switch tls.Mode {
	case types.AppTLSModeInsecure,
		types.AppTLSModeVerifyFull,
		types.AppTLSModeVerifyServerName,
		types.AppTLSModeVerifySpiffeID:
		mode = tls.Mode
	case "":
		// When not specified, use the evaluated mode.
		mode = a.GetTLSMode()
	default:
		return trace.BadParameter(
			"App %q has invalid 'tls.mode' %q. Supported values are: %s",
			a.GetName(),
			tls.Mode,
			quoteAndJoin([]string{
				types.AppTLSModeInsecure,
				types.AppTLSModeVerifyFull,
				types.AppTLSModeVerifyServerName,
				types.AppTLSModeVerifySpiffeID,
			}),
		)
	}

	if a.GetInsecureSkipVerify() && mode != types.AppTLSModeInsecure {
		return trace.BadParameter(
			"App %q cannot specify 'insecure_skip_verify: true' (deprecated) and 'tls.mode: %q'. Drop 'insecure_skip_verify', and if you want the app to use insecure connections set 'tls.mode: %q'",
			a.GetName(),
			mode,
			types.AppTLSModeInsecure,
		)
	}

	switch tls.ClientCertMode {
	case types.AppClientCertModeManaged:
		if mode == types.AppTLSModeInsecure {
			return trace.BadParameter("App %q can only enable 'tls.client_cert_mode' when 'tls.mode' is %q", a.GetName(), types.AppTLSModeVerifyFull)
		}
	case types.AppClientCertModeDisabled, "":
	default:
		return trace.BadParameter(
			"App %q has invalid 'tls.client_cert_mode'. Supported values are: %s",
			a.GetName(),
			quoteAndJoin([]string{"", types.AppClientCertModeDisabled, types.AppClientCertModeManaged}),
		)
	}

	switch mode {
	case types.AppTLSModeVerifyFull:
		// Note: tls.ServerName is optional and doesn't require any specific validation.
		if err := isValidSpiffeID(tls.ServerSpiffeId); err != nil {
			return trace.BadParameter("App %q has invalid `tls.server_spiffe_id`. The SPIFFE ID must be complete (trust domain and path) and start with 'spiffe://': %v", a.GetName(), err)
		}
	case types.AppTLSModeVerifyServerName:
		// Note: tls.ServerName is optional and doesn't require any specific validation.
		if tls.ServerSpiffeId != "" {
			return trace.BadParameter("App %q 'tls.server_spiffe_id' is not used when mode is set to %q. To perform both, server name and SPIFFE ID verifications use %q mode", a.GetName(), mode, types.AppTLSModeVerifyFull)
		}
	case types.AppTLSModeVerifySpiffeID:
		if err := isValidSpiffeID(tls.ServerSpiffeId); err != nil {
			return trace.BadParameter("App %q has invalid `tls.server_spiffe_id`. The SPIFFE ID must be complete (trust domain and path) and start with 'spiffe://': %v", a.GetName(), err)
		}
		if tls.ServerName != "" {
			return trace.BadParameter("App %q 'tls.server_name' is not used when mode is set to %q. To perform both, server name and SPIFFE ID verifications use %q mode", a.GetName(), mode, types.AppTLSModeVerifyFull)
		}
	case types.AppTLSModeInsecure:
		if tls.ServerName != "" || tls.ServerSpiffeId != "" || len(tls.AllowedCas) > 0 {
			return trace.BadParameter("App %q 'tls' are not in use since mode is set to %q", a.GetName(), mode)
		}
	}

	supportedCAs := types.AppSupportedInternalCAs()
	for _, allowedCA := range tls.AllowedCas {
		if slices.Contains(supportedCAs, allowedCA) {
			continue
		}
		if err := isValidCACertificatePEM(allowedCA); err != nil {
			return trace.BadParameter(
				"App %q 'tls.allowed_cas' values must include valid PEM-encoded CA certificates or a Teleport CA alias (%s): %s",
				a.GetName(),
				quoteAndJoin(supportedCAs),
				err,
			)
		}
	}

	return nil
}

// MarshalApp marshals Application resource to JSON.
func MarshalApp(app types.Application, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch app := app.(type) {
	case *types.AppV3:
		if err := app.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, app))
	default:
		return nil, trace.BadParameter("unsupported app resource %T", app)
	}
}

// UnmarshalApp unmarshals Application resource from JSON.
func UnmarshalApp(data []byte, opts ...MarshalOption) (types.Application, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var app types.AppV3
		if err := utils.FastUnmarshal(data, &app); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			app.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			app.SetExpiry(cfg.Expires)
		}
		return &app, nil
	}
	return nil, trace.BadParameter("unsupported app resource version %q", h.Version)
}

// MarshalAppServer marshals the AppServer resource to JSON.
func MarshalAppServer(appServer types.AppServer, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch appServer := appServer.(type) {
	case *types.AppServerV3:
		if err := appServer.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, appServer))
	default:
		return nil, trace.BadParameter("unsupported app server resource %T", appServer)
	}
}

// UnmarshalAppServer unmarshals AppServer resource from JSON.
func UnmarshalAppServer(data []byte, opts ...MarshalOption) (types.AppServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.AppServerV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported app server resource version %q", h.Version)
}

// NewApplicationFromKubeService creates application resources from kubernetes service.
// It transforms service fields and annotations into appropriate Teleport app fields.
// Service labels are copied to app labels.
func NewApplicationFromKubeService(service corev1.Service, clusterName, protocol string, port corev1.ServicePort) (types.Application, error) {
	appURI := buildAppURI(protocol, GetServiceFQDN(service), service.GetAnnotations()[types.DiscoveryPathLabel], port.Port)

	rewriteConfig, err := getAppRewriteConfig(service.GetAnnotations())
	if err != nil {
		return nil, trace.Wrap(err, "could not get app rewrite config for the service")
	}

	appNameAnnotation := service.GetAnnotations()[types.DiscoveryAppNameLabel]
	appName, err := getAppName(service.GetName(), service.GetNamespace(), clusterName, port.Name, appNameAnnotation)
	if err != nil {
		return nil, trace.Wrap(err, "could not create app name for the service")
	}

	labels, err := getAppLabels(service.GetLabels(), clusterName)
	if err != nil {
		return nil, trace.Wrap(err, "could not get labels for the service")
	}

	app, err := types.NewAppV3(types.Metadata{
		Name: appName,
		Description: cmp.Or(
			getDescription(service.GetAnnotations()),
			fmt.Sprintf("Discovered application in Kubernetes cluster %q", clusterName),
		),
		Labels: labels,
	}, types.AppSpecV3{
		URI:                appURI,
		Rewrite:            rewriteConfig,
		InsecureSkipVerify: getTLSInsecureSkipVerify(service.GetAnnotations()),
		PublicAddr:         getPublicAddr(service.GetAnnotations()),
	})
	if err != nil {
		return nil, trace.Wrap(err, "could not create an app from Kubernetes service")
	}

	return app, nil
}

// GetServiceFQDN returns the fully qualified domain name for the service.
func GetServiceFQDN(service corev1.Service) string {
	// If service type is ExternalName it points to external DNS name, to keep correct
	// HOST for HTTP requests we return already final external DNS name.
	// https://kubernetes.io/docs/concepts/services-networking/service/#externalname
	if service.Spec.Type == corev1.ServiceTypeExternalName {
		return service.Spec.ExternalName
	}
	return fmt.Sprintf("%s.%s.svc.%s", service.GetName(), service.GetNamespace(), clusterDomainResolver())
}

func buildAppURI(protocol, serviceFQDN, path string, port int32) string {
	return (&url.URL{
		Scheme: protocol,
		Host:   net.JoinHostPort(serviceFQDN, strconv.Itoa(int(port))),
		Path:   path,
	}).String()
}

func getAppRewriteConfig(annotations map[string]string) (*types.Rewrite, error) {
	rewritePayload := annotations[types.DiscoveryAppRewriteLabel]
	if rewritePayload == "" {
		return nil, nil
	}

	rw := types.Rewrite{}
	reader := strings.NewReader(rewritePayload)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&rw)
	if err != nil {
		return nil, trace.Wrap(err, "failed decoding rewrite config")
	}

	return &rw, nil
}

func getDescription(annotations map[string]string) string {
	return annotations[types.DiscoveryDescription]
}

func getPublicAddr(annotations map[string]string) string {
	return annotations[types.DiscoveryPublicAddr]
}

func getTLSInsecureSkipVerify(annotations map[string]string) bool {
	val := annotations[types.DiscoveryAppInsecureSkipVerify]
	if val == "" {
		return false
	}
	return val == "true"
}

func getAppName(serviceName, namespace, clusterName, portName, nameAnnotation string) (string, error) {
	if nameAnnotation != "" {
		name := nameAnnotation
		if portName != "" {
			name = fmt.Sprintf("%s-%s", name, portName)
		}

		if len(validation.IsDNS1035Label(name)) > 0 {
			return "", trace.BadParameter(
				"application name %q must be a lower case valid DNS subdomain: https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#application-name", name)
		}

		return name, nil
	}

	clusterName = strings.ReplaceAll(clusterName, ".", "-")
	if portName != "" {
		return fmt.Sprintf("%s-%s-%s-%s", serviceName, portName, namespace, clusterName), nil
	}
	return fmt.Sprintf("%s-%s-%s", serviceName, namespace, clusterName), nil
}

func getAppLabels(serviceLabels map[string]string, clusterName string) (map[string]string, error) {
	result := make(map[string]string, len(serviceLabels)+1)

	for k, v := range serviceLabels {
		if !types.IsValidLabelKey(k) {
			return nil, trace.BadParameter("invalid label key: %q", k)
		}

		result[k] = v
	}
	result[types.KubernetesClusterLabel] = clusterName

	return result, nil
}

var (
	// clusterDomainResolver is a function that resolves the cluster domain once and caches the result.
	// It's used to lazily resolve the cluster domain from the env var "TELEPORT_KUBE_CLUSTER_DOMAIN" or fallback to
	// a default value.
	// It's only used when agent is running in the Kubernetes cluster.
	clusterDomainResolver = sync.OnceValue[string](getClusterDomain)
)

const (
	// teleportKubeClusterDomain is the environment variable that specifies the cluster domain.
	teleportKubeClusterDomain = "TELEPORT_KUBE_CLUSTER_DOMAIN"
)

func getClusterDomain() string {
	if envDomain := os.Getenv(teleportKubeClusterDomain); envDomain != "" {
		return envDomain
	}
	return "cluster.local"
}

// RewriteHeadersAndApplyValueTraits rewrites the provided request's headers
// while applying value traits to them.
func RewriteHeadersAndApplyValueTraits(r *http.Request, rewrites iter.Seq[*types.Header], rewriteTraits wrappers.Traits, log *slog.Logger) {
	for header := range rewrites {
		values, err := ApplyValueTraits(header.Value, rewriteTraits)
		if err != nil {
			log.DebugContext(r.Context(), "Failed to apply traits",
				"header_value", header.Value,
				"error", err,
			)
			continue
		}
		r.Header.Del(header.Name)
		for _, value := range values {
			switch http.CanonicalHeaderKey(header.Name) {
			case teleport.HostHeader:
				r.Host = value
			default:
				r.Header.Add(header.Name, value)
			}
		}
	}
}

// isValidSpiffeID validates that s contains a valid SPIFFE ID.
func isValidSpiffeID(s string) error {
	_, err := spiffeid.FromString(s)
	return err
}

// isValidCACertificatePEM validates that s contains valid PEM-encoded CA
// certificate.
func isValidCACertificatePEM(s string) error {
	cert, err := tlsutils.ParseCertificatePEMStrict([]byte(s))
	if err != nil {
		return trace.Wrap(err)
	}

	switch {
	case !cert.BasicConstraintsValid || !cert.IsCA:
		return trace.BadParameter("certificate %q is not a CA", cert.Subject.String())
	case cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageCertSign == 0:
		return trace.BadParameter("CA certificate %q does not allow certificate signing", cert.Subject.String())
	}

	return nil
}

// quoteAndJoin takes a slice of strings and returns them quoted and
// comma-separated.
func quoteAndJoin(items []string) string {
	if len(items) == 0 {
		return ""
	}
	quotedItems := make([]string, len(items))
	for i, item := range items {
		quotedItems[i] = `"` + item + `"`
	}
	return strings.Join(quotedItems, ", ")
}
