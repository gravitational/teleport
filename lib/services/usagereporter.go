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
	usageevents "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
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

func (u *UsageUserLogin) Anonymize(a utils.Anonymizer) {
	u.UserName = a.Anonymize([]byte(u.UserName))

	// TODO: anonymizer connector type?
}

type UsageSSOCreate prehogv1.SSOCreateEvent

func (u *UsageSSOCreate) Anonymize(a utils.Anonymizer) {
	// TODO: anonymize connector type?
}

type UsageSessionStart prehogv1.SessionStartEvent

func (u *UsageSessionStart) Anonymize(a utils.Anonymizer) {
	u.UserName = a.Anonymize([]byte(u.UserName))

	// TODO: anonymize session type?
}

type UsageUpgradeBannerClickEvent prehogv1.UpgradeBannerClickEvent

func (u *UsageUpgradeBannerClickEvent) Anonymize(a utils.Anonymizer) {
	// Event is empty, nothing to do.
}

func ConvertUsageEvent(event *usageevents.UsageEventOneOf) (UsageAnonymizable, error) {
	switch event.GetEvent().(type) {
	case *usageevents.UsageEventOneOf_UpgradeBannerClick:
		return &UsageUpgradeBannerClickEvent{}, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
