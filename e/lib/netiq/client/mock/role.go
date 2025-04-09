package mock

import (
	"net/http"
	"strconv"
)

func (s *Server) handleGetRoles(w http.ResponseWriter, req *http.Request) {
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

	type listResponse struct {
		NextIndex int    `json:"nextIndex"`
		Roles     []Role `json:"roles"`
	}

	windowResponse, responseNextIndex := windowResponse(s.roles, nextIndex)
	_ = writeJSON(w, listResponse{
		Roles:     windowResponse,
		NextIndex: responseNextIndex,
	})
}

func (s *Server) handleGetRoleMembers(w http.ResponseWriter, req *http.Request) {
	if err := s.validateBearerToken(req); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = writeJSON(w, newBadRequestError(req.Method+" method not allowed"))
		return
	}

	payload, err := decodeDNPayloadRequestBody(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid request body"))
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

	type listResponse struct {
		Recipients []RoleAssignmentStatus `json:"assignmentStatusList"`
		NextIndex  int                    `json:"nextIndex"`
	}

	members, ok := s.roleMembers[payload.DN]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid role ID "+payload.DN))
		return
	}

	windowResponse, responseNextIndex := windowResponse(members, nextIndex)
	_ = writeJSON(w, listResponse{
		Recipients: windowResponse,
		NextIndex:  responseNextIndex,
	})
}

func (s *Server) handleGetRoleParents(w http.ResponseWriter, req *http.Request) {
	if err := s.validateBearerToken(req); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = writeJSON(w, newBadRequestError(req.Method+" method not allowed"))
		return
	}

	payload, err := decodePayloadRequestBody(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid request body"))
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

	type listResponse struct {
		Roles     []RoleRef `json:"roles"`
		NextIndex int       `json:"nextIndex"`
	}

	members, ok := s.roleParents[payload.ID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid role ID "+payload.ID))
		return
	}

	windowResponse, responseNextIndex := windowResponse(members, nextIndex)
	_ = writeJSON(w, listResponse{
		Roles:     windowResponse,
		NextIndex: responseNextIndex,
	})
}

// Role represents a role in the Identity Vault.
type Role struct {
	// ID is the unique identifier of the role (e.g. "cn=Role1,cn=Roles,cn=Access,cn=IDVault").
	ID string `json:"id"`
	// Name is the name of the role (e.g. "Role1").
	Name string `json:"name"`
	// Description is the description of the role.
	Description string `json:"description"`
	// Level is the level of the role.
	Level int `json:"level"`
	// RoleLevel is the role level.
	RoleLevel struct {
		// Name is the name of the role level.
		Name string `json:"name"`
		// Level is the level of the role level.
		Level int `json:"level"`
		// Cn is the common name of the role level.
		Cn string `json:"cn"`
	} `json:"roleLevel"`
}

func defaultRoles() []Role {
	return []Role{
		{
			ID:          "cn=Role1,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role1",
			Description: "Role 1",
		},
		{
			ID:          "cn=Role2,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role2",
			Description: "Role 2",
		},
		{
			ID:          "cn=Role3,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role3",
			Description: "Role 3",
		},
	}
}

type RoleAssignmentStatus struct {
	// DN is the distinguished name of the role assignment.
	Dn string `json:"dn"`
	// RecipientType is the type of the recipient.
	// Can be "USER" or "GROUP".
	RecipientType string `json:"recipientType"`
	// RecipientTypeSubContainer is the sub container of the recipient type.
	RecipientTypeSubContainer string `json:"recipientTypeSubContainer"`
	// RecipientDn is the distinguished name of the recipient.
	RecipientDn string `json:"recipientDn"`
	// RecipientFullName is the full name of the recipient.
	RecipientFullName string `json:"recipientFullName"`
	// StatusCode is the status code of the role assignment.
	StatusCode string `json:"statusCode"`
	// StatusDisplay is the display of the status.
	StatusDisplay string `json:"statusDisplay"`
	// EffectiveDate is the effective date of the role assignment.
	EffectiveDate string `json:"effectiveDate"`
	// ExpiryDate is the expiry date of the role assignment.
	ExpiryDate string `json:"expiryDate"`
	// Description is the description of the role assignment.
	Description string `json:"description"`
	// Grant is a flag that determines whether the role assignment is granted.
	Grant bool `json:"grant"`
}

