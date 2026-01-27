// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	"github.com/gravitational/trace"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// inferenceSecretCollection is a collection of InferenceSecret resources that
// can be written to an io.Writer in a human-readable format.
type inferenceSecretCollection []*summarizerv1.InferenceSecret

func (c inferenceSecretCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c inferenceSecretCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		rows = append(rows, []string{
			meta.GetName(),
			meta.GetDescription(),
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

func inferenceSecretHandler() Handler {
	return Handler{
		getHandler:    getInferenceSecret,
		createHandler: createInferenceSecret,
		updateHandler: updateInferenceSecret,
		deleteHandler: deleteInferenceSecret,
		singleton:     false,
		mfaRequired:   false,
		description:   "Stores session summarization inference provider secrets",
	}
}

func createInferenceSecret(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	secret, err := services.UnmarshalProtoResource[*summarizerv1.InferenceSecret](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if opts.Force {
		req := &summarizerv1.UpsertInferenceSecretRequest{
			Secret: secret,
		}
		_, err = sclt.UpsertInferenceSecret(ctx, req)
	} else {
		req := &summarizerv1.CreateInferenceSecretRequest{
			Secret: secret,
		}
		_, err = sclt.CreateInferenceSecret(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("inference_secret %q has been created\n", secret.GetMetadata().GetName())
	return nil
}

func updateInferenceSecret(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	secret, err := services.UnmarshalProtoResource[*summarizerv1.InferenceSecret](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateInferenceSecretRequest{
		Secret: secret,
	}
	if _, err := clt.SummarizerServiceClient().UpdateInferenceSecret(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_secret %q has been updated\n", secret.GetMetadata().GetName())
	return nil
}

func getInferenceSecret(ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	ssclt := clt.SummarizerServiceClient()
	if ref.Name != "" {
		req := &summarizerv1.GetInferenceSecretRequest{
			Name: ref.Name,
		}
		res, err := ssclt.GetInferenceSecret(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return inferenceSecretCollection{res.Secret}, nil
	}

	items, err := stream.Collect(clientutils.Resources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.InferenceSecret, string, error) {
			resp, err := ssclt.ListInferenceSecrets(
				ctx,
				&summarizerv1.ListInferenceSecretsRequest{
					PageToken: nextPageToken,
				})
			return resp.GetSecrets(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inferenceSecretCollection(items), nil
}

func deleteInferenceSecret(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	req := &summarizerv1.DeleteInferenceSecretRequest{
		Name: ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferenceSecret(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_secret %q has been deleted\n", ref.Name)
	return nil
}
