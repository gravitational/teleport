package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getAccessMonitoringRules(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.AccessMonitoringRuleClient()

	query := r.URL.Query()

	subject := query.Get("subject")
	limit, err := web.QueryLimit(query, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var rules []ui.AccessMonitoringRuleWithYaml
	nextKey := query.Get("startKey")
	// Until auth server supports filtering, proxy will temporarily do the filtering.
	for {
		var page []*accessmonitoringrulesv1.AccessMonitoringRule
		var err error

		page, nextKey, err = client.ListAccessMonitoringRules(r.Context(), limit, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, rule := range page {
			// TODO: Move filtering to backend. Support more subjects.
			// Web UI currently only supports access request.
			if subject != "" && subject == types.KindAccessRequest {
				subjects := rule.GetSpec().GetSubjects()
				// Technically user can define more than one subject, but
				// we currently don't expect that use case (more for future-proofing).
				if len(subjects) > 0 && subjects[0] != types.KindAccessRequest {
					continue
				}
			}

			uiRule, err := ui.MakeAccessMonitoringRuleWithYamlContent(rule)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			rules = append(rules, uiRule)
		}

		if nextKey == "" || len(rules) >= limit {
			break
		}
	}

	return ui.AccessMonitoringRulePage{
		Rules:    rules,
		StartKey: nextKey,
	}, nil
}

func (p *Plugin) createAccessMonitoringRule(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.AccessMonitoringRuleClient()

	var req ui.AccessMonitoringRuleWithYaml
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var resource *accessmonitoringrulesv1.AccessMonitoringRule
	if len(req.YAML) == 0 {
		resource = req.Object
	} else {
		resource, err = extractAndValidateRule(req.YAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	resourceName := resource.GetMetadata().GetName()
	resourceLabels := resource.GetMetadata().GetLabels()
	newResource, err := services.NewAccessMonitoringRuleWithLabels(resourceName, resourceLabels, resource.GetSpec())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := client.CreateAccessMonitoringRule(r.Context(), newResource)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("resource with name %q already exists", resourceName)
		}
		return nil, trace.Wrap(err)
	}
	uiRule, err := ui.MakeAccessMonitoringRuleWithYamlContent(created)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return uiRule, trace.Wrap(err)
}

func (p *Plugin) updateAccessMonitoringRule(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.AccessMonitoringRuleClient()

	var req ui.AccessMonitoringRuleWithYaml
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var resource *accessmonitoringrulesv1.AccessMonitoringRule
	if len(req.YAML) == 0 {
		resource = req.Object
	} else {
		resource, err = extractAndValidateRule(req.YAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	resourceName := params.ByName("name")
	if resourceName == "" {
		return nil, trace.BadParameter("missing resource name")
	}
	// Error if the user is trying to rename the resource.
	if resource.Metadata.Name != resourceName {
		return nil, trace.BadParameter("resource renaming is not supported, please create a different resource and then delete this one")
	}

	updated, err := client.UpdateAccessMonitoringRule(r.Context(), resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiRule, err := ui.MakeAccessMonitoringRuleWithYamlContent(updated)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return uiRule, trace.Wrap(err)
}

func (p *Plugin) deleteAccessMonitoringRule(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.AccessMonitoringRuleClient()

	if err = client.DeleteAccessMonitoringRule(r.Context(), params.ByName("name")); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), trace.Wrap(err)
}

func extractAndValidateRule(content string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	extractedRes, err := web.ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if extractedRes.Kind != types.KindAccessMonitoringRule {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}
	resource, err := services.UnmarshalAccessMonitoringRule(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}
