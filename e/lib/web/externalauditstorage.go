package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// externalAuditStorageGenerate generates a new ExternalAuditStorage configuration
// and saves it as the current draft.
func externalAuditStorageGenerate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.GenerateDraftExternalAuditStorageRequest
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

	clt := userClient.ExternalAuditStorageClient()
	generated, err := clt.GenerateDraftExternalAuditStorage(ctx, req.IntegrationName, auditConfig.Region())
	return generated, trace.Wrap(err)
}

type externalAuditStorageBootstrapArg struct {
	queryParam string
	cliFlag    string
	optional   bool
	validate   func(string) error
}

func notEmpty(s string) error {
	if len(s) > 0 {
		return nil
	}

	return trace.BadParameter("must not be empty")
}

// Some of the query params use shorter names to keep the URL a somewhat
// manageable length while still being readable.
var ecaBootstrapArgs = []externalAuditStorageBootstrapArg{
	{
		queryParam: "region",
		cliFlag:    "aws-region",
		validate:   aws.IsValidRegion,
	},
	{
		queryParam: "integration",
		cliFlag:    "integration",
		validate:   notEmpty,
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
		validate:   externalauditstorage.ValidateS3URI,
	},
	{
		queryParam: "events",
		cliFlag:    "audit-events",
		validate:   externalauditstorage.ValidateS3URI,
	},
	{
		queryParam: "results",
		cliFlag:    "athena-results",
		validate:   externalauditstorage.ValidateS3URI,
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

func readExternalAuditStorageBootstrapArgsFromQuery(query url.Values, clusterName string) ([]string, error) {
	cliArgs := []string{
		"integration",
		"configure",
		"externalauditstorage",
		"--bootstrap",
		fmt.Sprintf("--cluster-name=%s", shsprintf.EscapeDefaultContext(clusterName)),
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
		formattedArg, err := shsprintf.Sprintf(`--%s=%s`, arg.cliFlag, value)
		if err != nil {
			return nil, trace.Wrap(err, "formatting %s argument", arg.cliFlag)
		}
		cliArgs = append(cliArgs, formattedArg)
	}
	return cliArgs, nil
}

func (h *Plugin) getExternalAuditStorageBootstrapScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	clusterName := h.authMiddleware.ClusterName

	cliArgs, err := readExternalAuditStorageBootstrapArgsFromQuery(r.URL.Query(), clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(cliArgs, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete the External Audit Storage setup.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httplib.SetScriptHeaders(w.Header())
	_, err = io.WriteString(w, script)
	return nil, trace.Wrap(err)
}

// enableExternalAuditStorageDraft promotes the current draft ExternalAuditStorage to active.
func (h *Plugin) externalAuditStoragePromote(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalAuditStorageClient()
	err = clt.PromoteToClusterExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to promote current draft External Audit Storage configuration to cluster")
	}

	return web.OK(), nil
}

// externalAuditStorageGetCluster returns the current active ExternalAuditStorage.
func (h *Plugin) externalAuditStorageGetCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalAuditStorageClient()
	clusterAudit, err := clt.GetClusterExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch cluster External Audit Storage configuration")
	}

	return clusterAudit, nil
}

// externalAuditStorageGetDraft returns the current draft ExternalAuditStorage.
func (h *Plugin) externalAuditStorageGetDraft(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalAuditStorageClient()
	draftAudit, err := clt.GetDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch draft External Audit Storage configuration")
	}

	return draftAudit, nil
}

// externalAuditStorageDeleteDraft deletes the current ExternalAuditStorage draft.
func (h *Plugin) externalAuditStorageDeleteDraft(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalAuditStorageClient()
	err = clt.DeleteDraftExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to delete draft External Audit Storage configuration")
	}

	return web.OK(), nil
}

// externalAuditStorageDeleteCluster deletes the current active ExternalAuditStorage.
func (h *Plugin) externalAuditStorageDeleteCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	userClient, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := userClient.ExternalAuditStorageClient()
	err = clt.DisableClusterExternalAuditStorage(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to delete cluster External Audit Storage configuration")
	}

	return web.OK(), nil
}
