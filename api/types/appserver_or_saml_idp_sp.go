/*
Copyright 2023 Gravitational, Inc.

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
	"sort"
	"time"

	"github.com/gravitational/trace"
)

// AppServerOrSAMLIdPServiceProvider describes methods shared between an AppServer and a SAMLIdpServiceProvider resource.
//
// DEPRECATED: Use AppServer and SAMLIdPServiceProvider types individually.
type AppServerOrSAMLIdPServiceProvider interface {
	ResourceWithLabels
	GetAppServer() *AppServerV3
	GetSAMLIdPServiceProvider() *SAMLIdPServiceProviderV1
	GetName() string
	GetDescription() string
	GetPublicAddr() string
	IsAppServer() bool
}

// staticSAMLIdPServiceProviderDescription is a static description for SAMLIdpServiceProvider resources since the resource itself does not supply its own description.
const staticSAMLIdPServiceProviderDescription = "SAML Application"

// GetKind returns the kind that this AppServerOrSAMLIdPServiceProvider object represents, either KindAppServer or KindSAMLIdPServiceProvider.
func (a *AppServerOrSAMLIdPServiceProviderV1) GetKind() string {
	if a.IsAppServer() {
		return KindAppServer
	}
	return KindSAMLIdPServiceProvider
}

// GetDescription returns the description of either the App or the SAMLIdPServiceProvider.
func (a *AppServerOrSAMLIdPServiceProviderV1) GetDescription() string {
	if a.IsAppServer() {
		return a.GetAppServer().GetApp().GetDescription()
	}
	return staticSAMLIdPServiceProviderDescription
}

// GetDescription returns the public address of either the App or the SAMLIdPServiceProvider.
func (a *AppServerOrSAMLIdPServiceProviderV1) GetPublicAddr() string {
	if a.IsAppServer() {
		return a.GetAppServer().GetApp().GetPublicAddr()
	}
	// SAMLIdPServiceProviders don't have a PublicAddr
	return ""
}

// IsAppServer returns true if this AppServerOrSAMLIdPServiceProviderV1 represents an AppServer.
func (a *AppServerOrSAMLIdPServiceProviderV1) IsAppServer() bool {
	appOrSP := a.Resource
	_, ok := appOrSP.(*AppServerOrSAMLIdPServiceProviderV1_AppServer)
	return ok
}

// AppServersOrSAMLIdPServiceProviders is a list of AppServers and SAMLIdPServiceProviders.
type AppServersOrSAMLIdPServiceProviders []AppServerOrSAMLIdPServiceProvider

func (s AppServersOrSAMLIdPServiceProviders) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, 0, len(s))
	for _, app := range s {
		if app.IsAppServer() {
			resources = append(resources, ResourceWithLabels(app.GetAppServer()))
		} else {
			resources = append(resources, ResourceWithLabels(app.GetSAMLIdPServiceProvider()))
		}
	}
	return resources
}

// SortByCustom custom sorts by given sort criteria.
func (s AppServersOrSAMLIdPServiceProviders) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetName(), s[j].GetName(), isDesc)
		})
	case ResourceSpecDescription:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetDescription(), s[j].GetDescription(), isDesc)
		})
	case ResourceSpecPublicAddr:
		sort.SliceStable(s, func(i, j int) bool {
			return stringCompare(s[i].GetPublicAddr(), s[j].GetPublicAddr(), isDesc)
		})
	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindAppOrSAMLIdPServiceProvider)
	}

	return nil
}

// GetFieldVals returns list of select field values.
func (s AppServersOrSAMLIdPServiceProviders) GetFieldVals(field string) ([]string, error) {
	vals := make([]string, 0, len(s))
	switch field {
	case ResourceMetadataName:
		for _, appOrSP := range s {
			vals = append(vals, appOrSP.GetName())
		}
	case ResourceSpecDescription:
		for _, appOrSP := range s {
			vals = append(vals, appOrSP.GetDescription())
		}
	case ResourceSpecPublicAddr:
		for _, appOrSP := range s {
			vals = append(vals, appOrSP.GetPublicAddr())
		}
	default:
		return nil, trace.NotImplemented("getting field %q for resource %q is not supported", field, KindAppServer)
	}

	return vals, nil
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (a *AppServerOrSAMLIdPServiceProviderV1) CheckAndSetDefaults() error {
	if a.IsAppServer() {
		if err := a.GetAppServer().CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err := a.GetSAMLIdPServiceProvider().CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *AppServerOrSAMLIdPServiceProviderV1) Expiry() time.Time {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Metadata.Expiry()
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata.Expiry()
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetAllLabels() map[string]string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		staticLabels := make(map[string]string)
		for name, value := range appServer.Metadata.Labels {
			staticLabels[name] = value
		}

		var dynamicLabels map[string]CommandLabelV2
		if appServer.Spec.App != nil {
			for name, value := range appServer.Spec.App.Metadata.Labels {
				staticLabels[name] = value
			}

			dynamicLabels = appServer.Spec.App.Spec.DynamicLabels
		}

		return CombineLabels(staticLabels, dynamicLabels)
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata.Labels
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetLabel(key string) (value string, ok bool) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		if cmd, ok := appServer.Spec.App.Spec.DynamicLabels[key]; ok {
			return cmd.Result, ok
		}

		v, ok := appServer.Spec.App.Metadata.Labels[key]
		return v, ok
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		v, ok := sp.Metadata.Labels[key]
		return v, ok
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetMetadata() Metadata {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Metadata
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata
	}
}

// GetDescription returns the name of either the App or the SAMLIdPServiceProvider.
func (a *AppServerOrSAMLIdPServiceProviderV1) GetName() string {
	if a.IsAppServer() {
		return a.GetAppServer().GetApp().GetName()
	}
	return a.GetSAMLIdPServiceProvider().GetName()
}

func (a *AppServerOrSAMLIdPServiceProviderV1) SetName(name string) {
	if a.IsAppServer() {
		a.GetAppServer().GetApp().SetName(name)
	}
	a.GetSAMLIdPServiceProvider().SetName(name)
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetResourceID() int64 {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Metadata.ID
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata.ID
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) SetResourceID(id int64) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.Metadata.ID = id
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		sp.Metadata.ID = id
	}
}

// GetRevision returns the revision
func (a *AppServerOrSAMLIdPServiceProviderV1) GetRevision() string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.GetRevision()
	}

	sp := a.GetSAMLIdPServiceProvider()
	return sp.GetRevision()
}

// SetRevision sets the revision
func (a *AppServerOrSAMLIdPServiceProviderV1) SetRevision(rev string) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.SetRevision(rev)
		return
	}

	sp := a.GetSAMLIdPServiceProvider()
	sp.SetRevision(rev)
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetStaticLabels() map[string]string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Metadata.Labels
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata.Labels
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) SetStaticLabels(sl map[string]string) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.Metadata.Labels = sl
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		sp.Metadata.Labels = sl
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetSubKind() string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.SubKind
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.SubKind
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) SetSubKind(sk string) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.SubKind = sk
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		sp.SubKind = sk
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) GetVersion() string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Version
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Version
	}
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (a *AppServerOrSAMLIdPServiceProviderV1) MatchSearch(values []string) bool {
	return MatchSearch(nil, values, nil)
}

func (a *AppServerOrSAMLIdPServiceProviderV1) Origin() string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return appServer.Metadata.Origin()
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return sp.Metadata.Origin()
	}
}

// SetOrigin sets the origin value of the resource.
func (a *AppServerOrSAMLIdPServiceProviderV1) SetOrigin(origin string) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.Metadata.SetOrigin(origin)
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		sp.Metadata.SetOrigin(origin)
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) SetExpiry(expiry time.Time) {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		appServer.Metadata.SetExpiry(expiry)
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		sp.Metadata.SetExpiry(expiry)
	}
}

func (a *AppServerOrSAMLIdPServiceProviderV1) String() string {
	if a.IsAppServer() {
		appServer := a.GetAppServer()
		return fmt.Sprintf("AppServer(Name=%v, Version=%v, Hostname=%v, HostID=%v, App=%v)",
			appServer.GetName(), appServer.GetVersion(), appServer.GetHostname(), appServer.GetHostID(), appServer.GetApp())
	} else {
		sp := a.GetSAMLIdPServiceProvider()
		return fmt.Sprintf("SAMLIdPServiceProvider(Name=%v, Version=%v, EntityID=%v)",
			sp.GetName(), sp.GetVersion(), sp.GetEntityID())
	}
}
