package pluginsv1

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func TestTrimToMaxSize(t *testing.T) {
	const maxSize = 2000
	testCases := []struct {
		name         string
		in           *types.PluginV1
		expectNoTrim bool
	}{
		{
			name: "one field exceeds maxSize",
			in: func() *types.PluginV1 {
				p := utils.CloneProtoMsg(entraPlugin(t))
				p.SetStatus(&types.PluginStatusV1{
					ErrorMessage: "",
					LastRawError: strings.Repeat("A", 2500),
				})
				return p
			}(),
		},
		{
			name: "both field exceeds maxSize",
			in: func() *types.PluginV1 {
				p := utils.CloneProtoMsg(entraPlugin(t))
				p.SetStatus(&types.PluginStatusV1{
					ErrorMessage: strings.Repeat("A", 2500),
					LastRawError: strings.Repeat("A", 2500),
				})
				return p
			}(),
		},
		{
			name: "does not exceed maxSize",
			in: func() *types.PluginV1 {
				p := utils.CloneProtoMsg(entraPlugin(t))
				p.SetStatus(&types.PluginStatusV1{
					ErrorMessage: "Failed to sync, access denied.",
					LastRawError: strings.Repeat("A", 1500),
				})
				return p
			}(),
			expectNoTrim: true,
		},
		{
			name: "exceeds maxSize but has zero trimmable fields",
			in: func() *types.PluginV1 {
				p := utils.CloneProtoMsg(entraPlugin(t))
				settings := p.Spec.GetEntraId()
				// trimnming AccessGraphSettings is not supported
				settings.AccessGraphSettings = &types.PluginEntraIDAccessGraphSettings{
					AppSsoSettingsCache: []*types.PluginEntraIDAppSSOSettings{
						{
							AppId:          "test",
							FederatedSsoV2: bytes.Repeat([]byte("A"), 2000),
						},
					},
				}
				return p
			}(),
			expectNoTrim: true,
		},
		{
			name: "exceeds maxSize despite having outsized trimmable fields",
			in: func() *types.PluginV1 {
				p := utils.CloneProtoMsg(entraPlugin(t))
				settings := p.Spec.GetEntraId()
				// trimnming AccessGraphSettings is not supported
				settings.AccessGraphSettings = &types.PluginEntraIDAccessGraphSettings{
					AppSsoSettingsCache: []*types.PluginEntraIDAppSSOSettings{
						{
							AppId:          "test",
							FederatedSsoV2: bytes.Repeat([]byte("A"), 2000),
						},
					},
				}
				p.SetStatus(&types.PluginStatusV1{
					ErrorMessage: strings.Repeat("A", 2500),
					LastRawError: strings.Repeat("A", 2500),
				})
				return p
			}(),
			expectNoTrim: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := trimToMaxSize(tc.in, maxSize)

			if tc.expectNoTrim {
				require.Equal(t, tc.in.Status.LastRawError, out.GetStatus().GetLastRawError())
				require.Equal(t, tc.in.Size(), out.Size())
			} else {
				outStatus, ok := out.GetStatus().(*types.PluginStatusV1)
				require.True(t, ok, "expected plugin PluginStatusV1")
				require.LessOrEqual(t, outStatus.Size(), tc.in.Status.Size())
				require.LessOrEqual(t, out.Size(), tc.in.Size())
				require.LessOrEqual(t, out.Size(), maxSize)
			}

			if !tc.expectNoTrim && tc.in.Status.LastRawError != "" {
				require.Contains(t, out.GetStatus().GetLastRawError(), trimMsg)
			}
		})
	}
}
