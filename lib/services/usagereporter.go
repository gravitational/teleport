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
	"github.com/gravitational/trace"

	usageevents "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1 "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// UsageAnonymizable is an event that can be anonymized.
type UsageAnonymizable interface {
	// Anonymize uses the given anonymizer to anonymize the event and converts
	// it into a partially filled SubmitEventRequest.
	Anonymize(utils.Anonymizer) prehogv1.SubmitEventRequest
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

func (u *UsageUserLogin) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UserLogin{
			UserLogin: &prehogv1.UserLoginEvent{
				UserName:      a.AnonymizeString(u.UserName),
				ConnectorType: u.ConnectorType,
			},
		},
	}
}

// UsageSSOCreate is emitted when an SSO connector has been created.
type UsageSSOCreate prehogv1.SSOCreateEvent

func (u *UsageSSOCreate) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_SsoCreate{
			SsoCreate: &prehogv1.SSOCreateEvent{
				ConnectorType: u.ConnectorType,
			},
		},
	}
}

// UsageSessionStart is an event emitted when some Teleport session has started
// (ssh, etc).
type UsageSessionStart prehogv1.SessionStartEvent

func (u *UsageSessionStart) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_SessionStart{
			SessionStart: &prehogv1.SessionStartEvent{
				UserName:    a.AnonymizeString(u.UserName),
				SessionType: u.SessionType,
			},
		},
	}
}

// UsageResourceCreate is an event emitted when various resource types have been
// created.
type UsageResourceCreate prehogv1.ResourceCreateEvent

func (u *UsageResourceCreate) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_ResourceCreate{
			ResourceCreate: &prehogv1.ResourceCreateEvent{
				ResourceType: u.ResourceType,
			},
		},
	}
}

// UsageUIBannerClick is a UI event sent when a banner is clicked.
type UsageUIBannerClick prehogv1.UIBannerClickEvent

func (u *UsageUIBannerClick) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiBannerClick{
			UiBannerClick: &prehogv1.UIBannerClickEvent{
				UserName: a.AnonymizeString(u.UserName),
				Alert:    u.Alert,
			},
		},
	}
}

// UsageUIOnboardGetStartedClickEvent is a UI event sent when the "get started"
// button is clicked.
type UsageUIOnboardGetStartedClickEvent prehogv1.UIOnboardGetStartedClickEvent

func (u *UsageUIOnboardGetStartedClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardGetStartedClick{
			UiOnboardGetStartedClick: &prehogv1.UIOnboardGetStartedClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIOnboardCompleteGoToDashboardClickEvent is a UI event sent when
// onboarding is complete.
type UsageUIOnboardCompleteGoToDashboardClickEvent prehogv1.UIOnboardCompleteGoToDashboardClickEvent

func (u *UsageUIOnboardCompleteGoToDashboardClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardCompleteGoToDashboardClick{
			UiOnboardCompleteGoToDashboardClick: &prehogv1.UIOnboardCompleteGoToDashboardClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIOnboardAddFirstResourceClickEvent is a UI event sent when a user
// clicks the "add first resource" button.
type UsageUIOnboardAddFirstResourceClickEvent prehogv1.UIOnboardAddFirstResourceClickEvent

func (u *UsageUIOnboardAddFirstResourceClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceClick{
			UiOnboardAddFirstResourceClick: &prehogv1.UIOnboardAddFirstResourceClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIOnboardAddFirstResourceLaterClickEvent is a UI event sent when a user
// clicks the "add first resource later" button.
type UsageUIOnboardAddFirstResourceLaterClickEvent prehogv1.UIOnboardAddFirstResourceLaterClickEvent

func (u *UsageUIOnboardAddFirstResourceLaterClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceLaterClick{
			UiOnboardAddFirstResourceLaterClick: &prehogv1.UIOnboardAddFirstResourceLaterClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIOnboardSetCredentialSubmit is an UI event sent during registration
// when the user configures login credentials.
type UsageUIOnboardSetCredentialSubmit prehogv1.UIOnboardSetCredentialSubmitEvent

func (u *UsageUIOnboardSetCredentialSubmit) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &prehogv1.UIOnboardSetCredentialSubmitEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIOnboardRegisterChallengeSubmit is a UI event sent during registration
// when the MFA challenge is completed.
type UsageUIOnboardRegisterChallengeSubmit prehogv1.UIOnboardRegisterChallengeSubmitEvent

func (u *UsageUIOnboardRegisterChallengeSubmit) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardRegisterChallengeSubmit{
			UiOnboardRegisterChallengeSubmit: &prehogv1.UIOnboardRegisterChallengeSubmitEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIRecoveryCodesContinueClick is a UI event sent when a user configures recovery codes.
type UsageUIRecoveryCodesContinueClick prehogv1.UIRecoveryCodesContinueClickEvent

func (u *UsageUIRecoveryCodesContinueClick) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &prehogv1.UIRecoveryCodesContinueClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// ConvertUsageEvent converts a usage event from an API object into an
// anonymizable event. All events that can be submitted externally via the Auth
// API need to be defined here.
func ConvertUsageEvent(event *usageevents.UsageEventOneOf, identityUsername string) (UsageAnonymizable, error) {
	// Note: events (especially pre-registration) that embed a username of their
	// own should generally pass that through rather than using the identity
	// username provided to the function. It may be the username of a Teleport
	// component (e.g. proxy) rather than the end user.

	switch e := event.GetEvent().(type) {
	case *usageevents.UsageEventOneOf_UiBannerClick:
		return &UsageUIBannerClick{
			UserName: identityUsername,
			Alert:    e.UiBannerClick.Alert,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardGetStartedClick:
		return &UsageUIOnboardGetStartedClickEvent{
			UserName: e.UiOnboardGetStartedClick.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick:
		return &UsageUIOnboardCompleteGoToDashboardClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceClick:
		return &UsageUIOnboardAddFirstResourceClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick:
		return &UsageUIOnboardAddFirstResourceLaterClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardSetCredentialSubmit:
		return &UsageUIOnboardSetCredentialSubmit{
			UserName: e.UiOnboardSetCredentialSubmit.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardRegisterChallengeSubmit:
		return &UsageUIOnboardRegisterChallengeSubmit{
			UserName: e.UiOnboardRegisterChallengeSubmit.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiRecoveryCodesContinueClick:
		return &UsageUIRecoveryCodesContinueClick{
			UserName: e.UiRecoveryCodesContinueClick.Username,
		}, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
