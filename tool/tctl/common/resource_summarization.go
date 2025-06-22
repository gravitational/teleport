package common

import (
	"context"
	"fmt"
	"io"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func (rc *ResourceCommand) createSummarizationInferenceModel(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource,
) error {
	model, err := services.UnmarshalProtoResource[*summarizerv1.SummarizationInferenceModel](
		raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	req := &summarizerv1.CreateSummarizationInferenceModelRequest{
		Model: model,
	}
	if _, err := clt.SummarizerServiceClient().CreateSummarizationInferenceModel(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("summarization_inference_model %q has been created\n", model.GetMetadata().GetName())
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
