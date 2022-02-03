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
	"time"

	"github.com/gravitational/teleport/api"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

// AppServer represents a single proxied web app.
type AppServer interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns server namespace.
	GetNamespace() string
	// GetTeleportVersion returns the teleport version the server is running on.
	GetTeleportVersion() string
	// GetHostname returns the server hostname.
	GetHostname() string
	// GetHostID returns ID of the host the server is running on.
	GetHostID() string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() Rotation
	// SetRotation sets the state of certificate authority rotation.
	SetRotation(Rotation)
	// String returns string representation of the server.
	String() string
	// Copy returns a copy of this app server object.
	Copy() AppServer
	// GetApp returns the app this app server proxies.
	GetApp() Application
	// SetApp sets the app this app server proxies.
	SetApp(Application) error
}

// NewAppServerV3 creates a new app server instance.
func NewAppServerV3(meta Metadata, spec AppServerSpecV3) (*AppServerV3, error) {
	s := &AppServerV3{
		Metadata: meta,
		Spec:     spec,
	}
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

// NewAppServerV3FromApp creates a new app server from the provided app.
func NewAppServerV3FromApp(app *AppV3, hostname, hostID string) (*AppServerV3, error) {
	return NewAppServerV3(Metadata{
		Name: app.GetName(),
	}, AppServerSpecV3{
		Hostname: hostname,
		HostID:   hostID,
		App:      app,
	})
}

// NewLegacyAppServer creates legacy app server object. Used in tests.
//
// DELETE IN 9.0.
func NewLegacyAppServer(app *AppV3, hostname, hostID string) (Server, error) {
	return NewServer(hostID, KindAppServer,
		ServerSpecV2{
			Hostname: hostname,
			Apps: []*App{
				{
					Name:         app.GetName(),
					URI:          app.GetURI(),
					PublicAddr:   app.GetPublicAddr(),
					StaticLabels: app.GetStaticLabels(),
				},
			},
		})
}

// NewAppServersV3FromServer creates a list of app servers from Server resource.
//
// DELETE IN 9.0.
func NewAppServersV3FromServer(server Server) (result []AppServer, err error) {
	for _, legacyApp := range server.GetApps() {
		app, err := NewAppV3FromLegacyApp(legacyApp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		appServer, err := NewAppServerV3(Metadata{
			Name:    app.GetName(),
			Expires: server.GetMetadata().Expires,
		}, AppServerSpecV3{
			Version:  server.GetTeleportVersion(),
			Hostname: server.GetHostname(),
			HostID:   server.GetName(),
			Rotation: server.GetRotation(),
			App:      app,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, appServer)
	}
	return result, nil
}

// GetVersion returns the database server resource version.
func (s *AppServerV3) GetVersion() string {
	return s.Version
}

// GetTeleportVersion returns the Teleport version the server is running.
func (s *AppServerV3) GetTeleportVersion() string {
	return s.Spec.Version
}

// GetHostname returns the database server hostname.
func (s *AppServerV3) GetHostname() string {
	return s.Spec.Hostname
}

// GetHostID returns ID of the host the server is running on.
func (s *AppServerV3) GetHostID() string {
	return s.Spec.HostID
}

// GetKind returns the resource kind.
func (s *AppServerV3) GetKind() string {
	return s.Kind
}

// GetSubKind returns the resource subkind.
func (s *AppServerV3) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets the resource subkind.
func (s *AppServerV3) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns the resource ID.
func (s *AppServerV3) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets the resource ID.
func (s *AppServerV3) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetMetadata returns the resource metadata.
func (s *AppServerV3) GetMetadata() Metadata {
	return s.Metadata
}

// GetNamespace returns the resource namespace.
func (s *AppServerV3) GetNamespace() string {
	return s.Metadata.Namespace
}

// SetExpiry sets the resource expiry time.
func (s *AppServerV3) SetExpiry(expiry time.Time) {
	s.Metadata.SetExpiry(expiry)
}

// Expiry returns the resource expiry time.
func (s *AppServerV3) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// GetName returns the resource name.
func (s *AppServerV3) GetName() string {
	return s.Metadata.Name
}

// SetName sets the resource name.
func (s *AppServerV3) SetName(name string) {
	s.Metadata.Name = name
}

// GetRotation returns the server CA rotation state.
func (s *AppServerV3) GetRotation() Rotation {
	return s.Spec.Rotation
}

// SetRotation sets the server CA rotation state.
func (s *AppServerV3) SetRotation(r Rotation) {
	s.Spec.Rotation = r
}

// GetApp returns the app this app server proxies.
func (s *AppServerV3) GetApp() Application {
	return s.Spec.App
}

// SetApp sets the app this app server proxies.
func (s *AppServerV3) SetApp(app Application) error {
	appV3, ok := app.(*AppV3)
	if !ok {
		return trace.BadParameter("expected *AppV3, got %T", app)
	}
	s.Spec.App = appV3
	return nil
}

// String returns the server string representation.
func (s *AppServerV3) String() string {
	return fmt.Sprintf("AppServer(Name=%v, Version=%v, Hostname=%v, HostID=%v, App=%v)",
		s.GetName(), s.GetTeleportVersion(), s.GetHostname(), s.GetHostID(), s.GetApp())
}

// setStaticFields sets static resource header and metadata fields.
func (s *AppServerV3) setStaticFields() {
	s.Kind = KindAppServer
	s.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *AppServerV3) CheckAndSetDefaults() error {
	s.setStaticFields()
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.Spec.HostID == "" {
		return trace.BadParameter("missing app server HostID")
	}
	if s.Spec.Version == "" {
		s.Spec.Version = api.Version
	}
	if s.Spec.App == nil {
		return trace.BadParameter("missing app server App")
	}
	if err := s.Spec.App.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Origin returns the origin value of the resource.
func (s *AppServerV3) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *AppServerV3) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

// GetAllLabels returns all resource's labels. Considering:
// * Static labels from `Metadata.Labels` and `Spec.App`.
// * Dynamic labels from `Spec.App.Spec`.
func (s *AppServerV3) GetAllLabels() map[string]string {
	staticLabels := make(map[string]string)
	for name, value := range s.Metadata.Labels {
		staticLabels[name] = value
	}

	var dynamicLabels map[string]CommandLabelV2
	if s.Spec.App != nil {
		for name, value := range s.Spec.App.Metadata.Labels {
			staticLabels[name] = value
		}

		dynamicLabels = s.Spec.App.Spec.DynamicLabels
	}

	return CombineLabels(staticLabels, dynamicLabels)
}

// Copy returns a copy of this app server object.
func (s *AppServerV3) Copy() AppServer {
	return proto.Clone(s).(*AppServerV3)
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *AppServerV3) MatchSearch(values []string) bool {
	return MatchSearch(nil, values, nil)
}

// AppServers represents a list of app servers.
type AppServers []AppServer

// Len returns the slice length.
func (s AppServers) Len() int { return len(s) }

// Less compares app servers by name and host ID.
func (s AppServers) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName() && s[i].GetHostID() < s[j].GetHostID()
}

// Swap swaps two app servers.
func (s AppServers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
