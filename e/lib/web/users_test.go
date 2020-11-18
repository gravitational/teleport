package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestRequestParameters(t *testing.T) {
	r := saveUserRequest{
		Name:   "",
		Roles:  nil,
		Logins: nil,
	}
	assert.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "",
		Roles:  []string{"testrole"},
		Logins: nil,
	}
	assert.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "username",
		Roles:  nil,
		Logins: nil,
	}
	assert.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "username",
		Roles:  []string{"testrole"},
		Logins: nil,
	}
	assert.Nil(t, r.checkAndSetDefaults())
}

func TestCRUDs(t *testing.T) {
	u := saveUserRequest{
		Name:   "testname",
		Roles:  []string{"testrole"},
		Logins: nil,
	}

	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user services.User) error {
		return nil
	}

	m.mockGetUser = func(name string, withSecrets bool) (services.User, error) {
		return services.NewUser(name)
	}

	m.mockUpdateUser = func(ctx context.Context, user services.User) error {
		return nil
	}

	m.mockGetUsers = func(withSecrets bool) ([]services.User, error) {
		u, err := services.NewUser("testname")
		return []services.User{u}, err
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return nil
	}

	// test create
	user, err := createUser(newRequest(t, u), m, "")
	assert.Nil(t, err)
	assert.Equal(t, "testname", user.Name)
	assert.Equal(t, "local", user.AuthType)
	assert.Contains(t, user.Roles, "testrole")

	// test update
	u.Roles = []string{"newrole"}
	user, err = updateUser(newRequest(t, u), m, "")
	assert.Nil(t, err)
	assert.Contains(t, user.Roles, "newrole")

	// test list
	users, err := getUsers(m)
	assert.Nil(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "testname", users[0].Name)

	// test delete
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	assert.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	assert.Nil(t, err)
}

func TestCRUDErrors(t *testing.T) {
	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user services.User) error {
		return trace.AlreadyExists("")
	}

	m.mockGetUser = func(name string, withSecrets bool) (services.User, error) {
		return nil, trace.NotFound("")
	}

	m.mockUpdateUser = func(ctx context.Context, user services.User) error {
		return trace.NotFound("")
	}

	m.mockGetUsers = func(withSecrets bool) ([]services.User, error) {
		return nil, trace.AccessDenied("")
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return trace.NotFound("")
	}

	u := saveUserRequest{
		Name:   "testname",
		Roles:  []string{"testrole"},
		Logins: nil,
	}

	// update errors
	user, err := updateUser(newRequest(t, u), m, "")
	assert.True(t, trace.IsNotFound(err))
	assert.Nil(t, user)

	// create errors
	user, err = createUser(newRequest(t, u), m, "")
	assert.True(t, trace.IsAlreadyExists(err))
	assert.Nil(t, user)

	users, err := getUsers(m)
	assert.True(t, trace.IsAccessDenied(err))
	assert.Nil(t, users)

	// delete errors
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	assert.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	assert.True(t, trace.IsNotFound(err))

	// deleting self error
	param = httprouter.Params{httprouter.Param{Key: "username", Value: "self"}}
	req, err = http.NewRequest("", "/:username", nil)
	assert.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	assert.True(t, trace.IsBadParameter(err))
}

// newRequest creates http request with given body
func newRequest(t *testing.T, body interface{}) *http.Request {
	reqBody, err := json.Marshal(body)
	assert.Nil(t, err)

	req, err := http.NewRequest("", "", bytes.NewBuffer(reqBody))
	assert.Nil(t, err)

	return req
}

type mockedUserAPIGetter struct {
	mockGetUser    func(name string, withSecrets bool) (services.User, error)
	mockCreateUser func(ctx context.Context, user services.User) error
	mockUpdateUser func(ctx context.Context, user services.User) error
	mockGetUsers   func(withSecrets bool) ([]services.User, error)
	mockDeleteUser func(ctx context.Context, user string) error
}

func (m *mockedUserAPIGetter) GetUser(name string, withSecrets bool) (services.User, error) {
	if m.mockGetUser != nil {
		return m.mockGetUser(name, withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUser not implemented")
}

func (m *mockedUserAPIGetter) CreateUser(ctx context.Context, user services.User) error {
	if m.mockCreateUser != nil {
		return m.mockCreateUser(ctx, user)
	}
	return trace.NotImplemented("mockCreateUser not implemented")
}

func (m *mockedUserAPIGetter) UpdateUser(ctx context.Context, user services.User) error {
	if m.mockUpdateUser != nil {
		return m.mockUpdateUser(ctx, user)
	}
	return trace.NotImplemented("mockUpdateUser not implemented")
}

func (m *mockedUserAPIGetter) GetUsers(withSecrets bool) ([]services.User, error) {
	if m.mockGetUsers != nil {
		return m.mockGetUsers(withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUsers not implemented")
}

func (m *mockedUserAPIGetter) DeleteUser(ctx context.Context, name string) error {
	if m.mockDeleteUser != nil {
		return m.mockDeleteUser(ctx, name)
	}

	return trace.NotImplemented("mockDeleteUser not implemented")
}
