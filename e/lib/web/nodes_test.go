package web

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
)

func TestCreateNodeJoinToken(t *testing.T) {
	m := &mockedNodeAPIGetter{}
	m.mockGenerateToken = func(ctx context.Context, req auth.GenerateTokenRequest) (string, error) {
		return "some-token-id", nil
	}

	token, err := createNodeJoinToken(context.Background(), m)
	assert.Nil(t, err)

	assert.Equal(t, defaults.NodeJoinTokenTTL, token.Expiry.Sub(time.Now().UTC()).Round(time.Second))
	assert.Equal(t, "some-token-id", token.ID)
}

func TestGetNodeJoinScript(t *testing.T) {
	m := &mockedNodeAPIGetter{}
	m.mockGetProxyServers = func() ([]services.Server, error) {
		var s services.ServerV2
		s.SetPublicAddr("test-host:12345678")

		return []services.Server{&s}, nil
	}
	m.mockGetClusterCACert = func() (*auth.LocalCAResponse, error) {
		fakeBytes := []byte(fixtures.SigningCertPEM)
		return &auth.LocalCAResponse{TLSCA: fakeBytes}, nil
	}

	// Test bad token length.
	script, err := getNodeJoinScript("", m)
	assert.Empty(t, script)
	assert.True(t, trace.IsBadParameter(err))

	script, err = getNodeJoinScript("f18da1c9f6630a51e8daf121e7451d", m)
	assert.Empty(t, script)
	assert.True(t, trace.IsBadParameter(err))

	// Test valid token format.
	testTokenID := "f18da1c9f6630a51e8daf121e7451daa"
	script, err = getNodeJoinScript(testTokenID, m)
	assert.Nil(t, err)

	assert.Contains(t, script, testTokenID)
	assert.Contains(t, script, "test-host")
	assert.Contains(t, script, "12345678")
	assert.Contains(t, script, "sha256:")
}

type mockedNodeAPIGetter struct {
	mockGenerateToken    func(ctx context.Context, req auth.GenerateTokenRequest) (string, error)
	mockGetProxyServers  func() ([]services.Server, error)
	mockGetClusterCACert func() (*auth.LocalCAResponse, error)
}

func (m *mockedNodeAPIGetter) GenerateToken(ctx context.Context, req auth.GenerateTokenRequest) (string, error) {
	if m.mockGenerateToken != nil {
		return m.mockGenerateToken(ctx, req)
	}

	return "", trace.NotImplemented("mockGenerateToken not implemented")
}

func (m *mockedNodeAPIGetter) GetProxies() ([]services.Server, error) {
	if m.mockGetProxyServers != nil {
		return m.mockGetProxyServers()
	}

	return nil, trace.NotImplemented("mockGetProxyServers not implemented")
}

func (m *mockedNodeAPIGetter) GetClusterCACert() (*auth.LocalCAResponse, error) {
	if m.mockGetClusterCACert != nil {
		return m.mockGetClusterCACert()
	}

	return nil, trace.NotImplemented("mockGetClusterCACert not implemented")
}
