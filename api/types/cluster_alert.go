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

package types

import (
	"net/url"
	"regexp"
	"sort"
	"time"
	"unicode"

	"github.com/gravitational/trace"
)

// matchAlertLabelKey is a fairly conservative allowed charset for label keys.
var matchAlertLabelKey = regexp.MustCompile(`^[a-z0-9\.\-\/]+$`).MatchString

// matchAlertLabelVal is a slightly more permissive matcher for label values.
var matchAlertLabelVal = regexp.MustCompile(`^[a-z0-9\.\-_\/:|]+$`).MatchString

// matchAlertLabelLinkTextVal only allows alphanumeric characters and spaces.
var matchAlertLabelLinkTextVal = regexp.MustCompile(`^[a-zA-Z0-9 ]+$`).MatchString

const validLinkDestination = "goteleport.com"

type alertOptions struct {
	labels   map[string]string
	severity AlertSeverity
	created  time.Time
	expires  time.Time
}

// AlertOption is a functional option for alert construction.
type AlertOption func(options *alertOptions)

// WithAlertLabel constructs an alert with the specified label.
func WithAlertLabel(key, val string) AlertOption {
	return func(options *alertOptions) {
		if options.labels == nil {
			options.labels = make(map[string]string)
		}
		options.labels[key] = val
	}
}

// WithAlertSeverity sets the severity of an alert (defaults to MEDIUM).
func WithAlertSeverity(severity AlertSeverity) AlertOption {
	return func(options *alertOptions) {
		options.severity = severity
	}
}

// WithAlertCreated sets the alert's creation time. Auth server automatically fills
// this before inserting the alert in the backend if none is set.
func WithAlertCreated(created time.Time) AlertOption {
	return func(options *alertOptions) {
		options.created = created.UTC()
	}
}

// WithAlertExpires sets the alerts expiry time. Auth server automatically applies a
// 24h expiry before inserting the alert in the backend if none is set.
func WithAlertExpires(expires time.Time) AlertOption {
	return func(options *alertOptions) {
		options.expires = expires.UTC()
	}
}

// NewClusterAlert creates a new cluster alert.
func NewClusterAlert(name string, message string, opts ...AlertOption) (ClusterAlert, error) {
	options := alertOptions{
		severity: AlertSeverity_MEDIUM,
	}
	for _, opt := range opts {
		opt(&options)
	}
	alert := ClusterAlert{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name:    name,
				Labels:  options.labels,
				Expires: &options.expires,
			},
		},
		Spec: ClusterAlertSpec{
			Severity: options.severity,
			Message:  message,
			Created:  options.created,
		},
	}
	if err := alert.CheckAndSetDefaults(); err != nil {
		return ClusterAlert{}, trace.Wrap(err)
	}
	return alert, nil
}

// SortClusterAlerts applies the default cluster alert sorting, prioritizing
// elements by a combination of severity and creation time. Alerts are sorted
// with higher severity alerts first, and alerts of the same priority are sorted
// with newer alerts first.
func SortClusterAlerts(alerts []ClusterAlert) {
	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].Spec.Severity == alerts[j].Spec.Severity {
			return alerts[i].Spec.Created.After(alerts[j].Spec.Created)
		}
		return alerts[i].Spec.Severity > alerts[j].Spec.Severity
	})
}

func (c *ClusterAlert) setDefaults() {
	if c.Kind == "" {
		c.Kind = KindClusterAlert
	}

	if c.Version == "" {
		c.Version = V1
	}
}

// CheckAndSetDefaults verifies required fields.
func (c *ClusterAlert) CheckAndSetDefaults() error {
	c.setDefaults()
	if c.Version != V1 {
		return trace.BadParameter("unsupported cluster alert version: %s", c.Version)
	}

	if c.Kind != KindClusterAlert {
		return trace.BadParameter("expected kind %s, got %q", KindClusterAlert, c.Kind)
	}

	if c.Metadata.Name == "" {
		return trace.BadParameter("alert name must be specified")
	}

	if err := c.CheckMessage(); err != nil {
		return trace.Wrap(err)
	}

	for key, val := range c.Metadata.Labels {
		if !matchAlertLabelKey(key) {
			return trace.BadParameter("invalid alert label key: %q", key)
		}

		switch key {
		case AlertLink:
			u, err := url.Parse(val)
			if err != nil {
				return trace.BadParameter("invalid alert: label link %q is not a valid URL", val)
			}
			if u.Hostname() != validLinkDestination {
				return trace.BadParameter("invalid alert: label link not allowed %q", val)
			}
		case AlertLinkText:
			if !matchAlertLabelLinkTextVal(val) {
				return trace.BadParameter("invalid alert: label button text not allowed: %q", val)
			}
		default:
			if !matchAlertLabelVal(val) {
				// for links, we relax the conditions on label values
				return trace.BadParameter("invalid alert label value: %q", val)
			}
		}
	}
	return nil
}

func (c *ClusterAlert) CheckMessage() error {
	if c.Spec.Message == "" {
		return trace.BadParameter("alert message must be specified")
	}

	for _, c := range c.Spec.Message {
		if unicode.IsControl(c) {
			return trace.BadParameter("control characters not supported in alerts")
		}
	}
	return nil
}

// Match checks if the given cluster alert matches this query.
func (r *GetClusterAlertsRequest) Match(alert ClusterAlert) bool {
	if alert.Spec.Severity < r.Severity {
		return false
	}

	if r.AlertID != "" && r.AlertID != alert.Metadata.Name {
		return false
	}

	for key, val := range r.Labels {
		if alert.Metadata.Labels[key] != val {
			return false
		}
	}

	return true
}

func (ack *AlertAcknowledgement) Check() error {
	if ack.AlertID == "" {
		return trace.BadParameter("missing alert id in ack")
	}

	if ack.Reason == "" {
		return trace.BadParameter("ack reason must be specified")
	}

	for _, c := range ack.Reason {
		if unicode.IsControl(c) {
			return trace.BadParameter("control characters not supported in ack reason")
		}
	}

	if ack.Expires.IsZero() {
		return trace.BadParameter("missing expiry time")
	}

	return nil
}
