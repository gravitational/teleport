package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGetOriginatingIPAddress(t *testing.T) {
	r := &http.Request{
		Header: http.Header{},
	}

	// Test without x-forwarded-for set.
	r.RemoteAddr = "test:"
	addr, err := getIPAddress(r)
	require.NoError(t, err)
	require.Equal(t, "test", addr)

	// Test with x-forwarded-for set.
	r.Header.Set("X-FORWARDED-FOR", "first, second, third")
	addr, err = getIPAddress(r)
	require.NoError(t, err)
	require.Equal(t, "first", addr)
}

func TestGetAccountRecoveryCodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := &mockedAccountRecoveryAPIGetter{}

	// Test not found error returns empty metadata object.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
		return nil, trace.NotFound("")
	}
	res, err := getAccountRecoveryCodesMetadata(ctx, m)
	require.Nil(t, err)
	require.Empty(t, res)

	// Test other errors than NotFound returns error as is.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
		return nil, trace.BadParameter("")
	}
	_, err = getAccountRecoveryCodesMetadata(ctx, m)
	require.True(t, trace.IsBadParameter(err))

	// Test non nil returns no error.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
		return &types.RecoveryCodesV1{Spec: types.RecoveryCodesSpecV1{Created: time.Unix(int64(1605139200), 0)}}, nil
	}
	res, err = getAccountRecoveryCodesMetadata(ctx, m)
	require.Nil(t, err)
	require.NotEmpty(t, res.Created)
}

type mockedAccountRecoveryAPIGetter struct {
	mockGetAccountRecoveryCodes func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error)
}

func (m *mockedAccountRecoveryAPIGetter) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
	if m.mockGetAccountRecoveryCodes != nil {
		return m.mockGetAccountRecoveryCodes(ctx, req)
	}

	return nil, trace.NotImplemented("mockGetAccountRecoveryCodes not implemented")
}
