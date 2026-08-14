package mock

import (
	"net/http"
	"strconv"
)

func (s *Server) handleGetResources(w http.ResponseWriter, req *http.Request) {
	if err := s.validateBearerToken(req); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	queryNextIndex := req.URL.Query().Get("nextIndex")
	nextIndex, err := strconv.Atoi(queryNextIndex)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid nextIndex"))
		return
	}

	type listResourcesResponse struct {
		NextIndex int        `json:"nextIndex"`
		Resources []Resource `json:"resources"`
	}

	windowResponse, responseNextIndex := windowResponse(s.resources, nextIndex)
	_ = writeJSON(w, listResourcesResponse{
		Resources: windowResponse,
		NextIndex: responseNextIndex,
	})
}

// Resource represents a resource in the Identity Vault.
type Resource struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Description is the description of the resource.
	Description string `json:"description"`
	// Categories is the list of categories the resource belongs to.
	Categories []Category `json:"categories,omitempty"`
}

// Category represents a category in the Identity Vault.
type Category struct {
	// ID is the unique identifier of the category.
	ID string `json:"id"`
	// Name is the name of the category.
	Name string `json:"name"`
}

func defaultResources() []Resource {
	return []Resource{
		{
			ID:          "cn=Resource1,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource1",
			Description: "Resource 1",
		},
		{
			ID:          "cn=Resource2,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource2",
			Description: "Resource 2",
		},
		{
			ID:          "cn=Resource3,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource3",
			Description: "Resource 3",
		},
	}
}
