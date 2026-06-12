/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package labels

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/imds"
)

const (
	// AWSLabelNamespace is used as the namespace prefix for any labels
	// imported from AWS.
	AWSLabelNamespace = "aws"
	// AzureLabelNamespace is used as the namespace prefix for any labels
	// imported from Azure.
	AzureLabelNamespace = "azure"
	// GCPLabelNamespace is used as the namespace prefix for any labels imported
	// from GCP.
	GCPLabelNamespace = "gcp"
	// OracleLabelNamespace is used as the namespace prefix for any labels
	// imported from Oracle Cloud.
	OracleLabelNamespace = "oracle"
	// labelUpdatePeriod is the period for updating cloud labels.
	labelUpdatePeriod = time.Hour
)

const (
	awsErrorMessage    = "Could not fetch EC2 instance's tags, please ensure 'allow instance tags in metadata' is enabled on the instance."
	azureErrorMessage  = "Could not fetch Azure instance's tags."
	gcpErrorMessage    = "Could not fetch GCP instance's labels, please ensure instance's service principal has read access to instances."
	oracleErrorMessage = "Could not fetch Oracle Cloud instance's tags."
)

// CloudConfig is the configuration for a cloud label service.
type CloudConfig struct {
	Client               imds.Client
	Clock                clockwork.Clock
	Log                  *slog.Logger
	namespace            string
	instanceMetadataHint string
}

func (conf *CloudConfig) checkAndSetDefaults() error {
	if conf.Client == nil {
		return trace.BadParameter("missing parameter: Client")
	}
	switch conf.Client.GetType() {
	case types.InstanceMetadataTypeEC2:
		conf.namespace = AWSLabelNamespace
		conf.instanceMetadataHint = awsErrorMessage
	case types.InstanceMetadataTypeAzure:
		conf.namespace = AzureLabelNamespace
		conf.instanceMetadataHint = azureErrorMessage
	case types.InstanceMetadataTypeGCP:
		conf.namespace = GCPLabelNamespace
		conf.instanceMetadataHint = gcpErrorMessage
	case types.InstanceMetadataTypeOracle:
		conf.namespace = OracleLabelNamespace
		conf.instanceMetadataHint = oracleErrorMessage
	default:
		return trace.BadParameter("invalid client type: %v", conf.Client.GetType())
	}

	conf.Clock = cmp.Or(conf.Clock, clockwork.NewRealClock())
	conf.Log = cmp.Or(conf.Log, slog.With(teleport.ComponentKey, "cloudlabels"))
	return nil
}

// CloudImporter is a service that periodically imports tags from a cloud service via instance
// metadata.
type CloudImporter struct {
	*CloudConfig
	muLabels sync.RWMutex
	labels   map[string]string

	closeCh chan struct{}

	// instanceTagsNotFoundOnce is used to ensure that the error message for
	// incorrectly configured instance metadata is only logged once.
	instanceTagsNotFoundOnce sync.Once
}

// NewCloudImporter creates a new cloud label importer.
func NewCloudImporter(ctx context.Context, c *CloudConfig) (*CloudImporter, error) {
	if err := c.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CloudImporter{
		CloudConfig: c,
		labels:      make(map[string]string),
		closeCh:     make(chan struct{}),
	}, nil
}

// Get returns the list of updated cloud labels.
func (l *CloudImporter) Get() map[string]string {
	l.muLabels.RLock()
	defer l.muLabels.RUnlock()

	return maps.Clone(l.labels)
}

// Apply adds cloud labels to the provided resource.
func (l *CloudImporter) Apply(r types.ResourceWithLabels) {
	labels := l.Get()
	for k, v := range r.GetStaticLabels() {
		labels[k] = v
	}
	r.SetStaticLabels(labels)
}

// Sync will block and synchronously update cloud labels.
func (l *CloudImporter) Sync(ctx context.Context) error {
	tags, err := l.Client.GetTags(ctx)
	if err != nil {
		if trace.IsNotFound(err) || trace.IsAccessDenied(err) {
			// Only show the error the first time around.
			l.instanceTagsNotFoundOnce.Do(func() {
				l.Log.WarnContext(ctx, l.instanceMetadataHint) //nolint:sloglint // message should be a constant but in this case we are creating it at runtime.
			})
			return nil
		}
		return trace.Wrap(err)
	}

	m := make(map[string]string)
	for key, value := range tags {
		if !types.IsValidLabelKey(key) {
			l.Log.DebugContext(ctx, "Skipping cloud tag due to invalid label key", "tag", key)
			continue
		}
		m[FormatCloudLabelKey(l.namespace, key)] = value
	}

	l.muLabels.Lock()
	defer l.muLabels.Unlock()
	l.labels = m
	return nil
}

// Start will start a loop that continually keeps cloud labels updated.
func (l *CloudImporter) Start(ctx context.Context) {
	go l.periodicUpdateLabels(ctx)
}

func (l *CloudImporter) periodicUpdateLabels(ctx context.Context) {
	ticker := l.Clock.NewTicker(labelUpdatePeriod)
	defer ticker.Stop()

	for {
		if err := l.Sync(ctx); err != nil {
			l.Log.WarnContext(ctx, "Failed to fetch cloud tags", "error", err)
		}
		select {
		case <-ticker.Chan():
		case <-ctx.Done():
			return
		}
	}
}

// FormatCloudLabelKey formats label keys coming from a cloud instance.
func FormatCloudLabelKey(namespace, key string) string {
	return fmt.Sprintf("%s/%s", namespace, key)
}
