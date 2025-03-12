/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
	netutils "github.com/gravitational/teleport/api/utils/net"
)

var _ compare.IsEqual[Application] = (*AppV3)(nil)

// Application represents a web, TCP or cloud console application.
type Application interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the app namespace.
	GetNamespace() string
	// GetStaticLabels returns the app static labels.
	GetStaticLabels() map[string]string
	// SetStaticLabels sets the app static labels.
	SetStaticLabels(map[string]string)
	// GetDynamicLabels returns the app dynamic labels.
	GetDynamicLabels() map[string]CommandLabel
	// SetDynamicLabels sets the app dynamic labels.
	SetDynamicLabels(map[string]CommandLabel)
	// String returns string representation of the app.
	String() string
	// GetDescription returns the app description.
	GetDescription() string
	// GetURI returns the app connection endpoint.
	GetURI() string
	// SetURI sets the app endpoint.
	SetURI(string)
	// GetPublicAddr returns the app public address.
	GetPublicAddr() string
	// GetInsecureSkipVerify returns the app insecure setting.
	GetInsecureSkipVerify() bool
	// GetRewrite returns the app rewrite configuration.
	GetRewrite() *Rewrite
	// IsAWSConsole returns true if this app is AWS management console.
	IsAWSConsole() bool
	// IsAzureCloud returns true if this app represents Azure Cloud instance.
	IsAzureCloud() bool
	// IsGCP returns true if this app represents GCP instance.
	IsGCP() bool
	// IsTCP returns true if this app represents a TCP endpoint.
	IsTCP() bool
	// GetProtocol returns the application protocol.
	GetProtocol() string
	// GetAWSAccountID returns value of label containing AWS account ID on this app.
	GetAWSAccountID() string
	// GetAWSExternalID returns the AWS External ID configured for this app.
	GetAWSExternalID() string
	// GetUserGroups will get the list of user group IDs associated with the application.
	GetUserGroups() []string
	// SetUserGroups will set the list of user group IDs associated with the application.
	SetUserGroups([]string)
	// Copy returns a copy of this app resource.
	Copy() *AppV3
	// GetIntegration will return the Integration.
	// If present, the Application must use the Integration's credentials instead of ambient credentials to access Cloud APIs.
	GetIntegration() string
	// GetRequiredAppNames will return a list of required apps names that should be authenticated during this apps authentication process.
	GetRequiredAppNames() []string
	// GetCORS returns the CORS configuration for the app.
	GetCORS() *CORSPolicy
	// GetTCPPorts returns port ranges supported by the app to which connections can be forwarded to.
	GetTCPPorts() PortRanges
	// SetTCPPorts sets port ranges to which connections can be forwarded to.
	SetTCPPorts([]*PortRange)
	// GetIdentityCenter fetches identity center info for the app, if any.
	GetIdentityCenter() *AppIdentityCenter
}

