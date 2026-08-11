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

func (p *Plugin) registerClassifierHandlers() {
	p.h.GET("/webapi/sites/:site/classifiers", p.h.WithClusterAuth(p.listClassifiers))
	p.h.GET("/webapi/sites/:site/classifiers/:name", p.h.WithClusterAuth(p.getClassifier))
	p.h.POST("/webapi/sites/:site/classifiers", p.h.WithClusterAuth(p.createClassifier))
	p.h.PUT("/webapi/sites/:site/classifiers/:name", p.h.WithClusterAuth(p.updateClassifier))
	p.h.DELETE("/webapi/sites/:site/classifiers/:name", p.h.WithClusterAuth(p.deleteClassifier))
}

func (h *Plugin) listClassifiers(
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

	response, err := clt.SummarizerServiceClient().ListClassifiers(
		r.Context(),
		summarizerv1.ListClassifiersRequest_builder{
			PageSize:  pageSize,
			PageToken: query.Get("startKey"),
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListClassifiersResponse{
		Items:   ui.MakeClassifiers(response.GetClassifiers()),
		NextKey: response.GetNextPageToken(),
	}, nil
}

func (h *Plugin) getClassifier(
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

	response, err := clt.SummarizerServiceClient().GetClassifier(
		r.Context(),
		summarizerv1.GetClassifierRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeClassifier(response.GetClassifier()), nil
}

func (h *Plugin) createClassifier(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	var uiClassifier ui.Classifier
	if err := httplib.ReadJSON(r, &uiClassifier); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().CreateClassifier(
		r.Context(),
		summarizerv1.CreateClassifierRequest_builder{Classifier: uiClassifier.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeClassifier(response.GetClassifier()), nil
}

func (h *Plugin) updateClassifier(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	name := p.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("name is required")
	}

	var uiClassifier ui.Classifier
	if err := httplib.ReadJSON(r, &uiClassifier); err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure the name in the URL matches the classifier name.
	if uiClassifier.Name != "" && uiClassifier.Name != name {
		return nil, trace.BadParameter("classifier name in URL does not match classifier name in body")
	}
	uiClassifier.Name = name

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.SummarizerServiceClient().UpsertClassifier(
		r.Context(),
		summarizerv1.UpsertClassifierRequest_builder{Classifier: uiClassifier.ToProto()}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeClassifier(response.GetClassifier()), nil
}

func (h *Plugin) deleteClassifier(
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

	_, err = clt.SummarizerServiceClient().DeleteClassifier(
		r.Context(),
		summarizerv1.DeleteClassifierRequest_builder{Name: name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}