func defaultRoleMembers() map[string][]RoleAssignmentStatus {
	return map[string][]RoleAssignmentStatus{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				Dn:                        "cn=user1,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user1,ou=users,dc=example,dc=com",
				RecipientFullName:         "User One",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User One",
				Grant:                     true,
			},
			{
				Dn:                        "cn=user2,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user2,ou=users,dc=example,dc=com",
				RecipientFullName:         "User Two",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User Two",
				Grant:                     true,
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": {
			{
				Dn:                        "cn=user3,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user3,ou=users,dc=example,dc=com",
				RecipientFullName:         "User 3",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User Three",
				Grant:                     true,
			},
		},
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": {},
	}
}

// RoleRef represents a role reference in the Identity Vault.
type RoleRef struct {
	// ID is the unique identifier of the role (e.g. "cn=Role1,cn=Roles,cn=Access,cn=IDVault").
	ID string `json:"id"`
	// Name is the name of the role (e.g. "Role1").
	Name string `json:"name"`
	// Description is the description of the role.
	Description string `json:"description"`
	// Requester is the requester of the role.
	Requester string `json:"requester"`
	// Level is the level of the role.
	Level int `json:"level"`
	// RequestDescription is the description of the request.
	RequestDescription string `json:"requestDescription"`
}

func defaultRoleParentRefs() map[string][]RoleRef {
	return map[string][]RoleRef{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:          "cn=Role2,cn=Roles,cn=Access,cn=IDVault",
				Name:        "Role2",
				Description: "Role 2",
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": nil,
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": nil,
	}
}

func (s *Server) handleGetRoleMappedResources(w http.ResponseWriter, req *http.Request) {
	if err := s.validateBearerToken(req); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = writeJSON(w, err)
		return
	}

	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = writeJSON(w, newBadRequestError(req.Method+" method not allowed"))
		return
	}

	payload, err := decodePayloadRequestBody(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid request body"))
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

	type listResponse struct {
		Resources []ResourceRef `json:"resources"`
		NextIndex int           `json:"nextIndex"`
	}

	resources, ok := s.mappedResources[payload.ID]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid role ID "+payload.ID))
		return
	}

	windowResponse, responseNextIndex := windowResponse(resources, nextIndex)
	_ = writeJSON(w, listResponse{
		Resources: windowResponse,
		NextIndex: responseNextIndex,
	})
}

// ResourceRef represents a resource reference in the Identity Vault.
type ResourceRef struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Description is the description of the resource.
	Description string `json:"description"`
	// MappingDescription is the description of the mapping.
	MappingDescription string `json:"mappingDescription"`
	// Status is the display of the status.
	Status int `json:"status"`
	// EntityKey is the entity key of the resource.
	EntityKey string `json:"entityKey"`
	// EntitlementValues is the list of entitlement values associated with the resource.
	Entitlements []EntitlementValue `json:"entitlementValues"`
}

// EntitlementValue represents an entitlement value in the Identity Vault.
type EntitlementValue struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Value is the value of the entitlement.
	Value string `json:"value"`
}

func defaultMappedResources() map[string][]ResourceRef {
	return map[string][]ResourceRef{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:                 "cn=Resource1,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource1",
				Description:        "Resource 1",
				MappingDescription: "Resource 1 mapping",
				Status:             1,
				EntityKey:          "resource1",
			},
			{
				ID:                 "cn=Resource2,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource2",
				Description:        "Resource 2",
				MappingDescription: "Resource 2 mapping",
				Status:             1,
				EntityKey:          "resource2",
				Entitlements: []EntitlementValue{
					{
						Name:  "Entitlement1",
						ID:    "Entitlement 1",
						Value: "Value1",
					},
				},
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:                 "cn=Resource3,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource3",
				Description:        "Resource 3",
				MappingDescription: "Resource 3 mapping",
				Status:             1,
				EntityKey:          "resource3",
			},
		},
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": nil,
	}
}
