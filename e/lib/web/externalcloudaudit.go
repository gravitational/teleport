package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// externalCloudAuditGenerate generates a new ExternalCloudAudit configuration
// and saves it as the current draft.
func externalCloudAuditGenerate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.GenerateDraftExternalCloudAuditRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	auditConfig, err := userClient.GetClusterAuditConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch cluster audit config")
	}

	clt := userClient.ExternalCloudAuditClient()
	generated, err := clt.GenerateDraftExternalCloudAudit(ctx, req.IntegrationName, auditConfig.Region())
	return generated, trace.Wrap(err)
}

type externalCloudAuditBootstrapArg struct {
	queryParam string
	cliFlag    string
	optional   bool
	validate   func(string) error
}

// Some of the query params use shorter names to keep the URL a somewhat
// manageable length while still being readable.
var ecaBootstrapArgs = []externalCloudAuditBootstrapArg{
	{
		queryParam: "region",
		cliFlag:    "aws-region",
		validate:   aws.IsValidRegion,
	},
	{
		queryParam: "role",
		cliFlag:    "role",
		validate:   aws.IsValidIAMRoleName,
	},
	{
		queryParam: "policy",
		cliFlag:    "policy",
		validate:   aws.IsValidIAMPolicyName,
	},
	{
		queryParam: "recordings",
		cliFlag:    "session-recordings",
		validate:   externalcloudaudit.ValidateS3URI,
	},
	{
		queryParam: "events",
		cliFlag:    "audit-events",
		validate:   externalcloudaudit.ValidateS3URI,
	},
	{
		queryParam: "results",
		cliFlag:    "athena-results",
		validate:   externalcloudaudit.ValidateS3URI,
	},
	{
		queryParam: "workgroup",
		cliFlag:    "athena-workgroup",
		validate:   aws.IsValidAthenaWorkgroupName,
	},
	{
		queryParam: "db",
		cliFlag:    "glue-database",
		validate:   aws.IsValidGlueResourceName,
	},
	{
		queryParam: "table",
		cliFlag:    "glue-table",
		validate:   aws.IsValidGlueResourceName,
	},
	{
		queryParam: "partition",
		cliFlag:    "aws-partition",
		optional:   true,
		validate:   aws.IsValidPartition,
	},
}

func readExternalCloudAuditBootstrapArgsFromQuery(query url.Values) ([]string, error) {
	cliArgs := []string{
		"integration",
		"configure",
		"externalcloudaudit",
		"--bootstrap",
	}
	for _, arg := range ecaBootstrapArgs {
		value := query.Get(arg.queryParam)
		if len(value) == 0 {
			if arg.optional {
				continue
			}
			return nil, trace.BadParameter("required parameter %q not found", arg.queryParam)
		}
		if err := arg.validate(value); err != nil {
			return nil, trace.Wrap(err, "validating %s param", arg.queryParam)
		}
		cliArgs = append(cliArgs, fmt.Sprintf(`--%s=%s`, arg.cliFlag, value))
	}
	return cliArgs, nil
}

func getExternalCloudAuditBootstrapScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	cliArgs, err := readExternalCloudAuditBootstrapArgsFromQuery(r.URL.Query())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(cliArgs, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete the external audit setup.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httplib.SetScriptHeaders(w.Header())
	_, err = io.WriteString(w, script)
	return nil, trace.Wrap(err)
}

// enableExternalCloudAuditDraft promotes the current draft ExternalCloudAudit to active.
func (h *Plugin) externalCloudAuditPromote(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalCloudAuditClient()
	err = clt.PromoteToClusterExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to promote current draft external audit config to cluster")
	}

	return web.OK(), nil
}

// externalCloudAuditGetCluster returns the current active ExternalCloudAudit.
func (h *Plugin) externalCloudAuditGetCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalCloudAuditClient()
	clusterAudit, err := clt.GetClusterExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch cluster external cloud audit")
	}

	return ui.ExternalCloudAudit{
		IntegrationName: clusterAudit.Spec.IntegrationName,
	}, nil
}

// externalCloudAuditGetDraft returns the current draft ExternalCloudAudit.
func (h *Plugin) externalCloudAuditGetDraft(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalCloudAuditClient()
	draftAudit, err := clt.GetDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch draft external cloud audit")
	}

	return ui.ExternalCloudAudit{
		IntegrationName: draftAudit.Spec.IntegrationName,
	}, nil
}

// externalCloudAuditDeleteDraft deletes the current ExternalCloudAudit draft.
func (h *Plugin) externalCloudAuditDeleteDraft(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalCloudAuditClient()
	err = clt.DeleteDraftExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to delete draft external cloud audit")
	}

	return web.OK(), nil
}

// externalCloudAuditDeleteCluster deletes the current active ExternalCloudAudit.
func (h *Plugin) externalCloudAuditDeleteCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalCloudAuditClient()
	err = clt.DisableClusterExternalCloudAudit(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to delete cluster external cloud audit")
	}

	return web.OK(), nil
}
