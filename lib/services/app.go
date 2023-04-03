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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// AppGetter defines interface for fetching application resources.
type AppGetter interface {
	// GetApps returns all application resources.
	GetApps(context.Context) ([]types.Application, error)
	// GetApp returns the specified application resource.
	GetApp(ctx context.Context, name string) (types.Application, error)
}

// Apps defines an interface for managing application resources.
type Apps interface {
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

// MarshalApp marshals Application resource to JSON.
func MarshalApp(app types.Application, opts ...MarshalOption) ([]byte, error) {
	if err := app.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch app := app.(type) {
	case *types.AppV3:
		if !cfg.PreserveResourceID {
			copy := *app
			copy.SetResourceID(0)
			app = &copy
		}
		return utils.FastMarshal(app)
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			app.SetResourceID(cfg.ID)
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
	if err := appServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch appServer := appServer.(type) {
	case *types.AppServerV3:
		if !cfg.PreserveResourceID {
			copy := *appServer
			copy.SetResourceID(0)
			appServer = &copy
		}
		return utils.FastMarshal(appServer)
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported app server resource version %q", h.Version)
}

// CompareAppServers returns true if two application servers are equal.
func CompareAppServers(as1, as2 types.AppServer) bool {
	if !CompareMetadata(as1.GetMetadata(), as2.GetMetadata(), false, false) {
		return false
	}

	if as1.GetKind() != as2.GetKind() {
		return false
	}

	if as1.GetSubKind() != as2.GetSubKind() {
		return false
	}

	if as1.GetVersion() != as2.GetVersion() {
		return false
	}

	if as1.GetTeleportVersion() != as2.GetTeleportVersion() {
		return false
	}

	if as1.GetHostname() != as2.GetHostname() {
		return false
	}

	if as1.GetHostID() != as2.GetHostID() {
		return false
	}

	r := as1.GetRotation()
	if !r.Matches(as2.GetRotation()) {
		return false
	}

	if !CompareApps(as1.GetApp(), as2.GetApp()) {
		return false
	}

	if len(as1.GetProxyIDs()) != len(as2.GetProxyIDs()) {
		return false
	}

	for i, proxyID := range as1.GetProxyIDs() {
		if as2.GetProxyIDs()[i] != proxyID {
			return false
		}
	}

	return true
}

// CompareApps returns true if two applications are equal.
func CompareApps(app1, app2 types.Application) bool {
	if !CompareMetadata(app1.GetMetadata(), app2.GetMetadata(), false, false) {
		return false
	}

	if app1.GetKind() != app2.GetKind() {
		return false
	}

	if app1.GetSubKind() != app2.GetSubKind() {
		return false
	}

	if app1.GetVersion() != app2.GetVersion() {
		return false
	}

	if app1.GetURI() != app2.GetURI() {
		return false
	}

	if app1.GetPublicAddr() != app2.GetPublicAddr() {
		return false
	}

	if !compareDynamicLabels(app1.GetDynamicLabels(), app2.GetDynamicLabels()) {
		return false
	}

	if app1.GetInsecureSkipVerify() != app2.GetInsecureSkipVerify() {
		return false
	}

	if !compareRewrites(app1.GetRewrite(), app2.GetRewrite()) {
		return false
	}

	if app1.IsGCP() != app2.IsGCP() {
		return false
	}

	if app1.IsTCP() != app2.IsTCP() {
		return false
	}

	if app1.GetProtocol() != app2.GetProtocol() {
		return false
	}

	if app1.GetAWSAccountID() != app2.GetAWSAccountID() {
		return false
	}

	if app1.GetAWSExternalID() != app2.GetAWSExternalID() {
		return false
	}

	if app1.IsAWSConsole() != app2.IsAWSConsole() {
		return false
	}

	if app1.IsAzureCloud() != app2.IsAzureCloud() {
		return false
	}

	return true
}

func compareDynamicLabels(dl1, dl2 map[string]types.CommandLabel) bool {
	if len(dl1) != len(dl2) {
		return false
	}

	for k, cmd1 := range dl1 {
		cmd2, ok := dl2[k]
		if !ok {
			return false
		}

		if len(cmd1.GetCommand()) != len(cmd2.GetCommand()) {
			return false
		}

		for i, v := range cmd1.GetCommand() {
			if cmd2.GetCommand()[i] != v {
				return false
			}
		}

		if cmd1.GetPeriod() != cmd2.GetPeriod() {
			return false
		}

		if cmd1.GetResult() != cmd2.GetResult() {
			return false
		}
	}

	return true
}

func compareRewrites(r1, r2 *types.Rewrite) bool {
	if len(r1.Headers) != len(r2.Headers) {
		return false
	}

	for i, h := range r1.Headers {
		if h.Name != r2.Headers[i].Name {
			return false
		}
		if h.Value != r2.Headers[i].Value {
			return false
		}
	}

	return true
}
