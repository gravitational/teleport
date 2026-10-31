// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// GetLLMEndpointQuotas retrieve one or more LLM endpoint quotas.
func GetLLMEndpointQuotas(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		quotas, err := stream.Collect(
			clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error) {
				return client.LLMUsageClient().ListLLMEndpointQuotas(ctx, limit, startKey)
			}),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return newLLMEndpointQuotaCollection(quotas), nil
	}

	resp, err := client.LLMUsageClient().GetLLMEndpointQuota(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newLLMEndpointQuotaCollection([]*llmv1.LLMEndpointQuota{resp}), nil
}

type llmEndpointQuotaCollection struct {
	quotas []*llmv1.LLMEndpointQuota
}

// newLLMEndpointQuotaCollection creates a [Collection] over the LLM endpoint
// quotas.
func newLLMEndpointQuotaCollection(quotas []*llmv1.LLMEndpointQuota) Collection {
	return &llmEndpointQuotaCollection{quotas: quotas}
}

func (c *llmEndpointQuotaCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.quotas))
	for _, quota := range c.quotas {
		r = append(r, types.ProtoResource153ToLegacy(quota))
	}
	return r
}

func (c *llmEndpointQuotaCollection) WriteText(w io.Writer, _ bool) error {
	var rows [][]string
	for _, quota := range c.quotas {
		status := quota.GetStatus()
		if status == nil {
			continue
		}

		for _, interval := range status.GetActiveWindows() {
			resetsAt := "Never"
			if interval.GetEndsAt() != nil && interval.GetEndsAt().IsValid() {
				resetsAt = interval.GetEndsAt().AsTime().Format(time.RFC3339)
			}

			rows = append(rows, []string{
				quota.GetMetadata().GetName(),
				interval.GetIntervalName(),
				// TODO(gabrielcorado): show quota limits
				fmt.Sprintf("%d", interval.GetInputTokens()),
				fmt.Sprintf("%d", interval.GetOutputTokens()),
				resetsAt,
			})
		}
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "No active LLM endpoint quotas available.")
		return nil
	}

	headers := []string{"Endpoint", "Interval", "Input usage", "Output usage", "Resets at"}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func llmEndpointQuotaHandler() Handler {
	return Handler{
		getHandler:  GetLLMEndpointQuotas,
		description: "Large language model (LLM) quota consumption",
	}
}
