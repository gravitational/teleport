/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package labels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
)

const (
	// AWSLabelNamespace is used as the namespace prefix for any labels
	// imported from AWS.
	AWSLabelNamespace = "aws"
	// AzureLabelNamespace is used as the namespace prefix for any labels
	// imported from Azure.
	AzureLabelNamespace = "azure"
	// labelUpdatePeriod is the period for updating cloud labels.
	labelUpdatePeriod = time.Hour
)

const (
	awsErrorMessage   = "Could not fetch EC2 instance's tags, please ensure 'allow instance tags in metadata' is enabled on the instance."
	azureErrorMessage = "Could not fetch Azure instance's tags."
)

// CloudConfig is the configuration for a cloud label service.
type CloudConfig struct {
	Client               cloud.InstanceMetadata
	Clock                clockwork.Clock
	Log                  logrus.FieldLogger
	namespace            string
	instanceMetadataHint string
}

func (conf *CloudConfig) checkAndSetDefaults() error {
	if conf.Client == nil {
		return trace.BadParameter("missing parameter: Client")
	}
	if conf.Clock == nil {
		conf.Clock = clockwork.NewRealClock()
	}
	if conf.Log == nil {
		conf.Log = logrus.WithField(trace.Component, "cloudlabels")
	}
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
	cloudImporter := &CloudImporter{
		CloudConfig: c,
		labels:      make(map[string]string),
		closeCh:     make(chan struct{}),
	}
	switch c.Client.GetType() {
	case types.InstanceMetadataTypeEC2:
		cloudImporter.initEC2()
	case types.InstanceMetadataTypeAzure:
		cloudImporter.initAzure()
	}
	return cloudImporter, nil
}

func (l *CloudImporter) initEC2() {
	l.namespace = AWSLabelNamespace
	l.instanceMetadataHint = awsErrorMessage
}

func (l *CloudImporter) initAzure() {
	l.namespace = AzureLabelNamespace
	l.instanceMetadataHint = azureErrorMessage
}

// Get returns the list of updated cloud labels.
func (l *CloudImporter) Get() map[string]string {
	l.muLabels.RLock()
	defer l.muLabels.RUnlock()
	labels := make(map[string]string)
	for k, v := range l.labels {
		labels[k] = v
	}
	return labels
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
		if trace.IsNotFound(err) {
			// Only show the error the first time around.
			l.instanceTagsNotFoundOnce.Do(func() {
				l.Log.Warning(l.instanceMetadataHint)
			})
			return nil
		}
		return trace.Wrap(err)
	}

	m := make(map[string]string)
	for key, value := range tags {
		if !types.IsValidLabelKey(key) {
			l.Log.Debugf("Skipping cloud tag %q, not a valid label key.", key)
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
			l.Log.Warningf("Error fetching cloud tags: %v", err)
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
