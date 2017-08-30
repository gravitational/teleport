package web

import (
	"io"
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

func (p *Plugin) getResourceHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKind := params.ByName("kind")
	data, err := getResourceByKind(resourceKind, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeItemsResponse(data)
}

func (p *Plugin) upsertResourceHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	var req *upsertRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	items, err := upsertResource(req.Yaml, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeItemsResponse(items)
}

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

type upsertRequest struct {
	Yaml string `json:"yaml"`
}

// message returns structured message response
func message(msg string) interface{} {
	return map[string]string{"message": msg}
}

// ok returns structured OK response
func ok() interface{} {
	return message("OK")
}

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
		return trace.BadParameter("%q is not supported", resourceKind)
	}
}

func getResourceByKind(kind string, client auth.ClientI) (interface{}, error) {
	if kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}
	switch kind {
	case services.KindSAMLConnector:
		connectors, err := client.GetSAMLConnectors(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ui.ConvertSAMLConnectors(connectors)
	case services.KindOIDCConnector:
		connectors, err := client.GetOIDCConnectors(true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ui.ConvertOIDCConnectors(connectors)
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

	return nil, trace.BadParameter("'%v' is not supported", kind)
}

func upsertResource(data string, client auth.ClientI) (interface{}, error) {
	var raw services.UnknownResource
	reader := strings.NewReader(data)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&raw)
	if err != nil {
		if err == io.EOF {
			return nil, trace.BadParameter("no resources found, emtpy input?")

		}
		return nil, trace.Wrap(err)
	}

	yaml := raw.Raw
	switch raw.Kind {
	case services.KindSAMLConnector:
		conn, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(yaml)
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
		conn, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(yaml)
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
		role, err := services.GetRoleMarshaler().UnmarshalRole(yaml)
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
		tc, err := services.GetTrustedClusterMarshaler().Unmarshal(yaml)
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
		return nil, trace.BadParameter("%q is not supported", raw.Kind)
	}
}

type itemsResponse struct {
	Items interface{} `json:"items"`
}

func makeItemsResponse(items interface{}) (interface{}, error) {
	return itemsResponse{Items: items}, nil
}
