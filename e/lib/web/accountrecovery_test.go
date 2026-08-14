package web

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

func TestGetAccountRecoveryCodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := &mockedAccountRecoveryAPIGetter{}

	// Test not found error returns empty metadata object.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
		return nil, trace.NotFound("")
	}
	res, err := getAccountRecoveryCodesMetadata(ctx, m)
	require.NoError(t, err)
	require.Empty(t, res)

	// Test other errors than NotFound returns error as is.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
		return nil, trace.BadParameter("")
	}
	_, err = getAccountRecoveryCodesMetadata(ctx, m)
	require.True(t, trace.IsBadParameter(err))

	// Test non nil returns no error.
	m.mockGetAccountRecoveryCodes = func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
		return &proto.RecoveryCodes{Created: time.Unix(int64(1605139200), 0)}, nil
	}
	res, err = getAccountRecoveryCodesMetadata(ctx, m)
	require.NoError(t, err)
	require.NotEmpty(t, res.Created)
}

type mockedAccountRecoveryAPIGetter struct {
	mockGetAccountRecoveryCodes func(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error)
}

func (m *mockedAccountRecoveryAPIGetter) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	if m.mockGetAccountRecoveryCodes != nil {
		return m.mockGetAccountRecoveryCodes(ctx, req)
	}

	return nil, trace.NotImplemented("mockGetAccountRecoveryCodes not implemented")
}
