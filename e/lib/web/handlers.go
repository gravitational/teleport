package web

import (
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/e/lib/web/ui"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/client-go/1.4/pkg/util/yaml"
)

// Plugin is our plugins to web API of teleport OSS
type Plugin struct {
}

// AddHandlers registeres Plugin handlers
func (p *Plugin) AddHandlers(h *web.Handler) {
	h.GET("/enterprise/resources/:kind", h.WithAuth(p.getResources))
	h.PUT("/enterprise/resources/:kind", h.WithAuth(p.upsertResource))
	h.POST("/enterprise/resources/:kind", h.WithAuth(p.upsertResource))
	h.DELETE("/enterprise/resources/:kind/:name", h.WithAuth(p.deleteResource))
}

func (p *Plugin) getResources(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	resourceKind := params.ByName("kind")
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data, err := getResourceByKind(resourceKind, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeItemsResponse(data)
}

func (p *Plugin) upsertResource(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	var req *ui.ConfigItem
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKind := params.ByName("kind")
	items, err := upsertResourceByKind(resourceKind, *req, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeItemsResponse(items)
}

func (p *Plugin) deleteResource(w http.ResponseWriter, r *http.Request, params httprouter.Params, c *web.SessionContext) (interface{}, error) {
	resourceKind := params.ByName("kind")
	resourceName := params.ByName("name")

	client, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch resourceKind {
	case services.KindSAMLConnector:
		if err := client.DeleteSAMLConnector(resourceName); err != nil {
			return nil, trace.Wrap(err)
		}
		return ok(), nil
	case services.KindOIDCConnector:
		if err := client.DeleteOIDCConnector(resourceName); err != nil {
			return nil, trace.Wrap(err)
		}
		return ok(), nil
	case services.KindRole:
		if err := client.DeleteRole(resourceName); err != nil {
			return nil, trace.Wrap(err)
		}
		return ok(), nil
	case services.KindTrustedCluster:
		if err := client.DeleteTrustedCluster(resourceName); err != nil {
			return nil, trace.Wrap(err)
		}
		return ok(), nil
	default:
		return nil, trace.BadParameter("%q is not supported", resourceKind)
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

func upsertResourceByKind(kind string, uiItem ui.ConfigItem, client auth.ClientI) (interface{}, error) {
	var raw services.UnknownResource
	reader := strings.NewReader(uiItem.Content)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&raw)
	if err != nil {
		if err == io.EOF {
			return nil, trace.BadParameter("no resources found, emtpy input?")

		}
		return nil, trace.Wrap(err)
	}

	yaml := raw.Raw
	switch kind {
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
		return nil, trace.BadParameter("%q is not supported", kind)
	}
}

type itemsResponse struct {
	Items interface{} `json:"items"`
}

func makeItemsResponse(items interface{}) (interface{}, error) {
	return itemsResponse{Items: items}, nil
}
