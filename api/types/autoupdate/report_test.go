package autoupdate

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNewAutoUpdateAgentReport(t *testing.T) {
	now := timestamppb.Now()
	expires := timestamppb.New(now.AsTime().Add(autoUpdateAgentReportTTL))
	tests := []struct {
		name     string
		spec     *autoupdate.AutoUpdateAgentReportSpec
		authName string

		want    *autoupdate.AutoUpdateAgentReport
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:     "nil spec",
			authName: "test",
			wantErr:  require.Error,
		},
		{
			name: "empty name",
			spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: now,
			},
			wantErr: require.Error,
		},
		{
			name:     "no timestamp",
			authName: "test",
			spec:     &autoupdate.AutoUpdateAgentReportSpec{},
			wantErr:  require.Error,
		},
		{
			name:     "ok",
			authName: "test",
			spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: now,
			},
			want: &autoupdate.AutoUpdateAgentReport{
				Kind:    types.KindAutoUpdateAgentReport,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "test",
					Expires: expires,
				},
				Spec: &autoupdate.AutoUpdateAgentReportSpec{
					Timestamp: now,
				},
			},
			wantErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewAutoUpdateAgentReport(tt.spec, tt.authName)
			tt.wantErr(t, err)
			require.Empty(t, cmp.Diff(tt.want, result, protocmp.Transform()))
		})
	}
}
