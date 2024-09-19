package accessgraphui

import (
	protobuf "google.golang.org/protobuf/proto"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
)

// AccessGraphSettings is the settings for the access graph.
type AccessGraphSettings struct {
	EnableSecretsScan bool `json:"enable_secrets_scan"`
}

// FromProtoAccessGraphSettings converts an AccessGraphSettings proto to AccessGraphSettings.
func FromProtoAccessGraphSettings(proto *clusterconfigpb.AccessGraphSettings) *AccessGraphSettings {
	var enableSecretsScan bool
	if proto.GetSpec().GetSecretsScanConfig() == clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED {
		enableSecretsScan = true
	}
	return &AccessGraphSettings{
		EnableSecretsScan: enableSecretsScan,
	}
}

// UpdateProto converts AccessGraphSettings to an AccessGraphSettings proto.
func (a AccessGraphSettings) UpdateProto(msg *clusterconfigpb.AccessGraphSettings) *clusterconfigpb.AccessGraphSettings {
	if msg == nil {
		return &clusterconfigpb.AccessGraphSettings{}
	}

	proto := protobuf.Clone(msg).(*clusterconfigpb.AccessGraphSettings)
	if proto.GetSpec() == nil {
		proto.Spec = &clusterconfigpb.AccessGraphSettingsSpec{}
	}

	proto.Spec.SecretsScanConfig = clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED
	if a.EnableSecretsScan {
		proto.Spec.SecretsScanConfig = clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED
	}

	return proto
}
