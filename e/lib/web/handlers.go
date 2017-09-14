package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/e/lib/web/ui"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Plugin is our plugins to web API of teleport OSS
type Plugin struct {
}

// AddHandlers registeres Plugin handlers
func (p *Plugin) AddHandlers(h *web.Handler) {
	h.GET("/enterprise/resources/:kind", h.WithAuth(p.getResourceHandler))
	h.PUT("/enterprise/resources", h.WithAuth(p.upsertResourceHandler))
	h.POST("/enterprise/resources", h.WithAuth(p.upsertResourceHandler))
	h.DELETE("/enterprise/resources/:kind/:name", h.WithAuth(p.deleteResourceHandler))
}

// getResourceHandler is GET handler that returns ConfigItems for requested resource kind
func (p *Plugin) getResourceHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
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

// upsertResourceHandler is POST|PUT handler that upserts a new resource
func (p *Plugin) upsertResourceHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	var itemToUpsert ui.ConfigItem
	if err := httplib.ReadJSON(r, &itemToUpsert); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawRes, err := extractMetadata(itemToUpsert.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exists, err := checkIfResourceExists(*rawRes, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if exists && r.Method == http.MethodPost {
		return nil, trace.AlreadyExists("'%s' already exists", rawRes.Metadata.Name)
	}

	if !exists && r.Method == http.MethodPut {
		return nil, trace.NotFound("Cannot find resource with a name '%v'", rawRes.Metadata.Name)
	}

	err = validateKind(rawRes.Kind, itemToUpsert.Kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	items, err := upsertResource(*rawRes, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeResponse(items)
}

// deleteResourceHandler is DELETE handler that removes a resource by its kind and name values
func (p *Plugin) deleteResourceHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	client, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKind := params.ByName("kind")
	resourceName := params.ByName("name")
	if err := deleteResource(resourceKind, resourceName, client); err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
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

		uiSAMLItems, err := ui.ConvertSAMLConnectors(samlConnectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiOIDCItems, err := ui.ConvertOIDCConnectors(oidcConnectors)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return append(uiSAMLItems, uiOIDCItems...), nil
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
	case services.KindRole:
		role, err := services.GetRoleMarshaler().UnmarshalRole(json)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = role.CheckAndSetDefaults()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := client.UpsertRole(role, backend.Forever); err != nil {
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
		if err := client.UpsertTrustedCluster(tc); err != nil {
			return nil, trace.Wrap(err)
		}
		items, err := ui.ConvertTrustedClusters([]services.TrustedCluster{tc})
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
	case services.KindRole:
		_, err = client.GetRole(unknownRes.Metadata.Name)
	case services.KindTrustedCluster:
		_, err = client.GetTrustedCluster(unknownRes.Metadata.Name)
	default:
		return false, trace.BadParameter(getInvalidKindMessage(unknownRes.Kind))
	}

	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}

	return err == nil, nil
}

// validateKind verifies that given resource kind matches its expected value.
func validateKind(given string, expected string) error {
	if expected == services.KindAuthConnector {
		// AuthConnector might be of 2 types OIDC and SAML
		if given == services.KindOIDCConnector || given == services.KindSAMLConnector {
			return nil
		}
	}

	if expected == given {
		return nil
	}

	return trace.BadParameter("Invalid value for kind")
}

func getInvalidKindMessage(kind string) string {
	return fmt.Sprintf("resources of kind %q are not supported", kind)
}

// extractMetadata extracts resource meta information
func extractMetadata(yaml string) (*services.UnknownResource, error) {
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
