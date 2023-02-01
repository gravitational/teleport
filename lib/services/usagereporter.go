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
		Event: &prehogv1.SubmitEventRequest_SessionStartV2{
			SessionStartV2: &prehogv1.SessionStartEvent{
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

// UsageUIRecoveryCodesCopyClick is a UI event sent when a user copies recovery codes.
type UsageUIRecoveryCodesCopyClick prehogv1.UIRecoveryCodesCopyClickEvent

func (u *UsageUIRecoveryCodesCopyClick) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiRecoveryCodesCopyClick{
			UiRecoveryCodesCopyClick: &prehogv1.UIRecoveryCodesCopyClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIRecoveryCodesPrintClick is a UI event sent when a user prints recovery codes.
type UsageUIRecoveryCodesPrintClick prehogv1.UIRecoveryCodesPrintClickEvent

func (u *UsageUIRecoveryCodesPrintClick) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiRecoveryCodesPrintClick{
			UiRecoveryCodesPrintClick: &prehogv1.UIRecoveryCodesPrintClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

func discoverMetadataToPrehog(u *usageevents.DiscoverMetadata, identityUsername string) *prehogv1.DiscoverMetadata {
	return &prehogv1.DiscoverMetadata{
		Id:       u.Id,
		UserName: identityUsername,
	}
}

func validateDiscoverMetadata(u *prehogv1.DiscoverMetadata) error {
	if u == nil {
		return trace.BadParameter("metadata is required")
	}

	if len(u.Id) == 0 {
		return trace.BadParameter("metadata.id is required")
	}

	return nil
}

func discoverResourceToPrehog(u *usageevents.DiscoverResourceMetadata) *prehogv1.DiscoverResourceMetadata {
	return &prehogv1.DiscoverResourceMetadata{
		Resource: prehogv1.DiscoverResource(u.Resource),
	}
}

func validateDiscoverResourceMetadata(u *prehogv1.DiscoverResourceMetadata) error {
	if u == nil {
		return trace.BadParameter("resource is required")
	}

	if u.Resource == prehogv1.DiscoverResource_DISCOVER_RESOURCE_UNSPECIFIED {
		return trace.BadParameter("invalid resource")
	}

	return nil
}

func discoverStatusToPrehog(u *usageevents.DiscoverStepStatus) *prehogv1.DiscoverStepStatus {
	return &prehogv1.DiscoverStepStatus{
		Status: prehogv1.DiscoverStatus(u.Status),
		Error:  u.Error,
	}
}

func validateDiscoverStatus(u *prehogv1.DiscoverStepStatus) error {
	if u == nil {
		return trace.BadParameter("status is required")
	}

	if u.Status == prehogv1.DiscoverStatus_DISCOVER_STATUS_UNSPECIFIED {
		return trace.BadParameter("invalid status.status")
	}

	if u.Status == prehogv1.DiscoverStatus_DISCOVER_STATUS_ERROR && len(u.Error) == 0 {
		return trace.BadParameter("status.error is required when status.status is ERROR")
	}

	return nil
}

// UsageUIDiscoverStartedEvent is a UI event sent when a user starts the Discover Wizard.
type UsageUIDiscoverStartedEvent prehogv1.UIDiscoverStartedEvent

func (u *UsageUIDiscoverStartedEvent) CheckAndSetDefaults() error {
	if err := validateDiscoverMetadata(u.Metadata); err != nil {
		return trace.Wrap(err)
	}
	if err := validateDiscoverStatus(u.Status); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *UsageUIDiscoverStartedEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverStartedEvent{
			UiDiscoverStartedEvent: &prehogv1.UIDiscoverStartedEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Status: u.Status,
			},
		},
	}
}

// UsageUIDiscoverResourceSelectionEvent is a UI event sent when a user starts the Discover Wizard.
type UsageUIDiscoverResourceSelectionEvent prehogv1.UIDiscoverResourceSelectionEvent

func (u *UsageUIDiscoverResourceSelectionEvent) CheckAndSetDefaults() error {
	if err := validateDiscoverMetadata(u.Metadata); err != nil {
		return trace.Wrap(err)
	}
	if err := validateDiscoverResourceMetadata(u.Resource); err != nil {
		return trace.Wrap(err)
	}
	if err := validateDiscoverStatus(u.Status); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
func (u *UsageUIDiscoverResourceSelectionEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
			UiDiscoverResourceSelectionEvent: &prehogv1.UIDiscoverResourceSelectionEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UsageCertificateIssued is an event emitted when a certificate has been
// issued, used to track the duration and restriction.
type UsageCertificateIssued prehogv1.UserCertificateIssuedEvent

func (u *UsageCertificateIssued) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UserCertificateIssuedEvent{
			UserCertificateIssuedEvent: &prehogv1.UserCertificateIssuedEvent{
				UserName:        a.AnonymizeString(u.UserName),
				Ttl:             u.Ttl,
				IsBot:           u.IsBot,
				UsageDatabase:   u.UsageDatabase,
				UsageApp:        u.UsageApp,
				UsageKubernetes: u.UsageKubernetes,
				UsageDesktop:    u.UsageDesktop,
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
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceClick:
		return &UsageUIOnboardAddFirstResourceClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick:
		return &UsageUIOnboardAddFirstResourceLaterClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick:
		return &UsageUIOnboardCompleteGoToDashboardClickEvent{
			UserName: e.UiOnboardCompleteGoToDashboardClick.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardSetCredentialSubmit:
		return &UsageUIOnboardSetCredentialSubmit{
			UserName: e.UiOnboardSetCredentialSubmit.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiOnboardRegisterChallengeSubmit:
		return &UsageUIOnboardRegisterChallengeSubmit{
			UserName:  e.UiOnboardRegisterChallengeSubmit.Username,
			MfaType:   e.UiOnboardRegisterChallengeSubmit.MfaType,
			LoginFlow: e.UiOnboardRegisterChallengeSubmit.LoginFlow,
		}, nil
	case *usageevents.UsageEventOneOf_UiRecoveryCodesContinueClick:
		return &UsageUIRecoveryCodesContinueClick{
			UserName: e.UiRecoveryCodesContinueClick.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiRecoveryCodesCopyClick:
		return &UsageUIRecoveryCodesCopyClick{
			UserName: e.UiRecoveryCodesCopyClick.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiRecoveryCodesPrintClick:
		return &UsageUIRecoveryCodesPrintClick{
			UserName: e.UiRecoveryCodesPrintClick.Username,
		}, nil
	case *usageevents.UsageEventOneOf_UiDiscoverStartedEvent:
		ret := &UsageUIDiscoverStartedEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverStartedEvent.Metadata, identityUsername),
			Status:   discoverStatusToPrehog(e.UiDiscoverStartedEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverResourceSelectionEvent:
		ret := &UsageUIDiscoverResourceSelectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverResourceSelectionEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverResourceSelectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverResourceSelectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
