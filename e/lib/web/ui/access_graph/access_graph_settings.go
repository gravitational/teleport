package accessgraphui

import (
	protobuf "google.golang.org/protobuf/proto"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
)

// AccessGraphSettingsStatus holds the current status information about the Access Graph service
type AccessGraphSettingsStatus struct {
	InitialSyncComplete bool `json:"initial_sync_complete"`
	HTTPReady           bool `json:"http_ready"`
}

// AccessGraphSettings is the settings for the access graph.
type AccessGraphSettings struct {
	EnableSecretsScan bool                      `json:"enable_secrets_scan"`
	EnableDemoMode    bool                      `json:"enable_demo_mode"`
	Status            AccessGraphSettingsStatus `json:"status"`
}

// FromProtoAccessGraphSettings converts an AccessGraphSettings proto to AccessGraphSettings.
func FromProtoAccessGraphSettings(proto *clusterconfigpb.AccessGraphSettings, httpReady bool) *AccessGraphSettings {
	var enableSecretsScan bool
	if proto.GetSpec().GetSecretsScanConfig() == clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED {
		enableSecretsScan = true
	}
	var enableDemoMode bool
	if proto.GetStatus() != nil {
		enableDemoMode = proto.GetSpec().GetDemoMode() == clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED
	}
	return &AccessGraphSettings{
		EnableSecretsScan: enableSecretsScan,
		Status: AccessGraphSettingsStatus{
			InitialSyncComplete: proto.GetStatus().GetInitialSyncComplete(),
			HTTPReady:           httpReady,
		},
		EnableDemoMode: enableDemoMode,
	}
}

// UpdateProto converts AccessGraphSettings to an AccessGraphSettings proto.
func (a AccessGraphSettings) UpdateProto(msg *clusterconfigpb.AccessGraphSettings) *clusterconfigpb.AccessGraphSettings {
	if msg == nil {
		return &clusterconfigpb.AccessGraphSettings{}
	}

	proto := protobuf.Clone(msg).(*clusterconfigpb.AccessGraphSettings)
	if proto.GetSpec() == nil {
		proto.SetSpec(&clusterconfigpb.AccessGraphSettingsSpec{})
	}

	proto.GetSpec().SetSecretsScanConfig(clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED)
	if a.EnableSecretsScan {
		proto.GetSpec().SetSecretsScanConfig(clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED)
	}

	proto.GetSpec().SetDemoMode(clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_DISABLED)
	if a.EnableDemoMode {
		proto.GetSpec().SetDemoMode(clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED)
	}

	return proto
}
