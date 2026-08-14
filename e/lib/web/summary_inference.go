package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) registerInferenceHandlers() {
	// Inference Model handlers
	p.h.GET("/webapi/sites/:site/inference/models", p.h.WithClusterAuth(p.listInferenceModels))
	p.h.GET("/webapi/sites/:site/inference/models/:name", p.h.WithClusterAuth(p.getInferenceModel))
	p.h.POST("/webapi/sites/:site/inference/models", p.h.WithClusterAuth(p.createInferenceModel))
	p.h.PUT("/webapi/sites/:site/inference/models/:name", p.h.WithClusterAuth(p.updateInferenceModel))
	p.h.DELETE("/webapi/sites/:site/inference/models/:name", p.h.WithClusterAuth(p.deleteInferenceModel))

	// Test inference model endpoint
	p.h.POST("/webapi/sites/:site/inference/test-model", p.h.WithClusterAuth(p.testInferenceModel))

	// Inference Secret handlers
	p.h.GET("/webapi/sites/:site/inference/secrets", p.h.WithClusterAuth(p.listInferenceSecrets))
	p.h.GET("/webapi/sites/:site/inference/secrets/:name", p.h.WithClusterAuth(p.getInferenceSecret))
	p.h.POST("/webapi/sites/:site/inference/secrets", p.h.WithClusterAuth(p.createInferenceSecret))
	p.h.PUT("/webapi/sites/:site/inference/secrets/:name", p.h.WithClusterAuth(p.updateInferenceSecret))
	p.h.DELETE("/webapi/sites/:site/inference/secrets/:name", p.h.WithClusterAuth(p.deleteInferenceSecret))

	// Inference Policy handlers
	p.h.GET("/webapi/sites/:site/inference/policies", p.h.WithClusterAuth(p.listInferencePolicies))
	p.h.GET("/webapi/sites/:site/inference/policies/:name", p.h.WithClusterAuth(p.getInferencePolicy))
	p.h.POST("/webapi/sites/:site/inference/policies", p.h.WithClusterAuth(p.createInferencePolicy))
	p.h.PUT("/webapi/sites/:site/inference/policies/:name", p.h.WithClusterAuth(p.updateInferencePolicy))
	p.h.DELETE("/webapi/sites/:site/inference/policies/:name", p.h.WithClusterAuth(p.deleteInferencePolicy))

	// Retrieval Model handlers. The retrieval model is a singleton resource, so
	// the routes do not take a name.
	p.h.GET("/webapi/sites/:site/retrieval/model", p.h.WithClusterAuth(p.getRetrievalModel))
	p.h.POST("/webapi/sites/:site/retrieval/model", p.h.WithClusterAuth(p.createRetrievalModel))
	p.h.PUT("/webapi/sites/:site/retrieval/model", p.h.WithClusterAuth(p.updateRetrievalModel))
	p.h.DELETE("/webapi/sites/:site/retrieval/model", p.h.WithClusterAuth(p.deleteRetrievalModel))

	// Test retrieval model endpoint
	p.h.POST("/webapi/sites/:site/retrieval/test-model", p.h.WithClusterAuth(p.testRetrievalModel))
}

