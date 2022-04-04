// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"sort"

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"

	"github.com/gravitational/trace"
)

// ListApps lists cluster applications
func (s *Handler) ListApps(ctx context.Context, req *api.ListAppsRequest) (*api.ListAppsResponse, error) {
	apps, err := s.DaemonService.ListApps(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListAppsResponse{}
	for _, app := range apps {
		response.Apps = append(response.Apps, newAPIApp(app))
	}

	return response, nil
}

func newAPIApp(app clusters.App) *api.App {
	apiLabels := APILabels{}
	for name, value := range app.GetAllLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(apiLabels)

	return &api.App{
		Uri:         app.URI.String(),
		Name:        app.GetName(),
		Labels:      apiLabels,
		Description: app.GetDescription(),
		AppUri:      app.GetURI(),
		PublicAddr:  app.GetPublicAddr(),
		AwsConsole:  app.IsAWSConsole(),
	}
}
