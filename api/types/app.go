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
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
)

// Application represents a web app.
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
	// LabelsString returns all labels as a string.
	LabelsString() string
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
	// GetAWSAccountID returns value of label containing AWS account ID on this app.
	GetAWSAccountID() string
	// Copy returns a copy of this app resource.
	Copy() *AppV3
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

// NewAppV3FromLegacyApp creates a new app resource from legacy app struct.
//
// DELETE IN 9.0.
func NewAppV3FromLegacyApp(app *App) (*AppV3, error) {
	return NewAppV3(Metadata{
		Name:        app.Name,
		Description: app.Description,
		Labels:      app.StaticLabels,
	}, AppSpecV3{
		URI:                app.URI,
		PublicAddr:         app.PublicAddr,
		DynamicLabels:      app.DynamicLabels,
		InsecureSkipVerify: app.InsecureSkipVerify,
		Rewrite:            app.Rewrite,
	})
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

// GetResourceID returns the app resource ID.
func (a *AppV3) GetResourceID() int64 {
	return a.Metadata.ID
}

// SetResourceID sets the app resource ID.
func (a *AppV3) SetResourceID(id int64) {
	a.Metadata.ID = id
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

// GetAllLabels returns the app combined static and dynamic labels.
func (a *AppV3) GetAllLabels() map[string]string {
	return CombineLabels(a.Metadata.Labels, a.Spec.DynamicLabels)
}

// LabelsString returns all app labels as a string.
func (a *AppV3) LabelsString() string {
	return LabelsAsString(a.Metadata.Labels, a.Spec.DynamicLabels)
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
	return strings.HasPrefix(a.Spec.URI, constants.AWSConsoleURL)
}

// GetAWSAccountID returns value of label containing AWS account ID on this app.
func (a *AppV3) GetAWSAccountID() string {
	return a.Metadata.Labels[constants.AWSAccountIDLabel]
}

// String returns the app string representation.
func (a *AppV3) String() string {
	return fmt.Sprintf("App(Name=%v, PublicAddr=%v, Labels=%v)",
		a.GetName(), a.GetPublicAddr(), a.GetAllLabels())
}

// Copy returns a copy of this database resource.
func (a *AppV3) Copy() *AppV3 {
	return proto.Clone(a).(*AppV3)
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
		return trace.BadParameter("app %q URI is empty", a.GetName())
	}

	url, err := url.Parse(a.Spec.PublicAddr)
	if err != nil {
		return trace.BadParameter("invalid PublicAddr format: %v", err)
	}
	host := a.Spec.PublicAddr
	if url.Host != "" {
		host = url.Host
	}

	// DEPRECATED DELETE IN 11.0 use KubeTeleportProxyALPNPrefix check only.
	if strings.HasPrefix(host, constants.KubeSNIPrefix) {
		return trace.BadParameter("app %q DNS prefix found in %q public_url is reserved for internal usage",
			constants.KubeSNIPrefix, a.Spec.PublicAddr)
	}

	if strings.HasPrefix(host, constants.KubeTeleportProxyALPNPrefix) {
		return trace.BadParameter("app %q DNS prefix found in %q public_url is reserved for internal usage",
			constants.KubeTeleportProxyALPNPrefix, a.Spec.PublicAddr)
	}

	return nil
}

// DeduplicateApps deduplicates apps by name.
func DeduplicateApps(apps []Application) (result []Application) {
	seen := make(map[string]struct{})
	for _, app := range apps {
		if _, ok := seen[app.GetName()]; ok {
			continue
		}
		seen[app.GetName()] = struct{}{}
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
