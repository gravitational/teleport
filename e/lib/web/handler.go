package web

import (
	"fmt"
	"net/http"
	"strings"

	enterpriseAuth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/web/ui"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Plugin is our plugins to web API of teleport OSS
type Plugin struct {
	// ProxyClient is authenticated auth server client
	ProxyClient auth.ClientI
}

// InitPlugin initializes sets plugin that adds extra web handlers
func InitPlugin() {
	web.SetPlugin(&Plugin{})
}

// AddHandlers registries Plugin handlers
func (p *Plugin) AddHandlers(h *web.Handler) {
	h.GET("/enterprise/sites/:site/resources/:kind", h.WithClusterAuth(p.getResourceHandle))
	h.PUT("/enterprise/sites/:site/resources", h.WithClusterAuth(p.upsertResourceHandle))
	h.POST("/enterprise/sites/:site/resources", h.WithClusterAuth(p.upsertResourceHandle))
	h.DELETE("/enterprise/sites/:site/resources/:kind/:name", h.WithClusterAuth(p.deleteResourceHandle))
	h.GET("/enterprise/license/status", httplib.MakeHandler(p.getLicenseCheckStatusHandle))
	p.ProxyClient = h.GetProxyClient()
}

// getResourceHandle is GET handler that returns ConfigItems for requested resource kind
func (p *Plugin) getResourceHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := c.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kind := params.ByName("kind")
	data, err := getResourceByKind(kind, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return makeResponse(data)
}

// upsertResourceHandle is POST|PUT handler that upserts a new resource
func (p *Plugin) upsertResourceHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	var itemToUpsert ui.ConfigItem
	if err := httplib.ReadJSON(r, &itemToUpsert); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := c.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawRes, err := extractResourceInfo(itemToUpsert.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = validateKind(rawRes.Kind, itemToUpsert.Kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = validateMetadata(*rawRes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exists, err := checkIfResourceExists(*rawRes, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if exists && r.Method == http.MethodPost {
		return nil, trace.AlreadyExists("%q already exists", rawRes.Metadata.Name)
	}

	if !exists && r.Method == http.MethodPut {
		return nil, trace.NotFound("Cannot find resource with a name %q", rawRes.Metadata.Name)
	}

	items, err := upsertResource(*rawRes, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeResponse(items)
}

// deleteResourceHandle is DELETE handler that removes a resource by its kind and name values
func (p *Plugin) deleteResourceHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := c.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKind := params.ByName("kind")
	resourceName := params.ByName("name")
	if err := deleteResource(resourceKind, resourceName, clt); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

// getLicenseCheckStatusHandle is GET handle that returns the license check status
func (p *Plugin) getLicenseCheckStatusHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	client, ok := p.ProxyClient.(*auth.Client)
	if !ok {
		return nil, trace.BadParameter("expected *auth.Client, got: %T", client)
	}
	enterpriseClient, err := enterpriseAuth.NewClient(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	licenseCheckResult, err := enterpriseClient.GetLicenseCheckResult()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ui.NewLicenseCheckStatus(licenseCheckResult), nil
}

// message returns structured message response
func message(msg string) interface{} {
	return map[string]string{"message": msg}
}

// ok returns structured OK response
func ok() interface{} {
	return message("OK")
}

// deleteResource deletes a resource
func deleteResource(resourceKind string, resourceName string, client auth.ClientI) error {
	switch resourceKind {
	case services.KindSAMLConnector:
		if err := client.DeleteSAMLConnector(resourceName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case services.KindOIDCConnector:
		if err := client.DeleteOIDCConnector(resourceName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case services.KindGithubConnector:
		if err := client.DeleteGithubConnector(resourceName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case services.KindRole:
		if err := client.DeleteRole(resourceName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case services.KindTrustedCluster:
		if err := client.DeleteTrustedCluster(resourceName); err != nil {
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter(getInvalidKindMessage(resourceKind))
	}
}

// getResourceByKind returns a collection of ConfigItem wrappers that contains resources of requested kind
func getResourceByKind(kind string, client auth.ClientI) ([]ui.ConfigItem, error) {
	if kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch kind {
	case services.KindAuthConnector:
		oidcConnectors, err := client.GetOIDCConnectors(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		samlConnectors, err := client.GetSAMLConnectors(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		githubConnectors, err := client.GetGithubConnectors(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiSAMLItems, err := ui.ConvertSAMLConnectors(samlConnectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiOIDCItems, err := ui.ConvertOIDCConnectors(oidcConnectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiGithubItems, err := ui.ConvertGithubConnectors(githubConnectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items := append(uiSAMLItems, uiOIDCItems...)
		return append(items, uiGithubItems...), nil
	case services.KindRole:
		roles, err := client.GetRoles()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ui.ConvertRoles(roles)
	case services.KindTrustedCluster:
		trustedClusters, err := client.GetTrustedClusters()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ui.ConvertTrustedClusters(trustedClusters)
	}

	return nil, trace.BadParameter(getInvalidKindMessage(kind))
}

// upsertResource updates a resource and returns ConfigItem wrapper with the updated resource
func upsertResource(unknownRes services.UnknownResource, client auth.ClientI) (interface{}, error) {
	json := unknownRes.Raw
	switch unknownRes.Kind {
	case services.KindSAMLConnector:
		conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := conn.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := client.UpsertSAMLConnector(conn); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertSAMLConnectors([]services.SAMLConnector{conn})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return items, nil
	case services.KindOIDCConnector:
		conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := client.UpsertOIDCConnector(conn); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertOIDCConnectors([]services.OIDCConnector{conn})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return items, nil
	case services.KindGithubConnector:
		conn, err := services.GetGithubConnectorMarshaler().Unmarshal(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := client.UpsertGithubConnector(conn); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertGithubConnectors([]services.GithubConnector{conn})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return items, nil
	case services.KindRole:
		role, err := services.GetRoleMarshaler().UnmarshalRole(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = role.CheckAndSetDefaults()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := client.UpsertRole(role); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertRoles([]services.Role{role})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return items, nil
	case services.KindTrustedCluster:
		tc, err := services.GetTrustedClusterMarshaler().Unmarshal(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var upserted services.TrustedCluster
		if upserted, err = client.UpsertTrustedCluster(tc); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertTrustedClusters([]services.TrustedCluster{upserted})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return items, nil
	case "":
		return nil, trace.BadParameter("missing resource kind")
	default:
		return nil, trace.BadParameter(getInvalidKindMessage(unknownRes.Kind))
	}
}

// checkIfResourceExists checks if resource exists and returns an error if cannot find it
func checkIfResourceExists(unknownRes services.UnknownResource, client auth.ClientI) (bool, error) {
	var err error
	switch unknownRes.Kind {
	case services.KindOIDCConnector:
		_, err = client.GetOIDCConnector(unknownRes.Metadata.Name, false)
	case services.KindSAMLConnector:
		_, err = client.GetSAMLConnector(unknownRes.Metadata.Name, false)
	case services.KindGithubConnector:
		_, err = client.GetGithubConnector(unknownRes.Metadata.Name, false)
	case services.KindRole:
		_, err = client.GetRole(unknownRes.Metadata.Name)
	case services.KindTrustedCluster:
		// trusted cluster name will be automatically resolved
		if unknownRes.Metadata.Name == "" {
			return false, nil
		}
		_, err = client.GetTrustedCluster(unknownRes.Metadata.Name)
	default:
		return false, trace.BadParameter(getInvalidKindMessage(unknownRes.Kind))
	}

	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}

	return err == nil, nil
}

// validateMetadata verifies resource metadata
func validateMetadata(unknownRes services.UnknownResource) error {
	// since trusted cluster allows empty names, ignore this check
	if unknownRes.Kind == services.KindTrustedCluster {
		return nil
	}

	return unknownRes.Metadata.CheckAndSetDefaults()
}

// validateKind verifies that given resource kind matches its expected value.
func validateKind(given string, expected string) error {
	if expected == services.KindAuthConnector {
		switch given {
		case services.KindOIDCConnector, services.KindSAMLConnector, services.KindGithubConnector:
			return nil
		}
	}

	if expected == given {
		return nil
	}

	return trace.BadParameter("Invalid value for kind")
}

// getInvalidKindMessage returns a message about unsupported resource kind
func getInvalidKindMessage(kind string) string {
	return fmt.Sprintf("resources of kind %q are not supported", kind)
}

// extractResourceInfo extracts resource information
func extractResourceInfo(yaml string) (*services.UnknownResource, error) {
	var unknownRes services.UnknownResource
	reader := strings.NewReader(yaml)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&unknownRes)
	if err != nil {
		return nil, trace.BadParameter("Not a valid resource declaration")
	}

	return &unknownRes, nil
}

type webAPIResponse struct {
	Items interface{} `json:"items"`
}

// makeResponse takes a collection of objects and returns API response object
func makeResponse(items interface{}) (interface{}, error) {
	return webAPIResponse{Items: items}, nil
}
