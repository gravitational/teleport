/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

// inferenceModelCollection is a collection of InferenceModel resources that
// can be written to an io.Writer in a human-readable format.
type inferenceModelCollection []*summarizerv1.InferenceModel

func (c inferenceModelCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c inferenceModelCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description", "Provider", "Provider Model ID"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		providerName := ""
		providerModel := ""
		if spec.GetOpenai() != nil {
			providerName = "OpenAI"
			providerModel = spec.GetOpenai().OpenaiModelId
		}
		rows = append(rows, []string{
			meta.GetName(),
			meta.GetDescription(),
			providerName,
			providerModel,
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func inferenceModelHandler() Handler {
	return Handler{
		getHandler:    getInferenceModel,
		createHandler: createInferenceModel,
		updateHandler: updateInferenceModel,
		deleteHandler: deleteInferenceModel,
		singleton:     false,
		mfaRequired:   false,
		description:   "Specifies a session summarization inference model configuration",
	}
}

func createInferenceModel(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.InferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if opts.Force {
		req := &summarizerv1.UpsertInferenceModelRequest{
			Model: model,
		}
		_, err = sclt.UpsertInferenceModel(ctx, req)
	} else {
		req := &summarizerv1.CreateInferenceModelRequest{
			Model: model,
		}
		_, err = sclt.CreateInferenceModel(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("inference_model %q has been created\n", model.GetMetadata().GetName())
	return nil
}

func updateInferenceModel(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.InferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateInferenceModelRequest{
		Model: model,
	}
	if _, err := clt.SummarizerServiceClient().UpdateInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_model %q has been updated\n", model.GetMetadata().GetName())
	return nil
}

func getInferenceModel(ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	ssclt := clt.SummarizerServiceClient()
	if ref.Name != "" {
		req := &summarizerv1.GetInferenceModelRequest{
			Name: ref.Name,
		}
		res, err := ssclt.GetInferenceModel(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return inferenceModelCollection{res.Model}, nil
	}

	items, err := stream.Collect(clientutils.Resources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.InferenceModel, string, error) {
			resp, err := ssclt.ListInferenceModels(
				ctx,
				&summarizerv1.ListInferenceModelsRequest{
					PageToken: nextPageToken,
				})
			return resp.GetModels(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return inferenceModelCollection(items), nil
}

func deleteInferenceModel(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	req := &summarizerv1.DeleteInferenceModelRequest{
		Name: ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_model %q has been deleted\n", ref.Name)
	return nil
}
