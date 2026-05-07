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
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

// retrievalModelCollection is a collection of RetrievalModel resources that
// can be written to an io.Writer in a human-readable format.
type retrievalModelCollection []*summarizerv1.RetrievalModel

func (c retrievalModelCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c retrievalModelCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description", "Provider", "Provider Model ID", "Inference Model"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		providerName := ""
		providerModel := ""
		if spec.GetOpenai() != nil {
			providerName = "OpenAI"
			providerModel = spec.GetOpenai().GetOpenaiModelId()
		} else if spec.GetBedrock() != nil {
			providerName = "Bedrock"
			providerModel = spec.GetBedrock().GetBedrockModelId()
		}
		rows = append(rows, []string{
			meta.GetName(),
			meta.GetDescription(),
			providerName,
			providerModel,
			spec.GetInferenceModelName(),
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func retrievalModelHandler() Handler {
	return Handler{
		getHandler:    getRetrievalModel,
		createHandler: createRetrievalModel,
		updateHandler: updateRetrievalModel,
		deleteHandler: deleteRetrievalModel,
		singleton:     true,
		mfaRequired:   false,
		description:   "Specifies the embeddings provider configuration used for session search",
	}
}

func createRetrievalModel(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.RetrievalModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if opts.Force {
		_, err = sclt.UpsertRetrievalModel(ctx, &summarizerv1.UpsertRetrievalModelRequest{
			Model: model,
		})
	} else {
		_, err = sclt.CreateRetrievalModel(ctx, &summarizerv1.CreateRetrievalModelRequest{
			Model: model,
		})
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("retrieval_model %q has been created\n", model.GetMetadata().GetName())
	return nil
}

func updateRetrievalModel(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.RetrievalModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := clt.SummarizerServiceClient().UpdateRetrievalModel(ctx, &summarizerv1.UpdateRetrievalModelRequest{
		Model: model,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("retrieval_model %q has been updated\n", model.GetMetadata().GetName())
	return nil
}

func getRetrievalModel(ctx context.Context, clt *authclient.Client, _ services.Ref, _ GetOpts) (Collection, error) {
	res, err := clt.SummarizerServiceClient().GetRetrievalModel(ctx, &summarizerv1.GetRetrievalModelRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return retrievalModelCollection{res.GetModel()}, nil
}

func deleteRetrievalModel(ctx context.Context, clt *authclient.Client, _ services.Ref) error {
	if _, err := clt.SummarizerServiceClient().DeleteRetrievalModel(ctx, &summarizerv1.DeleteRetrievalModelRequest{}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("retrieval_model has been deleted")
	return nil
}
