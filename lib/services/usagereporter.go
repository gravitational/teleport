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
	"github.com/gravitational/trace"

	usageevents "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// UsageAnonymizable is an event that can be anonymized.
type UsageAnonymizable interface {
	// Anonymize uses the given anonymizer to anonymize all fields in place.
	Anonymize(utils.Anonymizer) UsageAnonymizable
}

// UsageReporter is a service that accepts Teleport usage events.
type UsageReporter interface {
	// SubmitAnonymizedUsageEvent submits a usage event. The payload will be
	// anonymized by the reporter implementation.
	SubmitAnonymizedUsageEvents(event ...UsageAnonymizable) error
}

// UsageUserLogin is an event emitted when a user logs into Teleport,
// potentially via SSO.
type UsageUserLogin prehogv1.UserLoginEvent

func (u *UsageUserLogin) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUserLogin{
		UserName:      a.Anonymize([]byte(u.UserName)),
		ConnectorType: u.ConnectorType, // TODO: anonymizer connector type?
	}
}

// UsageSSOCreate is emitted when an SSO connector has been created.
type UsageSSOCreate prehogv1.SSOCreateEvent

func (u *UsageSSOCreate) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageSSOCreate{
		ConnectorType: u.ConnectorType, // TODO: anonymize connector type?
	}
}

// UsageSessionStart is an event emitted when some Teleport session has started
// (ssh, etc).
type UsageSessionStart prehogv1.SessionStartEvent

func (u *UsageSessionStart) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageSessionStart{
		UserName:    a.Anonymize([]byte(u.UserName)),
		SessionType: u.SessionType,
	}
}

// UsageResourceCreate is an event emitted when various resource types have been
// created.
type UsageResourceCreate prehogv1.ResourceCreateEvent

func (u *UsageResourceCreate) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageResourceCreate{
		ResourceType: u.ResourceType, // TODO: anonymize this?
	}
}

// UsageUIBannerClick is a UI event sent when a banner is clicked.
type UsageUIBannerClick prehogv1.UIBannerClickEvent

func (u *UsageUIBannerClick) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUIBannerClick{
		UserName: a.Anonymize([]byte(u.UserName)),
		Alert:    u.Alert,
	}
}

type UsageUIOnboardDomainNameTCSubmitEvent prehogv1.UIOnboardDomainNameTCSubmitEvent

func (u *UsageUIOnboardDomainNameTCSubmitEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	// Empty event.
	return &UsageUIOnboardDomainNameTCSubmitEvent{}
}

type UsageUIOnboardGoToDashboardClickEvent prehogv1.UIOnboardGoToDashboardClickEvent

func (u *UsageUIOnboardGoToDashboardClickEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	// Empty event.
	return &UsageUIOnboardGoToDashboardClickEvent{}
}

type UsageUIOnboardGetStartedClickEvent prehogv1.UIOnboardGetStartedClickEvent

func (u *UsageUIOnboardGetStartedClickEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUIOnboardGetStartedClickEvent{
		UserName: a.Anonymize([]byte(u.UserName)),
	}
}

type UsageUIOnboardCompleteGoToDashboardClickEvent prehogv1.UIOnboardCompleteGoToDashboardClickEvent

func (u *UsageUIOnboardCompleteGoToDashboardClickEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUIOnboardCompleteGoToDashboardClickEvent{
		UserName: a.Anonymize([]byte(u.UserName)),
	}
}

type UsageUIOnboardAddFirstResourceClickEvent prehogv1.UIOnboardAddFirstResourceClickEvent

func (u *UsageUIOnboardAddFirstResourceClickEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUIOnboardAddFirstResourceClickEvent{
		UserName: a.Anonymize([]byte(u.UserName)),
	}
}

type UsageUIOnboardAddFirstResourceLaterClickEvent prehogv1.UIOnboardAddFirstResourceLaterClickEvent

func (u *UsageUIOnboardAddFirstResourceLaterClickEvent) Anonymize(a utils.Anonymizer) UsageAnonymizable {
	return &UsageUIOnboardAddFirstResourceLaterClickEvent{
		UserName: a.Anonymize([]byte(u.UserName)),
	}
}

// ConvertUsageEvent converts a usage event from an API object into an
// anonymizable event. All events that can be submitted externally via the Auth
// API need to be defined here.
func ConvertUsageEvent(event *usageevents.UsageEventOneOf) (UsageAnonymizable, error) {
	switch e := event.GetEvent().(type) {
	case *usageevents.UsageEventOneOf_UiBannerClick:
		return &UsageUIBannerClick{
			UserName: e.UiBannerClick.UserName,
			Alert:    e.UiBannerClick.Alert,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardDomainNameTcSubmit:
		return &UsageUIOnboardDomainNameTCSubmitEvent{}, nil
	case *usageevents.UsageEventOneOf_UiOnboardGoToDashboardClick:
		return &UsageUIOnboardCompleteGoToDashboardClickEvent{}, nil
	case *usageevents.UsageEventOneOf_UiOnboardGetStartedClick:
		return &UsageUIOnboardGetStartedClickEvent{
			UserName: e.UiOnboardGetStartedClick.UserName,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick:
		return &UsageUIOnboardCompleteGoToDashboardClickEvent{
			UserName: e.UiOnboardCompleteGoToDashboardClick.UserName,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceClick:
		return &UsageUIOnboardAddFirstResourceClickEvent{
			UserName: e.UiOnboardAddFirstResourceClick.UserName,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick:
		return &UsageUIOnboardAddFirstResourceLaterClickEvent{
			UserName: e.UiOnboardAddFirstResourceLaterClick.UserName,
		}, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
