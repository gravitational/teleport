package services

import (
	"context"
	"iter"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

type Summarizer interface {
	CreateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	GetSummarizationInferenceModel(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceModel, error)
	UpdateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	UpsertSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	DeleteSummarizationInferenceModel(ctx context.Context, name string) error
	ListSummarizationInferenceModels(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceModel, string, error)

	CreateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	GetSummarizationInferenceSecret(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceSecret, error)
	UpdateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	UpsertSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	DeleteSummarizationInferenceSecret(ctx context.Context, name string) error
	ListSummarizationInferenceSecrets(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceSecret, string, error)

	CreateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	GetSummarizationInferencePolicy(ctx context.Context, name string) (*summarizerv1.SummarizationInferencePolicy, error)
	UpdateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	UpsertSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	DeleteSummarizationInferencePolicy(ctx context.Context, name string) error
	ListSummarizationInferencePolicies(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferencePolicy, string, error)
	// AllSummarizationInferencePolicies returns all summarization inference policies.
	AllSummarizationInferencePolicies(ctx context.Context) iter.Seq2[*summarizerv1.SummarizationInferencePolicy, error]
}
