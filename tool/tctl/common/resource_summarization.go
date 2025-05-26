package common

import (
	"context"
	"fmt"
	"io"
	"strings"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// CRUD operations for SummarizationInferenceModel resources

func (rc *ResourceCommand) createSummarizationInferenceModel(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
		req := &summarizerv1.UpsertSummarizationInferenceModelRequest{
			Model: model,
		}
		_, err = sclt.UpsertSummarizationInferenceModel(ctx, req)
	} else {
		req := &summarizerv1.CreateSummarizationInferenceModelRequest{
			Model: model,
		}
		_, err = sclt.CreateSummarizationInferenceModel(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("summarization_inference_model %q has been created\n", model.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) updateSummarizationInferenceModel(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateSummarizationInferenceModelRequest{
		Model: model,
	}
	if _, err := clt.SummarizerServiceClient().UpdateSummarizationInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_model %q has been updated\n", model.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) getSummarizationInferenceModels(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetSummarizationInferenceModelRequest{
			Name: rc.ref.Name,
		}
		model, err := ssclt.GetSummarizationInferenceModel(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return summarizerInferenceModelCollection{model}, nil
	}

	items, err := utils.CollectResources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.SummarizationInferenceModel, string, error) {
			resp, err := ssclt.ListSummarizationInferenceModels(
				ctx,
				&summarizerv1.ListSummarizationInferenceModelsRequest{
					PageToken: nextPageToken,
				})
			return resp.GetModels(), resp.GetNextPageToken(), trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return summarizerInferenceModelCollection(items), nil
}

func (rc *ResourceCommand) deleteSummarizationInferenceModel(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteSummarizationInferenceModelRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteSummarizationInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_model %q has been deleted\n", rc.ref.Name)
	return nil
}

type summarizerInferenceModelCollection []*summarizerv1.SummarizationInferenceModel

func (c summarizerInferenceModelCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c summarizerInferenceModelCollection) writeText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description", "Provider", "Provider Model ID"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		providerName := ""
		providerModel := ""
		if spec.GetOpenAi() != nil {
			providerName = "OpenAI"
			providerModel = spec.GetOpenAi().OpenaiModelId
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

// CRUD operations for SummarizationInferenceSecret resources

func (rc *ResourceCommand) createSummarizationInferenceSecret(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	secret, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceSecret](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
		req := &summarizerv1.UpsertSummarizationInferenceSecretRequest{
			Secret: secret,
		}
		_, err = sclt.UpsertSummarizationInferenceSecret(ctx, req)
	} else {
		req := &summarizerv1.CreateSummarizationInferenceSecretRequest{
			Secret: secret,
		}
		_, err = sclt.CreateSummarizationInferenceSecret(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("summarization_inference_secret %q has been created\n", secret.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) updateSummarizationInferenceSecret(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	secret, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceSecret](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateSummarizationInferenceSecretRequest{
		Secret: secret,
	}
	if _, err := clt.SummarizerServiceClient().UpdateSummarizationInferenceSecret(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_secret %q has been updated\n", secret.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) getSummarizationInferenceSecrets(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetSummarizationInferenceSecretRequest{
			Name: rc.ref.Name,
		}
		secret, err := ssclt.GetSummarizationInferenceSecret(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return summarizerInferenceSecretCollection{secret}, nil
	}

	items, err := utils.CollectResources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.SummarizationInferenceSecret, string, error) {
			resp, err := ssclt.ListSummarizationInferenceSecrets(
				ctx,
				&summarizerv1.ListSummarizationInferenceSecretsRequest{
					PageToken: nextPageToken,
				})
			return resp.GetSecrets(), resp.GetNextPageToken(), trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return summarizerInferenceSecretCollection(items), nil
}

func (rc *ResourceCommand) deleteSummarizationInferenceSecret(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteSummarizationInferenceSecretRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteSummarizationInferenceSecret(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_secret %q has been deleted\n", rc.ref.Name)
	return nil
}

type summarizerInferenceSecretCollection []*summarizerv1.SummarizationInferenceSecret

func (c summarizerInferenceSecretCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c summarizerInferenceSecretCollection) writeText(w io.Writer, verbose bool) error {
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

// CRUD operations for SummarizationInferencePolicy resources

func (rc *ResourceCommand) createSummarizationInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	policy, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferencePolicy](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if rc.IsForced() {
		req := &summarizerv1.UpsertSummarizationInferencePolicyRequest{
			Policy: policy,
		}
		_, err = sclt.UpsertSummarizationInferencePolicy(ctx, req)
	} else {
		req := &summarizerv1.CreateSummarizationInferencePolicyRequest{
			Policy: policy,
		}
		_, err = sclt.CreateSummarizationInferencePolicy(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("summarization_inference_policy %q has been created\n", policy.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) updateSummarizationInferencePolicy(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	policy, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferencePolicy](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := &summarizerv1.UpdateSummarizationInferencePolicyRequest{
		Policy: policy,
	}
	if _, err := clt.SummarizerServiceClient().UpdateSummarizationInferencePolicy(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_policy %q has been updated\n", policy.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) getSummarizationInferencePolicies(
	ctx context.Context, clt *authclient.Client,
) (ResourceCollection, error) {
	ssclt := clt.SummarizerServiceClient()
	if rc.ref.Name != "" {
		req := &summarizerv1.GetSummarizationInferencePolicyRequest{
			Name: rc.ref.Name,
		}
		policy, err := ssclt.GetSummarizationInferencePolicy(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return summarizerInferencePolicyCollection{policy}, nil
	}

	items, err := utils.CollectResources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.SummarizationInferencePolicy, string, error) {
			resp, err := ssclt.ListSummarizationInferencePolicies(
				ctx,
				&summarizerv1.ListSummarizationInferencePoliciesRequest{
					PageToken: nextPageToken,
				})
			return resp.GetPolicies(), resp.GetNextPageToken(), trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return summarizerInferencePolicyCollection(items), nil
}

func (rc *ResourceCommand) deleteSummarizationInferencePolicy(
	ctx context.Context, clt *authclient.Client,
) error {
	req := &summarizerv1.DeleteSummarizationInferencePolicyRequest{
		Name: rc.ref.Name,
	}
	if _, err := clt.SummarizerServiceClient().DeleteSummarizationInferencePolicy(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_policy %q has been deleted\n", rc.ref.Name)
	return nil
}

type summarizerInferencePolicyCollection []*summarizerv1.SummarizationInferencePolicy

func (c summarizerInferencePolicyCollection) resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c summarizerInferencePolicyCollection) writeText(w io.Writer, verbose bool) error {
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
