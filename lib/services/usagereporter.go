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

package services

import (
	prehogv1 "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// UsageAnonymizable is an event that can be anonymized.
type UsageAnonymizable interface {
	// Anonymize uses the given anonymizer to anonymize all fields in place.
	Anonymize(utils.Anonymizer)
}

// UsageReporter is a service that accepts Teleport usage events.
type UsageReporter interface {
	// SubmitAnonymizedUsageEvent submits a usage event. The payload will be
	// anonymized by the reporter implementation.
	SubmitAnonymizedUsageEvents(event ...UsageAnonymizable) error
}

type UsageUserLogin prehogv1.UserLoginEvent

func (l *UsageUserLogin) Anonymize(a utils.Anonymizer) {
	l.UserName = a.Anonymize([]byte(l.UserName))

	// TODO: anonymizer connector type?
}

type UsageSSOCreate prehogv1.SSOCreateEvent

func (c *UsageSSOCreate) Anonymize(a utils.Anonymizer) {
	// TODO: anonymize connector type?
}

// type UsageSessionStart api.SessionCreateRequest
type UsageSessionStart prehogv1.SessionStartEvent

func (c *UsageSessionStart) Anonymize(a utils.Anonymizer) {
	c.UserName = a.Anonymize([]byte(c.UserName))

	// TODO: anonymize session type?
}
