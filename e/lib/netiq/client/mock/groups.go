package mock

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
)

func (s *Server) handleGetGroups(w http.ResponseWriter, req *http.Request) {
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

	type listGroupsResponse struct {
		Groups    []Group `json:"groups"`
		NextIndex int     `json:"nextIndex"`
	}

	windowResponse, responseNextIndex := windowResponse(s.groups, nextIndex)
	_ = writeJSON(w, listGroupsResponse{
		Groups:    windowResponse,
		NextIndex: responseNextIndex,
	})
}

// Group represents a group in the Identity Vault.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func defaultGroups() []Group {
	return []Group{
		{
			ID:          "cn=Group1,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group1",
			Description: "Group 1",
		},
		{
			ID:          "cn=Group2,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group2",
			Description: "Group 2",
		},
		{
			ID:          "cn=Group3,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group3",
			Description: "Group 3",
		},
	}
}

func (s *Server) handleGetGroupMembers(w http.ResponseWriter, req *http.Request) {
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

	type listGroupsMembersResponse struct {
		Recipients   []GroupMember `json:"recipients"`
		ArraySize    int           `json:"arraySize"`
		NextIndex    int           `json:"nextIndex"`
		CurrentIndex string        `json:"currentIndex"`
		EndIndex     string        `json:"endIndex"`
		Total        int           `json:"total"`
	}

	members, ok := s.groupMembers[payload.DN]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_ = writeJSON(w, newBadRequestError("invalid group ID "+payload.DN))
		return
	}

	windowResponse, responseNextIndex := windowResponse(members, nextIndex)
	_ = writeJSON(w, listGroupsMembersResponse{
		Recipients: windowResponse,
		NextIndex:  responseNextIndex,
	})
}

// GroupMember represents a member of a group in the Identity Vault.
type GroupMember struct {
	// Name is the name of the member.
	Name string `json:"name"`
	// Dn is the distinguished name of the member.
	Dn string `json:"dn"`
	// IsGroupAssignment is a flag that determines whether the member is a group assignment.
	IsGroupAssignment bool `json:"isGroupAssignment"`
}

func defaultGroupMembers() map[string][]GroupMember {
	return map[string][]GroupMember{
		"cn=Group1,cn=Groups,cn=Access,cn=IDVault": {
			{
				Dn:                "cn=user1,ou=users,dc=example,dc=com",
				Name:              "User One",
				IsGroupAssignment: false,
			},
			{
				Dn:                "cn=user2,ou=users,dc=example,dc=com",
				Name:              "User Two",
				IsGroupAssignment: false,
			},
		},
		"cn=Group2,cn=Groups,cn=Access,cn=IDVault": {
			{
				Dn:                "cn=user3,ou=users,dc=example,dc=com",
				Name:              "User Three",
				IsGroupAssignment: false,
			},
		},
		"cn=Group3,cn=Groups,cn=Access,cn=IDVault": {},
	}
}

type idPayloadRequest struct {
	ID string `json:"id"`
}

func decodePayloadRequestBody(r io.Reader) (idPayloadRequest, error) {
	b := idPayloadRequest{}
	err := json.NewDecoder(r).Decode(&b)
	return b, trace.Wrap(err)
}

type dnPayloadRequest struct {
	DN string `json:"dn"`
}

func decodeDNPayloadRequestBody(r io.Reader) (dnPayloadRequest, error) {
	b := dnPayloadRequest{}
	err := json.NewDecoder(r).Decode(&b)
	return b, trace.Wrap(err)
}
