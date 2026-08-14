package mock

import (
	"net/http"
	"strconv"
)

func (s *Server) handleGetUsers(w http.ResponseWriter, req *http.Request) {
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

	windowResponse, responseNextIndex := windowResponse(s.users, nextIndex)
	_ = writeJSON(w, listUsersResponse{
		UsersList: windowResponse,
		NextIndex: responseNextIndex,
	})
}

func defaultUsers() []User {
	return []User{
		{
			Dn:       "cn=user1,ou=users,dc=example,dc=com",
			FullName: "User One",
			SecondaryAttributes: []SecondaryAttribute{
				{
					Key:          "Email",
					DisplayLabel: "Email",
					DataType:     "string",
					AttributeValues: []AttributeValue{
						{
							Type:  "string",
							Value: "one@company",
						},
					},
				},
			},
			DisabledLogin: []string{"false"},
		},
		{
			Dn:       "cn=user2,ou=users,dc=example,dc=com",
			FullName: "User two",
			SecondaryAttributes: []SecondaryAttribute{
				{
					Key:          "Email",
					DisplayLabel: "Email",
					DataType:     "string",
					AttributeValues: []AttributeValue{
						{
							Type:  "string",
							Value: "two@company",
						},
					},
				},
			},
			DisabledLogin: []string{"true"},
		},
		{
			Dn:       "cn=user3,ou=users,dc=example,dc=com",
			FullName: "User tree",
			SecondaryAttributes: []SecondaryAttribute{
				{
					Key:          "Email",
					DisplayLabel: "Email",
					DataType:     "string",
					AttributeValues: []AttributeValue{
						{
							Type:  "string",
							Value: "tree@company",
						},
					},
				},
			},
			DisabledLogin: []string{"false"},
		},
	}
}

type listUsersResponse struct {
	UsersList []User `json:"usersList"`
	NextIndex int    `json:"nextIndex"`
}

// User represents a user in the system.
type User struct {
	Dn                  string               `json:"dn"`
	FullName            string               `json:"fullName"`
	SecondaryAttributes []SecondaryAttribute `json:"secondaryAttributes"`
	DisabledLogin       []string             `json:"disabledLogin"`
}

// SecondaryAttribute represents a secondary attribute of a user.
type SecondaryAttribute struct {
	Key             string           `json:"key"`
	DisplayLabel    string           `json:"displayLabel"`
	DataType        string           `json:"dataType"`
	AttributeValues []AttributeValue `json:"attributeValues,omitempty"`
}

// AttributeValue represents a value of an attribute.
type AttributeValue struct {
	Type  string `json:"@type"`
	Value string `json:"$"`
}

const windowSize = 2

func windowResponse[T any](slice []T, nextIndex int) ([]T, int) {
	// next index must be offseted by 1
	nextIndex--

	max := min(nextIndex+windowSize, len(slice))
	responseNextIndex := 0
	if nextIndex+2 <= len(slice) {
		responseNextIndex = nextIndex + windowSize + 1
	}

	return slice[nextIndex:max], responseNextIndex
}
