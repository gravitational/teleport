package plugins

import (
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// labelName is the name of the plugin instance
	labelName = "name"
	// labelPluginType is the type of the plugin, corresponding to api/types.PluginType
	labelPluginType = "type"
	// labelStatus is the plugin status code, corresponding to stringified api/types.PluginStatusCode
	labelStatus = "status"
)

var statuses = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricHostedPluginStatus,
		Help:      "Hosted plugin status",
	},
	[]string{labelName, labelPluginType, labelStatus},
)

// statusSink is an implementation of common.StatusSink
// using teleport Plugin resource's `Status` field
//
// Plugin instances use this to report their "status"
// (either "running", i.e. everything is normal, or "error").
// The status is then exposed to the users via the Plugin resource via the web UI,
// and to administrators via the Prometheus metrics.
type statusSink struct {
	service     services.Plugins
	name        string
	pluginType  string
	retryConfig retryutils.LinearConfig
}

// newStatusSink creates a status sink based on the plugin type.
// If the plugin type is AWS IC, it returns an icPluginSink with custom behavior.
// Otherwise, it returns a standard statusSink.
func newStatusSink(service services.Plugins, name string, pluginType string) common.StatusSink {
	retryConfig := retryutils.LinearConfig{
		Step: 100 * time.Millisecond,
		Max:  1 * time.Second,
	}
	sink := &statusSink{
		service:     service,
		name:        name,
		pluginType:  pluginType,
		retryConfig: retryConfig,
	}
	if pluginType == types.PluginTypeAWSIdentityCenter {
		return &icPluginSink{
			pluginsService: service,
			sink:           sink,
		}
	}

	return sink
}

// icPluginSink is a specialized status sink for AWS Identity Center plugin.
// It wraps the base status sink and adds custom logic for AWS IC.
type icPluginSink struct {
	pluginsService services.Plugins
	sink           *statusSink
}

// Emit sends the plugin status, applying custom logic for AWS IC plugin.
// If the status detail field is nil, an existing detail status will be applied.
func (s *icPluginSink) Emit(ctx context.Context, status types.PluginStatus) error {
	newStatus := &types.PluginStatusV1{
		Code:         status.GetCode(),
		ErrorMessage: status.GetErrorMessage(),
	}
	if status.GetAwsIc() != nil {
		newStatus.Details = &types.PluginStatusV1_AwsIc{
			AwsIc: status.GetAwsIc(),
		}
	} else {
		plugin, err := s.pluginsService.GetPlugin(ctx, types.PluginTypeAWSIdentityCenter, false /* withSecrets */)
		if err != nil {
			return trace.Wrap(err)
		}
		newStatus.Details = &types.PluginStatusV1_AwsIc{
			AwsIc: plugin.GetStatus().GetAwsIc(),
		}
	}

	return s.sink.Emit(ctx, newStatus)
}

func (s *statusSink) Emit(ctx context.Context, status types.PluginStatus) error {
	retry, err := retryutils.NewLinear(s.retryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	// This timeout caps the total duration of retry attempts.
	// It is chosen to be relatively long to allow for several attempts
	// (although serious write contention on this resource is not expected)
	// but short enough to not interfere with plugin's responsiveness.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = retry.For(ctx, func() error {
		return s.service.SetPluginStatus(ctx, s.name, status)
	})
	if err != nil {
		return trace.Wrap(err, "error setting status for plugin %q", s.name)
	}

	// Plugin can only be of one status at the time.
	// If we change the status (e.g. from "running" to "error"),
	// we need to clear any previous status metrics for this plugin.
	statuses.DeletePartialMatch(prometheus.Labels{
		labelName: s.name,
	})

	statuses.With(prometheus.Labels{
		labelName:       s.name,
		labelPluginType: s.pluginType,
		labelStatus:     strings.ToLower(status.GetCode().String()),
	}).Set(1)
	return nil
}
