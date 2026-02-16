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
	"strings"

	"github.com/gravitational/trace"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// inferencePolicyCollection is a collection of InferencePolicy resources that
// can be written to an io.Writer in a human-readable format.
type inferencePolicyCollection []*summarizerv1.InferencePolicy

func (c inferencePolicyCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c inferencePolicyCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description", "Kinds", "Filter", "Model"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		rows = append(rows, []string{
			meta.GetName(),
			meta.GetDescription(),
			strings.Join(spec.Kinds, ", "),
			spec.Filter,
			spec.Model,
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func inferencePolicyHandler() Handler {
	return Handler{
		getHandler:    getInferencePolicies,
		createHandler: createInferencePolicy,
		updateHandler: updateInferencePolicy,
		deleteHandler: deleteInferencePolicy,
		singleton:     false,
		mfaRequired:   false,
		description:   "Specifies which sessions will be summarized and which inference model to use",
	}
}

func createInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts,
) error {
	policy, err := services.UnmarshalProtoResource[*summarizerv1.InferencePolicy](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if opts.Force {
		req := &summarizerv1.UpsertInferencePolicyRequest{
			Policy: policy,
		}
		_, err = sclt.UpsertInferencePolicy(ctx, req)
	} else {
		req := &summarizerv1.CreateInferencePolicyRequest{
			Policy: policy,
		}
		_, err = sclt.CreateInferencePolicy(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("inference_policy %q has been created\n", policy.GetMetadata().GetName())
	return nil
}

func updateInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts,
) error {
	policy, err := services.UnmarshalProtoResource[*summarizerv1.InferencePolicy](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateInferencePolicyRequest{
		Policy: policy,
	}
	if _, err := clt.SummarizerServiceClient().UpdateInferencePolicy(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_policy %q has been updated\n", policy.GetMetadata().GetName())
	return nil
}

func getInferencePolicies(
	ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts,
) (Collection, error) {
	ssclt := clt.SummarizerServiceClient()
	if ref.Name != "" {
		req := &summarizerv1.GetInferencePolicyRequest{
			Name: ref.Name,
		}
		res, err := ssclt.GetInferencePolicy(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return inferencePolicyCollection{res.Policy}, nil
	}

	items, err := stream.Collect(clientutils.Resources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.InferencePolicy, string, error) {
			resp, err := ssclt.ListInferencePolicies(
				ctx,
				&summarizerv1.ListInferencePoliciesRequest{
					PageToken: nextPageToken,
				})
			return resp.GetPolicies(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inferencePolicyCollection(items), nil
}

func deleteInferencePolicy(
	ctx context.Context, clt *authclient.Client, ref services.Ref,
) error {
	req := &summarizerv1.DeleteInferencePolicyRequest{
		Name: ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferencePolicy(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_policy %q has been deleted\n", ref.Name)
	return nil
}
