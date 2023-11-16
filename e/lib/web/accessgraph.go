/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"context"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/web"
)

// queryAccessGraph is a handler for the /v1/accessgraph/query endpoint.
// It queries the access graph and returns the results.
func (p *Plugin) queryAccessGraph(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, webCtx *web.SessionContext) (any, error) {
	query := r.URL.Query().Get("query")
	if query == "" {
		return nil, trace.BadParameter("query parameter is required")
	}

	cl, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agClt := cl.AccessGraphClient()
	if agClt == nil {
		return nil, trace.NotFound("access graph client is not configured")
	}

	// Skip the RBAC check. Auth server will perform it.
	ctx := r.Context()
	resp, err := agClt.Query(ctx, &accessgraphv1.QueryRequest{
		Query: query,
	})

	usageReport := getTAGResponseReport(resp)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		usageEventReq := &proto.SubmitUsageEventRequest{
			Event: &usageeventsv1.UsageEventOneOf{
				Event: &usageeventsv1.UsageEventOneOf_TagExecuteQuery{
					TagExecuteQuery: usageReport,
				},
			},
		}
		authClient, err := webCtx.GetClient()
		if err != nil {
			p.Log.WithError(err).Warn("Failed to get auth client")
			return
		}
		if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
			p.Log.WithError(err).Warn("Failed to emit TAG usage event")
		}
	}()

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// getAccessGraphFile is a handler for the /v1/accessgraph/file/:file endpoint.
// It returns web assets from TAG.
func (p *Plugin) getAccessGraphFile(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	filePath := params.ByName("file")

	acl, err := p.getAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agClt := acl.AccessGraphClient()
	if agClt == nil {
		return nil, trace.NotFound("access graph client is not configured")
	}
	ctx := r.Context()
	resp, err := agClt.GetFile(ctx, &accessgraphv1.GetFileRequest{
		Filepath: filePath,
	})
	if err != nil {
		// If access graph is not enabled in the Auth server, return 404 instead of 500.
		if trace.IsNotImplemented(err) {
			return nil, trace.NotFound("file %q is not found", filePath)
		}

		p.Log.Errorf("Failed to get file %q: %v", filePath, err)
		return nil, trace.Wrap(err)
	}

	ext := filepath.Ext(filePath)
	extType := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", extType)

	// Send the file contents to the client
	w.Write(resp.Data)

	// Return nil to prevent the content type to be set to application/json
	return nil, nil
}

// getTAGResponseReport returns a usage event report for the TAG query response.
// If the response is nil, it returns a report for an empty response with
// IsSuccess=false.
// This function is used to report TAG usage events to the auth server.
func getTAGResponseReport(rsp *accessgraphv1.QueryResponse) *usageeventsv1.TAGExecuteQueryEvent {
	if rsp == nil {
		return &usageeventsv1.TAGExecuteQueryEvent{
			TotalNodes: 0,
			TotalEdges: 0,
			IsSuccess:  false,
		}
	}
	return &usageeventsv1.TAGExecuteQueryEvent{
		TotalNodes: int64(len(rsp.Nodes)),
		TotalEdges: int64(len(rsp.Edges)),
		IsSuccess:  true,
	}
}
