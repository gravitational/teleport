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

package common

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

// CRUD operations for InferenceModel resources

// createInferenceModel creates or updates a new inference model resource.
func (rc *ResourceCommand) createInferenceModel(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.InferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
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

// updateInferenceModel updates an inference model resource.
func (rc *ResourceCommand) updateInferenceModel(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
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

// getInferenceModels retrieves one or more inference model resources.
func (rc *ResourceCommand) getInferenceModels(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetInferenceModelRequest{
			Name: rc.ref.Name,
		}
		res, err := ssclt.GetInferenceModel(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return InferenceModelCollection{res.Model}, nil
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
	return InferenceModelCollection(items), nil
}

// deleteInferenceModel deletes an inference model resource.
func (rc *ResourceCommand) deleteInferenceModel(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteInferenceModelRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_model %q has been deleted\n", rc.ref.Name)
	return nil
}

// InferenceModelCollection is a collection of InferenceModel resources that
// can be written to an io.Writer in a human-readable format.
type InferenceModelCollection []*summarizerv1.InferenceModel

func (c InferenceModelCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c InferenceModelCollection) Resources() []types.Resource {
	return c.resources()
}

func (c InferenceModelCollection) writeText(w io.Writer, verbose bool) error {
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

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

// CRUD operations for InferenceSecret resources

// createInferenceSecret creates or updates a new inference secret resource.
func (rc *ResourceCommand) createInferenceSecret(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	secret, err := services.UnmarshalProtoResource[*summarizerv1.InferenceSecret](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
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

// getInferenceSecrets retrieves one or more inference secret resources.
func (rc *ResourceCommand) getInferenceSecrets(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetInferenceSecretRequest{
			Name: rc.ref.Name,
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

func (rc *ResourceCommand) deleteInferenceSecret(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteInferenceSecretRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferenceSecret(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_secret %q has been deleted\n", rc.ref.Name)
	return nil
}

// inferenceSecretCollection is a collection of InferenceSecret resources that
// can be written to an io.Writer in a human-readable format.
type inferenceSecretCollection []*summarizerv1.InferenceSecret

func (c inferenceSecretCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c inferenceSecretCollection) writeText(w io.Writer, verbose bool) error {
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

// CRUD operations for InferencePolicy resources

// createInferencePolicy creates or updates a new inference policy resource.
func (rc *ResourceCommand) createInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	policy, err := services.UnmarshalProtoResource[*summarizerv1.InferencePolicy](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
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

// updateInferencePolicy updates an inference policy resource.
func (rc *ResourceCommand) updateInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
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

// getInferencePolicies retrieves one or more inference policy resources.
func (rc *ResourceCommand) getInferencePolicies(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetInferencePolicyRequest{
			Name: rc.ref.Name,
		}
		res, err := ssclt.GetInferencePolicy(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return InferencePolicyCollection{res.Policy}, nil
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
	return InferencePolicyCollection(items), nil
}

// deleteInferencePolicy deletes an inference policy resource.
func (rc *ResourceCommand) deleteInferencePolicy(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteInferencePolicyRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteInferencePolicy(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("inference_policy %q has been deleted\n", rc.ref.Name)
	return nil
}

// InferencePolicyCollection is a collection of InferencePolicy resources that
// can be written to an io.Writer in a human-readable format.
type InferencePolicyCollection []*summarizerv1.InferencePolicy

func (c InferencePolicyCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c InferencePolicyCollection) Resources() []types.Resource {
	return c.resources()
}

func (c InferencePolicyCollection) writeText(w io.Writer, verbose bool) error {
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
