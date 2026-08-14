package oktaplugin

import (
	"context"

	"github.com/gravitational/teleport/api/types"
)

// PluginStaticCredentialsService is a stripped down version of the OSS
// [services.PluginStaticCredentials].
type PluginStaticCredentialsService interface {
	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}