// listInferenceModels lists all inference models.
func (h *Plugin) listInferenceModels(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	pageSize, err := web.QueryLimitAsInt32(query, "limit", 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().ListInferenceModels(
		r.Context(),
		summarizerv1.ListInferenceModelsRequest_builder{
			PageSize:  pageSize,
			PageToken: query.Get("startKey"),
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListInferenceModelsResponse{
		Items:   ui.MakeInferenceModels(response.GetModels()),
		NextKey: response.GetNextPageToken(),
	}, nil
}

// getInferenceModel retrieves a specific inference model.
func (h *Plugin) getInferenceModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetInferenceModel(
		r.Context(),
		summarizerv1.GetInferenceModelRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceModel(response.GetModel()), nil
}

// createInferenceModel creates a new inference model.
func (h *Plugin) createInferenceModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiModel ui.InferenceModel
	if err := httplib.ReadJSON(r, &uiModel); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().CreateInferenceModel(
		r.Context(),
		summarizerv1.CreateInferenceModelRequest_builder{Model: uiModel.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceModel(response.GetModel()), nil
}

// updateInferenceModel updates an existing inference model.
func (h *Plugin) updateInferenceModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	var uiModel ui.InferenceModel
	if err := httplib.ReadJSON(r, &uiModel); err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the name in the URL matches the model name
	if uiModel.Name != "" && uiModel.Name != name {
		return nil, trace.BadParameter("model name in URL does not match model name in body")
	}
	uiModel.Name = name

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().UpsertInferenceModel(
		r.Context(),
		summarizerv1.UpsertInferenceModelRequest_builder{Model: uiModel.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceModel(response.GetModel()), nil
}

// deleteInferenceModel deletes an inference model.
func (h *Plugin) deleteInferenceModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.SummarizerServiceClient().DeleteInferenceModel(
		r.Context(),
		summarizerv1.DeleteInferenceModelRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// listInferenceSecrets lists all inference secrets.
func (h *Plugin) listInferenceSecrets(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	pageSize, err := web.QueryLimitAsInt32(query, "limit", 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().ListInferenceSecrets(
		r.Context(),
		summarizerv1.ListInferenceSecretsRequest_builder{
			PageSize:  pageSize,
			PageToken: query.Get("startKey"),
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListInferenceSecretsResponse{
		Items:   ui.MakeInferenceSecrets(response.GetSecrets()),
		NextKey: response.GetNextPageToken(),
	}, nil
}

// getInferenceSecret retrieves a specific inference secret.
func (h *Plugin) getInferenceSecret(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetInferenceSecret(
		r.Context(),
		summarizerv1.GetInferenceSecretRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceSecret(response.GetSecret()), nil
}

// createInferenceSecret creates a new inference secret.
func (h *Plugin) createInferenceSecret(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiSecret ui.InferenceSecret
	if err := httplib.ReadJSON(r, &uiSecret); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().CreateInferenceSecret(
		r.Context(),
		summarizerv1.CreateInferenceSecretRequest_builder{Secret: uiSecret.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceSecret(response.GetSecret()), nil
}

// updateInferenceSecret updates an existing inference secret.
func (h *Plugin) updateInferenceSecret(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	var uiSecret ui.InferenceSecret
	if err := httplib.ReadJSON(r, &uiSecret); err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the name in the URL matches the secret name
	if uiSecret.Name != "" && uiSecret.Name != name {
		return nil, trace.BadParameter("secret name in URL does not match secret name in body")
	}
	uiSecret.Name = name

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().UpsertInferenceSecret(
		r.Context(),
		summarizerv1.UpsertInferenceSecretRequest_builder{Secret: uiSecret.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferenceSecret(response.GetSecret()), nil
}

// deleteInferenceSecret deletes an inference secret.
func (h *Plugin) deleteInferenceSecret(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.SummarizerServiceClient().DeleteInferenceSecret(
		r.Context(),
		summarizerv1.DeleteInferenceSecretRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// listInferencePolicies lists all inference policies.
func (h *Plugin) listInferencePolicies(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	pageSize, err := web.QueryLimitAsInt32(query, "limit", 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().ListInferencePolicies(
		r.Context(),
		summarizerv1.ListInferencePoliciesRequest_builder{
			PageSize:  pageSize,
			PageToken: query.Get("startKey"),
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListInferencePoliciesResponse{
		Items:   ui.MakeInferencePolicies(response.GetPolicies()),
		NextKey: response.GetNextPageToken(),
	}, nil
}

// getInferencePolicy retrieves a specific inference policy.
func (h *Plugin) getInferencePolicy(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetInferencePolicy(
		r.Context(),
		summarizerv1.GetInferencePolicyRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferencePolicy(response.GetPolicy()), nil
}

// createInferencePolicy creates a new inference policy.
func (h *Plugin) createInferencePolicy(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiPolicy ui.InferencePolicy
	if err := httplib.ReadJSON(r, &uiPolicy); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().CreateInferencePolicy(
		r.Context(),
		summarizerv1.CreateInferencePolicyRequest_builder{Policy: uiPolicy.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferencePolicy(response.GetPolicy()), nil
}

// updateInferencePolicy updates an existing inference policy.
func (h *Plugin) updateInferencePolicy(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	var uiPolicy ui.InferencePolicy
	if err := httplib.ReadJSON(r, &uiPolicy); err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the name in the URL matches the policy name
	if uiPolicy.Name != "" && uiPolicy.Name != name {
		return nil, trace.BadParameter("policy name in URL does not match policy name in body")
	}
	uiPolicy.Name = name

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().UpsertInferencePolicy(
		r.Context(),
		summarizerv1.UpsertInferencePolicyRequest_builder{Policy: uiPolicy.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeInferencePolicy(response.GetPolicy()), nil
}

// deleteInferencePolicy deletes an inference policy.
func (h *Plugin) deleteInferencePolicy(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.SummarizerServiceClient().DeleteInferencePolicy(
		r.Context(),
		summarizerv1.DeleteInferencePolicyRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// testInferenceModel tests an inference model configuration by making a test request.
func (h *Plugin) testInferenceModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var testReq ui.TestInferenceModelRequest
	if err := httplib.ReadJSON(r, &testReq); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().TestInferenceModel(
		r.Context(),
		testReq.ToProto(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeTestInferenceModelResponse(response), nil
}

// getRetrievalModel retrieves the singleton retrieval model.
func (h *Plugin) getRetrievalModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().GetRetrievalModel(
		r.Context(),
		summarizerv1.GetRetrievalModelRequest_builder{}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeRetrievalModel(response.GetModel()), nil
}

// createRetrievalModel creates the singleton retrieval model.
func (h *Plugin) createRetrievalModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiModel ui.RetrievalModel
	if err := httplib.ReadJSON(r, &uiModel); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().CreateRetrievalModel(
		r.Context(),
		summarizerv1.CreateRetrievalModelRequest_builder{Model: uiModel.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeRetrievalModel(response.GetModel()), nil
}

// updateRetrievalModel updates the singleton retrieval model.
func (h *Plugin) updateRetrievalModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiModel ui.RetrievalModel
	if err := httplib.ReadJSON(r, &uiModel); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use Upsert rather than the conditional Update RPC: the UI representation
	// does not carry a resource revision, mirroring updateInferenceModel.
	response, err := clt.SummarizerServiceClient().UpsertRetrievalModel(
		r.Context(),
		summarizerv1.UpsertRetrievalModelRequest_builder{Model: uiModel.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeRetrievalModel(response.GetModel()), nil
}

// deleteRetrievalModel deletes the singleton retrieval model.
func (h *Plugin) deleteRetrievalModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.SummarizerServiceClient().DeleteRetrievalModel(
		r.Context(),
		summarizerv1.DeleteRetrievalModelRequest_builder{}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// testRetrievalModel tests a retrieval model configuration by making a test request.
func (h *Plugin) testRetrievalModel(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var testReq ui.TestRetrievalModelRequest
	if err := httplib.ReadJSON(r, &testReq); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().TestRetrievalModel(
		r.Context(),
		testReq.ToProto(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeTestRetrievalModelResponse(response), nil
}