// NewAppV3 creates a new app resource.
func NewAppV3(meta Metadata, spec AppSpecV3) (*AppV3, error) {
	app := &AppV3{
		Metadata: meta,
		Spec:     spec,
	}
	if err := app.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// GetVersion returns the app resource version.
func (a *AppV3) GetVersion() string {
	return a.Version
}

// GetKind returns the app resource kind.
func (a *AppV3) GetKind() string {
	return a.Kind
}

// GetSubKind returns the app resource subkind.
func (a *AppV3) GetSubKind() string {
	return a.SubKind
}

// SetSubKind sets the app resource subkind.
func (a *AppV3) SetSubKind(sk string) {
	a.SubKind = sk
}

// GetRevision returns the revision
func (a *AppV3) GetRevision() string {
	return a.Metadata.GetRevision()
}

// SetRevision sets the revision
func (a *AppV3) SetRevision(rev string) {
	a.Metadata.SetRevision(rev)
}

// GetMetadata returns the app resource metadata.
func (a *AppV3) GetMetadata() Metadata {
	return a.Metadata
}

// Origin returns the origin value of the resource.
func (a *AppV3) Origin() string {
	return a.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (a *AppV3) SetOrigin(origin string) {
	a.Metadata.SetOrigin(origin)
}

// GetNamespace returns the app resource namespace.
func (a *AppV3) GetNamespace() string {
	return a.Metadata.Namespace
}

// SetExpiry sets the app resource expiration time.
func (a *AppV3) SetExpiry(expiry time.Time) {
	a.Metadata.SetExpiry(expiry)
}

// Expiry returns the app resource expiration time.
func (a *AppV3) Expiry() time.Time {
	return a.Metadata.Expiry()
}

// GetName returns the app resource name.
func (a *AppV3) GetName() string {
	return a.Metadata.Name
}

// SetName sets the app resource name.
func (a *AppV3) SetName(name string) {
	a.Metadata.Name = name
}

// GetStaticLabels returns the app static labels.
func (a *AppV3) GetStaticLabels() map[string]string {
	return a.Metadata.Labels
}

// SetStaticLabels sets the app static labels.
func (a *AppV3) SetStaticLabels(sl map[string]string) {
	a.Metadata.Labels = sl
}

// GetDynamicLabels returns the app dynamic labels.
func (a *AppV3) GetDynamicLabels() map[string]CommandLabel {
	if a.Spec.DynamicLabels == nil {
		return nil
	}
	return V2ToLabels(a.Spec.DynamicLabels)
}

// SetDynamicLabels sets the app dynamic labels
func (a *AppV3) SetDynamicLabels(dl map[string]CommandLabel) {
	a.Spec.DynamicLabels = LabelsToV2(dl)
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (a *AppV3) GetLabel(key string) (value string, ok bool) {
	if cmd, ok := a.Spec.DynamicLabels[key]; ok {
		return cmd.Result, ok
	}

	v, ok := a.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns the app combined static and dynamic labels.
func (a *AppV3) GetAllLabels() map[string]string {
	return CombineLabels(a.Metadata.Labels, a.Spec.DynamicLabels)
}

// GetDescription returns the app description.
func (a *AppV3) GetDescription() string {
	return a.Metadata.Description
}

// GetURI returns the app connection address.
func (a *AppV3) GetURI() string {
	return a.Spec.URI
}

// SetURI sets the app connection address.
func (a *AppV3) SetURI(uri string) {
	a.Spec.URI = uri
}

// GetPublicAddr returns the app public address.
func (a *AppV3) GetPublicAddr() string {
	return a.Spec.PublicAddr
}

// GetInsecureSkipVerify returns the app insecure setting.
func (a *AppV3) GetInsecureSkipVerify() bool {
	return a.Spec.InsecureSkipVerify
}

// GetRewrite returns the app rewrite configuration.
func (a *AppV3) GetRewrite() *Rewrite {
	return a.Spec.Rewrite
}

// IsAWSConsole returns true if this app is AWS management console.
func (a *AppV3) IsAWSConsole() bool {
	// TODO(greedy52) support region based console URL like:
	// https://us-east-1.console.aws.amazon.com/
	for _, consoleURL := range []string{
		constants.AWSConsoleURL,
		constants.AWSUSGovConsoleURL,
		constants.AWSCNConsoleURL,
	} {
		if strings.HasPrefix(a.Spec.URI, consoleURL) {
			return true
		}
	}

	return a.Spec.Cloud == CloudAWS
}

// IsAzureCloud returns true if this app is Azure Cloud instance.
func (a *AppV3) IsAzureCloud() bool {
	return a.Spec.Cloud == CloudAzure
}

// IsGCP returns true if this app is GCP instance.
func (a *AppV3) IsGCP() bool {
	return a.Spec.Cloud == CloudGCP
}

// IsTCP returns true if this app represents a TCP endpoint.
func (a *AppV3) IsTCP() bool {
	return IsAppTCP(a.Spec.URI)
}

func IsAppTCP(uri string) bool {
	return strings.HasPrefix(uri, "tcp://")
}

// GetProtocol returns the application protocol.
func (a *AppV3) GetProtocol() string {
	if a.IsTCP() {
		return "TCP"
	}
	return "HTTP"
}

// GetAWSAccountID returns value of label containing AWS account ID on this app.
func (a *AppV3) GetAWSAccountID() string {
	return a.Metadata.Labels[constants.AWSAccountIDLabel]
}

// GetAWSExternalID returns the AWS External ID configured for this app.
func (a *AppV3) GetAWSExternalID() string {
	if a.Spec.AWS == nil {
		return ""
	}
	return a.Spec.AWS.ExternalID
}

// GetUserGroups will get the list of user group IDss associated with the application.
func (a *AppV3) GetUserGroups() []string {
	return a.Spec.UserGroups
}

// SetUserGroups will set the list of user group IDs associated with the application.
func (a *AppV3) SetUserGroups(userGroups []string) {
	a.Spec.UserGroups = userGroups
}

// GetTCPPorts returns port ranges supported by the app to which connections can be forwarded to.
func (a *AppV3) GetTCPPorts() PortRanges {
	return a.Spec.TCPPorts
}

// SetTCPPorts sets port ranges to which connections can be forwarded to.
func (a *AppV3) SetTCPPorts(ports []*PortRange) {
	a.Spec.TCPPorts = ports
}

// GetIntegration will return the Integration.
// If present, the Application must use the Integration's credentials instead of ambient credentials to access Cloud APIs.
func (a *AppV3) GetIntegration() string {
	return a.Spec.Integration
}

// String returns the app string representation.
func (a *AppV3) String() string {
	return fmt.Sprintf("App(Name=%v, PublicAddr=%v, Labels=%v)",
		a.GetName(), a.GetPublicAddr(), a.GetAllLabels())
}

// Copy returns a copy of this database resource.
func (a *AppV3) Copy() *AppV3 {
	return utils.CloneProtoMsg(a)
}

func (a *AppV3) GetRequiredAppNames() []string {
	return a.Spec.RequiredAppNames
}

func (a *AppV3) GetCORS() *CORSPolicy {
	return a.Spec.CORS
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (a *AppV3) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName(), a.GetDescription(), a.GetPublicAddr())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (a *AppV3) setStaticFields() {
	a.Kind = KindApp
	a.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (a *AppV3) CheckAndSetDefaults() error {
	a.setStaticFields()
	if err := a.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for key := range a.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("app %q invalid label key: %q", a.GetName(), key)
		}
	}
	if a.Spec.URI == "" {
		if a.Spec.Cloud != "" {
			a.Spec.URI = fmt.Sprintf("cloud://%v", a.Spec.Cloud)
		} else {
			return trace.BadParameter("app %q URI is empty", a.GetName())
		}
	}
	if a.Spec.Cloud == "" && a.IsAWSConsole() {
		a.Spec.Cloud = CloudAWS
	}
	switch a.Spec.Cloud {
	case "", CloudAWS, CloudAzure, CloudGCP:
		break
	default:
		return trace.BadParameter("app %q has unexpected Cloud value %q", a.GetName(), a.Spec.Cloud)
	}
	publicAddr := a.Spec.PublicAddr
	// If the public addr has digits in a sub-host and a port, it might cause url.Parse to fail.
	// Eg of a failing url: 123.teleport.example.com:3080
	// This is not a valid URL, but we have been using it as such.
	// To prevent this from failing, we add the `//`.
	if !strings.Contains(publicAddr, "//") && strings.Contains(publicAddr, ":") {
		publicAddr = "//" + publicAddr
	}
	publicAddrURL, err := url.Parse(publicAddr)
	if err != nil {
		return trace.BadParameter("invalid PublicAddr format: %v", err)
	}
	host := a.Spec.PublicAddr
	if publicAddrURL.Host != "" {
		host = publicAddrURL.Host
	}

	if strings.HasPrefix(host, constants.KubeTeleportProxyALPNPrefix) {
		return trace.BadParameter("app %q DNS prefix found in %q public_url is reserved for internal usage",
			constants.KubeTeleportProxyALPNPrefix, a.Spec.PublicAddr)
	}

	if a.Spec.Rewrite != nil {
		switch a.Spec.Rewrite.JWTClaims {
		case "", JWTClaimsRewriteRolesAndTraits, JWTClaimsRewriteRoles, JWTClaimsRewriteNone, JWTClaimsRewriteTraits:
		default:
			return trace.BadParameter("app %q has unexpected JWT rewrite value %q", a.GetName(), a.Spec.Rewrite.JWTClaims)

		}
	}

	if len(a.Spec.TCPPorts) != 0 {
		if err := a.checkTCPPorts(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *AppV3) checkTCPPorts() error {
	// Parsing the URI here does not break compatibility. The URI is parsed only if Ports are present.
	// This means that old apps that do have invalid URIs but don't use Ports can continue existing.
	uri, err := url.Parse(a.Spec.URI)
	if err != nil {
		return trace.BadParameter("invalid app URI format: %v", err)
	}

	// The scheme of URI is enforced to be "tcp" on purpose. This way in the future we can add
	// multi-port support to web apps without throwing hard errors when a cluster with a multi-port
	// web app gets downgraded to a version which supports multi-port only for TCP apps.
	//
	// For now, we simply ignore the Ports field set on non-TCP apps.
	if uri.Scheme != "tcp" {
		return nil
	}

	if uri.Port() != "" {
		return trace.BadParameter("TCP app URI %q must not include a port number when the app spec defines a list of ports", a.Spec.URI)
	}

	for _, portRange := range a.Spec.TCPPorts {
		if err := netutils.ValidatePortRange(int(portRange.Port), int(portRange.EndPort)); err != nil {
			return trace.Wrap(err, "validating a port range of a TCP app")
		}
	}

	return nil
}

// GetIdentityCenter returns the Identity Center information for the app, if any.
// May be nil.
func (a *AppV3) GetIdentityCenter() *AppIdentityCenter {
	return a.Spec.IdentityCenter
}

// GetDisplayName fetches a human-readable display name for the App.
func (a *AppV3) GetDisplayName() string {
	// Only Identity Center apps have a display name at this point. Returning
	// the empty string signals to the caller they should fall back to whatever
	// they have been using in the past.
	if a.Spec.IdentityCenter == nil {
		return ""
	}
	return a.Metadata.Description
}

// IsEqual determines if two application resources are equivalent to one another.
func (a *AppV3) IsEqual(i Application) bool {
	if other, ok := i.(*AppV3); ok {
		return deriveTeleportEqualAppV3(a, other)
	}
	return false
}

// DeduplicateApps deduplicates apps by combination of app name and public address.
// Apps can have the same name but also could have different addresses.
func DeduplicateApps(apps []Application) (result []Application) {
	type key struct{ name, addr string }
	seen := make(map[key]struct{})
	for _, app := range apps {
		key := key{app.GetName(), app.GetPublicAddr()}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, app)
	}
	return result
}

// Apps is a list of app resources.
type Apps []Application

// Find returns app with the specified name or nil.
func (a Apps) Find(name string) Application {
	for _, app := range a {
		if app.GetName() == name {
			return app
		}
	}
	return nil
}

// AsResources returns these apps as resources with labels.
func (a Apps) AsResources() (resources ResourcesWithLabels) {
	for _, app := range a {
		resources = append(resources, app)
	}
	return resources
}

// Len returns the slice length.
func (a Apps) Len() int { return len(a) }

// Less compares apps by name.
func (a Apps) Less(i, j int) bool { return a[i].GetName() < a[j].GetName() }

// Swap swaps two apps.
func (a Apps) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// GetPermissionSets fetches the list of permission sets from the Identity Center
// app information. Handles nil identity center values.
func (a *AppIdentityCenter) GetPermissionSets() []*IdentityCenterPermissionSet {
	if a == nil {
		return nil
	}
	return a.PermissionSets
}

// PortRanges is a list of port ranges.
type PortRanges []*PortRange

// Contains checks if targetPort is within any of the port ranges.
func (p PortRanges) Contains(targetPort int) bool {
	return slices.ContainsFunc(p, func(portRange *PortRange) bool {
		return netutils.IsPortInRange(int(portRange.Port), int(portRange.EndPort), targetPort)
	})
}

// String returns a string representation of port ranges.
func (p PortRanges) String() string {
	var builder strings.Builder
	for i, portRange := range p {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(portRange.String())
	}
	return builder.String()
}

// String returns a string representation of a port range.
func (p *PortRange) String() string {
	if p.EndPort == 0 {
		return strconv.Itoa(int(p.Port))
	} else {
		return fmt.Sprintf("%d-%d", p.Port, p.EndPort)
	}
}
