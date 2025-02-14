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

package usagereporter

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// Anonymizable is an event that can be anonymized.
type Anonymizable interface {
	// Anonymize uses the given anonymizer to anonymize the event and converts
	// it into a partially filled SubmitEventRequest.
	Anonymize(utils.Anonymizer) prehogv1a.SubmitEventRequest
}

// UserLoginEvent is an event emitted when a user logs into Teleport,
// potentially via SSO.
type UserLoginEvent prehogv1a.UserLoginEvent

func (u *UserLoginEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	var deviceID string
	if u.DeviceId != "" {
		deviceID = a.AnonymizeString(u.DeviceId)
	}
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UserLogin{
			UserLogin: &prehogv1a.UserLoginEvent{
				UserName:                 a.AnonymizeString(u.UserName),
				ConnectorType:            u.ConnectorType,
				DeviceId:                 deviceID,
				RequiredPrivateKeyPolicy: u.RequiredPrivateKeyPolicy,
			},
		},
	}
}

// AccessRequestCreateEvent is emitted when Access Request is created.
type AccessRequestCreateEvent prehogv1a.AccessRequestEvent

func (e *AccessRequestCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessRequestCreateEvent{
			AccessRequestCreateEvent: &prehogv1a.AccessRequestEvent{
				UserName: a.AnonymizeString(e.UserName),
			},
		},
	}
}

// AccessRequestCreateEvent is emitted when Access Request is reviewed.
type AccessRequestReviewEvent prehogv1a.AccessRequestEvent

func (e *AccessRequestReviewEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessRequestReviewEvent{
			AccessRequestReviewEvent: &prehogv1a.AccessRequestEvent{
				UserName: a.AnonymizeString(e.UserName),
			},
		},
	}
}

// BotJoinEvent is an event emitted when a user logs into Teleport,
// potentially via SSO.
type BotJoinEvent prehogv1a.BotJoinEvent

func (u *BotJoinEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_BotJoin{
			BotJoin: &prehogv1a.BotJoinEvent{
				BotName:       a.AnonymizeString(u.BotName),
				JoinTokenName: a.AnonymizeString(u.JoinTokenName),
				JoinMethod:    u.JoinMethod,
				UserName:      a.AnonymizeString(u.UserName),
				BotInstanceId: a.AnonymizeString(u.BotInstanceId),
			},
		},
	}
}

// SSOCreateEvent is emitted when an SSO connector has been created.
type SSOCreateEvent prehogv1a.SSOCreateEvent

func (u *SSOCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SsoCreate{
			SsoCreate: &prehogv1a.SSOCreateEvent{
				ConnectorType: u.ConnectorType,
			},
		},
	}
}

// SessionStartEvent is an event emitted when some Teleport session has started
// (ssh, etc).
type SessionStartEvent prehogv1a.SessionStartEvent

func (u *SessionStartEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	sessionStart := &prehogv1a.SessionStartEvent{
		UserName:    a.AnonymizeString(u.UserName),
		SessionType: u.SessionType,
		UserKind:    u.UserKind,
	}
	if u.Database != nil {
		sessionStart.Database = &prehogv1a.SessionStartDatabaseMetadata{
			DbType:     u.Database.DbType,
			DbProtocol: u.Database.DbProtocol,
			DbOrigin:   u.Database.DbOrigin,
			UserAgent:  u.Database.UserAgent,
		}
	}
	if u.Desktop != nil {
		sessionStart.Desktop = &prehogv1a.SessionStartDesktopMetadata{
			DesktopType:       u.Desktop.DesktopType,
			Origin:            u.Desktop.Origin,
			WindowsDomain:     a.AnonymizeString(u.Desktop.WindowsDomain),
			AllowUserCreation: u.Desktop.AllowUserCreation,
			Nla:               u.Desktop.Nla,
		}
	}
	if u.App != nil {
		sessionStart.App = &prehogv1a.SessionStartAppMetadata{
			IsMultiPort: u.App.IsMultiPort,
		}
	}
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SessionStartV2{
			SessionStartV2: sessionStart,
		},
	}
}

// ResourceCreateEvent is an event emitted when various resource types have been
// created.
type ResourceCreateEvent prehogv1a.ResourceCreateEvent

func (u *ResourceCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	event := &prehogv1a.ResourceCreateEvent{
		ResourceType:   u.ResourceType,
		ResourceOrigin: u.ResourceOrigin,
		CloudProvider:  u.CloudProvider,
	}
	if db := u.Database; db != nil {
		event.Database = &prehogv1a.DiscoveredDatabaseMetadata{
			DbType:     db.DbType,
			DbProtocol: db.DbProtocol,
		}
	}
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_ResourceCreate{
			ResourceCreate: event,
		},
	}
}

func integrationEnrollMetadataToPrehog(u *usageeventsv1.IntegrationEnrollMetadata, userMD UserMetadata) *prehogv1a.IntegrationEnrollMetadata {
	// Some enums are out of sync and need to be mapped manually
	var prehogKind prehogv1a.IntegrationEnrollKind
	switch u.Kind {
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_SERVICENOW:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_SERVICENOW
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_ENTRA_ID:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_ENTRA_ID
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_DATADOG_INCIDENT_MANAGEMENT:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_DATADOG_INCIDENT_MANAGEMENT
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_AWS:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_AWS
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_GCP:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_GCP
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_AZURE:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_AZURE
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_SPACELIFT:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_SPACELIFT
	case usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_KUBERNETES:
		prehogKind = prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_MACHINE_ID_KUBERNETES
	default:
		prehogKind = prehogv1a.IntegrationEnrollKind(u.Kind)
	}
	return &prehogv1a.IntegrationEnrollMetadata{
		Id:       u.Id,
		Kind:     prehogKind,
		UserName: userMD.Username,
	}
}

func validateIntegrationEnrollMetadata(u *prehogv1a.IntegrationEnrollMetadata) error {
	if u == nil {
		return trace.BadParameter("metadata is required")
	}

	if len(u.UserName) == 0 {
		return trace.BadParameter("metadata.user_name is required")
	}

	if len(u.Id) == 0 {
		return trace.BadParameter("metadata.id is required")
	}

	if u.Kind == prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_UNSPECIFIED {
		return trace.BadParameter("metadata.kind is required")
	}

	return nil
}

// UIIntegrationEnrollStartEvent is a UI event sent when a user starts enrolling a integration.
type UIIntegrationEnrollStartEvent prehogv1a.UIIntegrationEnrollStartEvent

func (u *UIIntegrationEnrollStartEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateIntegrationEnrollMetadata(u.Metadata))
}

func (u *UIIntegrationEnrollStartEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollStartEvent{
			UiIntegrationEnrollStartEvent: &prehogv1a.UIIntegrationEnrollStartEvent{
				Metadata: &prehogv1a.IntegrationEnrollMetadata{
					Id:       u.Metadata.Id,
					Kind:     u.Metadata.Kind,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
			},
		},
	}
}

// UIIntegrationEnrollCompleteEvent is a UI event sent when a user completes enrolling an integration.
type UIIntegrationEnrollCompleteEvent prehogv1a.UIIntegrationEnrollCompleteEvent

func (u *UIIntegrationEnrollCompleteEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateIntegrationEnrollMetadata(u.Metadata))
}

func (u *UIIntegrationEnrollCompleteEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollCompleteEvent{
			UiIntegrationEnrollCompleteEvent: &prehogv1a.UIIntegrationEnrollCompleteEvent{
				Metadata: &prehogv1a.IntegrationEnrollMetadata{
					Id:       u.Metadata.Id,
					Kind:     u.Metadata.Kind,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
			},
		},
	}
}

// UIIntegrationEnrollStepEvent is a UI event sent for the specified configuration step in a
// given integration enrollment flow.
type UIIntegrationEnrollStepEvent prehogv1a.UIIntegrationEnrollStepEvent

func (u *UIIntegrationEnrollStepEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateIntegrationEnrollMetadata(u.Metadata))
}

func (u *UIIntegrationEnrollStepEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollStepEvent{
			UiIntegrationEnrollStepEvent: &prehogv1a.UIIntegrationEnrollStepEvent{
				Metadata: &prehogv1a.IntegrationEnrollMetadata{
					Id:       u.Metadata.Id,
					Kind:     u.Metadata.Kind,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Step: u.Step,
				Status: &prehogv1a.IntegrationEnrollStepStatus{
					Code:  u.Status.GetCode(),
					Error: u.Status.GetError(),
				},
			},
		},
	}
}

// UIBannerClickEvent is a UI event sent when a banner is clicked.
type UIBannerClickEvent prehogv1a.UIBannerClickEvent

func (u *UIBannerClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiBannerClick{
			UiBannerClick: &prehogv1a.UIBannerClickEvent{
				UserName: a.AnonymizeString(u.UserName),
				Alert:    u.Alert,
			},
		},
	}
}

// UIOnboardCompleteGoToDashboardClickEvent is a UI event sent when
// onboarding is complete.
type UIOnboardCompleteGoToDashboardClickEvent prehogv1a.UIOnboardCompleteGoToDashboardClickEvent

func (u *UIOnboardCompleteGoToDashboardClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardCompleteGoToDashboardClick{
			UiOnboardCompleteGoToDashboardClick: &prehogv1a.UIOnboardCompleteGoToDashboardClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardAddFirstResourceClickEvent is a UI event sent when a user
// clicks the "add first resource" button.
type UIOnboardAddFirstResourceClickEvent prehogv1a.UIOnboardAddFirstResourceClickEvent

func (u *UIOnboardAddFirstResourceClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardAddFirstResourceClick{
			UiOnboardAddFirstResourceClick: &prehogv1a.UIOnboardAddFirstResourceClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardAddFirstResourceLaterClickEvent is a UI event sent when a user
// clicks the "add first resource later" button.
type UIOnboardAddFirstResourceLaterClickEvent prehogv1a.UIOnboardAddFirstResourceLaterClickEvent

func (u *UIOnboardAddFirstResourceLaterClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardAddFirstResourceLaterClick{
			UiOnboardAddFirstResourceLaterClick: &prehogv1a.UIOnboardAddFirstResourceLaterClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardSetCredentialSubmitEvent is an UI event sent during registration
// when the user configures login credentials.
type UIOnboardSetCredentialSubmitEvent prehogv1a.UIOnboardSetCredentialSubmitEvent

func (u *UIOnboardSetCredentialSubmitEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &prehogv1a.UIOnboardSetCredentialSubmitEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardQuestionnaireSubmitEvent is a UI event sent during registration when
// user submit their onboarding questionnaire.
type UIOnboardQuestionnaireSubmitEvent prehogv1a.UIOnboardQuestionnaireSubmitEvent

func (u *UIOnboardQuestionnaireSubmitEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardQuestionnaireSubmit{
			UiOnboardQuestionnaireSubmit: &prehogv1a.UIOnboardQuestionnaireSubmitEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardRegisterChallengeSubmitEvent is a UI event sent during registration
// when the MFA challenge is completed.
type UIOnboardRegisterChallengeSubmitEvent prehogv1a.UIOnboardRegisterChallengeSubmitEvent

func (u *UIOnboardRegisterChallengeSubmitEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiOnboardRegisterChallengeSubmit{
			UiOnboardRegisterChallengeSubmit: &prehogv1a.UIOnboardRegisterChallengeSubmitEvent{
				UserName:  a.AnonymizeString(u.UserName),
				MfaType:   u.MfaType,
				LoginFlow: u.LoginFlow,
			},
		},
	}
}

// UIRecoveryCodesContinueClickEvent is a UI event sent when a user configures recovery codes.
type UIRecoveryCodesContinueClickEvent prehogv1a.UIRecoveryCodesContinueClickEvent

func (u *UIRecoveryCodesContinueClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &prehogv1a.UIRecoveryCodesContinueClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIRecoveryCodesCopyClickEvent is a UI event sent when a user copies recovery codes.
type UIRecoveryCodesCopyClickEvent prehogv1a.UIRecoveryCodesCopyClickEvent

func (u *UIRecoveryCodesCopyClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiRecoveryCodesCopyClick{
			UiRecoveryCodesCopyClick: &prehogv1a.UIRecoveryCodesCopyClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUIRecoveryCodesPrintClick is a UI event sent when a user prints recovery codes.
type UsageUIRecoveryCodesPrintClick prehogv1a.UIRecoveryCodesPrintClickEvent

func (u *UsageUIRecoveryCodesPrintClick) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiRecoveryCodesPrintClick{
			UiRecoveryCodesPrintClick: &prehogv1a.UIRecoveryCodesPrintClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// RoleCreateEvent is an event emitted when a custom role is created.
type RoleCreateEvent prehogv1a.RoleCreateEvent

func (u *RoleCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	role := u.RoleName
	if !slices.Contains(teleport.PresetRoles, u.RoleName) {
		role = a.AnonymizeString(u.RoleName)
	}

	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_RoleCreate{
			RoleCreate: &prehogv1a.RoleCreateEvent{
				UserName: a.AnonymizeString(u.UserName),
				RoleName: role,
			},
		},
	}
}

// BotCreateEvent is an event emitted when a Machine ID bot is created.
type BotCreateEvent prehogv1a.BotCreateEvent

func (u *BotCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_BotCreate{
			BotCreate: &prehogv1a.BotCreateEvent{
				UserName:    a.AnonymizeString(u.UserName),
				RoleName:    a.AnonymizeString(u.RoleName),
				BotUserName: a.AnonymizeString(u.BotUserName),
				BotName:     a.AnonymizeString(u.BotName),
				RoleCount:   u.RoleCount,
				JoinMethod:  u.JoinMethod,
			},
		},
	}
}

// UICreateNewRoleClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleClickEvent prehogv1a.UICreateNewRoleClickEvent

func (u *UICreateNewRoleClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiCreateNewRoleClick{
			UiCreateNewRoleClick: &prehogv1a.UICreateNewRoleClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleSaveClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleSaveClickEvent prehogv1a.UICreateNewRoleSaveClickEvent

func (u *UICreateNewRoleSaveClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiCreateNewRoleSaveClick{
			UiCreateNewRoleSaveClick: &prehogv1a.UICreateNewRoleSaveClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleCancelClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleCancelClickEvent prehogv1a.UICreateNewRoleCancelClickEvent

func (u *UICreateNewRoleCancelClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiCreateNewRoleCancelClick{
			UiCreateNewRoleCancelClick: &prehogv1a.UICreateNewRoleCancelClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleViewDocumentationClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleViewDocumentationClickEvent prehogv1a.UICreateNewRoleViewDocumentationClickEvent

func (u *UICreateNewRoleViewDocumentationClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiCreateNewRoleViewDocumentationClick{
			UiCreateNewRoleViewDocumentationClick: &prehogv1a.UICreateNewRoleViewDocumentationClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICallToActionClickEvent is a UI event sent when a user prints recovery codes.
type UICallToActionClickEvent prehogv1a.UICallToActionClickEvent

func (u *UICallToActionClickEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiCallToActionClickEvent{
			UiCallToActionClickEvent: &prehogv1a.UICallToActionClickEvent{
				Cta:      u.Cta,
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UserCertificateIssuedEvent is an event emitted when a certificate has been
// issued, used to track the duration and restriction.
type UserCertificateIssuedEvent prehogv1a.UserCertificateIssuedEvent

func (u *UserCertificateIssuedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	e := &prehogv1a.UserCertificateIssuedEvent{
		UserName:         a.AnonymizeString(u.UserName),
		Ttl:              u.Ttl,
		IsBot:            u.IsBot,
		UsageDatabase:    u.UsageDatabase,
		UsageApp:         u.UsageApp,
		UsageKubernetes:  u.UsageKubernetes,
		UsageDesktop:     u.UsageDesktop,
		PrivateKeyPolicy: u.PrivateKeyPolicy,
	}
	if u.BotInstanceId != "" {
		e.BotInstanceId = a.AnonymizeString(u.BotInstanceId)
	}
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UserCertificateIssuedEvent{
			UserCertificateIssuedEvent: e,
		},
	}
}

// KubeRequestEvent is an event emitted when a Kubernetes API request is
// handled.
type KubeRequestEvent prehogv1a.KubeRequestEvent

func (u *KubeRequestEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_KubeRequest{
			KubeRequest: &prehogv1a.KubeRequestEvent{
				UserName: a.AnonymizeString(u.UserName),
				UserKind: u.UserKind,
			},
		},
	}
}

// SFTPEvent is an event emitted for each file operation in a SFTP connection.
type SFTPEvent prehogv1a.SFTPEvent

func (u *SFTPEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_Sftp{
			Sftp: &prehogv1a.SFTPEvent{
				UserName: a.AnonymizeString(u.UserName),
				UserKind: u.UserKind,
				Action:   u.Action,
			},
		},
	}
}

// AgentMetadataEvent is an event emitted after an agent first connects to the auth server.
type AgentMetadataEvent prehogv1a.AgentMetadataEvent

func (u *AgentMetadataEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AgentMetadataEvent{
			AgentMetadataEvent: &prehogv1a.AgentMetadataEvent{
				Version:               u.Version,
				HostId:                a.AnonymizeString(u.HostId),
				Services:              u.Services,
				Os:                    u.Os,
				OsVersion:             u.OsVersion,
				HostArchitecture:      u.HostArchitecture,
				GlibcVersion:          u.GlibcVersion,
				InstallMethods:        u.InstallMethods,
				ContainerRuntime:      u.ContainerRuntime,
				ContainerOrchestrator: u.ContainerOrchestrator,
				CloudEnvironment:      u.CloudEnvironment,
				ExternalUpgrader:      u.ExternalUpgrader,
			},
		},
	}
}

type ResourceKind = prehogv1a.ResourceKind

const (
	ResourceKindNode            = prehogv1a.ResourceKind_RESOURCE_KIND_NODE
	ResourceKindAppServer       = prehogv1a.ResourceKind_RESOURCE_KIND_APP_SERVER
	ResourceKindKubeServer      = prehogv1a.ResourceKind_RESOURCE_KIND_KUBE_SERVER
	ResourceKindDBServer        = prehogv1a.ResourceKind_RESOURCE_KIND_DB_SERVER
	ResourceKindWindowsDesktop  = prehogv1a.ResourceKind_RESOURCE_KIND_WINDOWS_DESKTOP
	ResourceKindNodeOpenSSH     = prehogv1a.ResourceKind_RESOURCE_KIND_NODE_OPENSSH
	ResourceKindNodeOpenSSHEICE = prehogv1a.ResourceKind_RESOURCE_KIND_NODE_OPENSSH_EICE
)

func ResourceKindFromKeepAliveType(t types.KeepAlive_KeepAliveType) ResourceKind {
	switch t {
	case types.KeepAlive_NODE:
		return ResourceKindNode
	case types.KeepAlive_APP:
		return ResourceKindAppServer
	case types.KeepAlive_KUBERNETES:
		return ResourceKindKubeServer
	case types.KeepAlive_DATABASE:
		return ResourceKindDBServer
	default:
		return 0
	}
}

type ResourceHeartbeatEvent struct {
	Name   string
	Kind   prehogv1a.ResourceKind
	Static bool
}

func (u *ResourceHeartbeatEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_ResourceHeartbeat{
			ResourceHeartbeat: &prehogv1a.ResourceHeartbeatEvent{
				ResourceName: a.AnonymizeNonEmpty(u.Name),
				ResourceKind: u.Kind,
				Static:       u.Static,
			},
		},
	}
}

// AssistCompletionEvent is an event emitted after each completion by the Assistant
type AssistCompletionEvent prehogv1a.AssistCompletionEvent

func (e *AssistCompletionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AssistCompletion{
			AssistCompletion: &prehogv1a.AssistCompletionEvent{
				UserName:         a.AnonymizeString(e.UserName),
				ConversationId:   e.ConversationId,
				TotalTokens:      e.TotalTokens,
				PromptTokens:     e.PromptTokens,
				CompletionTokens: e.CompletionTokens,
			},
		},
	}
}

// EditorChangeEvent is an event emitted when the default editor is added or removed to an user
type EditorChangeEvent prehogv1a.EditorChangeEvent

func (e *EditorChangeEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_EditorChangeEvent{
			EditorChangeEvent: &prehogv1a.EditorChangeEvent{
				UserName: a.AnonymizeString(e.UserName),
				Status:   e.Status,
			},
		},
	}
}

type AssistExecutionEvent prehogv1a.AssistExecutionEvent

func (e *AssistExecutionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AssistExecution{
			AssistExecution: &prehogv1a.AssistExecutionEvent{
				UserName:         a.AnonymizeString(e.UserName),
				ConversationId:   e.ConversationId,
				NodeCount:        e.NodeCount,
				TotalTokens:      e.TotalTokens,
				PromptTokens:     e.PromptTokens,
				CompletionTokens: e.CompletionTokens,
			},
		},
	}
}

type AssistNewConversationEvent prehogv1a.AssistNewConversationEvent

func (e *AssistNewConversationEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AssistNewConversation{
			AssistNewConversation: &prehogv1a.AssistNewConversationEvent{
				UserName: a.AnonymizeString(e.UserName),
				Category: e.Category,
			},
		},
	}
}

type AssistAccessRequestEvent prehogv1a.AssistAccessRequestEvent

// Anonymize anonymizes the event.
func (e *AssistAccessRequestEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AssistAccessRequest{
			AssistAccessRequest: &prehogv1a.AssistAccessRequestEvent{
				UserName:         a.AnonymizeString(e.UserName),
				ResourceType:     e.ResourceType,
				TotalTokens:      e.TotalTokens,
				PromptTokens:     e.PromptTokens,
				CompletionTokens: e.CompletionTokens,
			},
		},
	}
}

type AssistActionEvent prehogv1a.AssistActionEvent

// Anonymize anonymizes the event.
func (e *AssistActionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AssistAction{
			AssistAction: &prehogv1a.AssistActionEvent{
				UserName:         a.AnonymizeString(e.UserName),
				Action:           e.Action,
				TotalTokens:      e.TotalTokens,
				PromptTokens:     e.PromptTokens,
				CompletionTokens: e.CompletionTokens,
			},
		},
	}
}

type AccessListCreateEvent prehogv1a.AccessListCreateEvent

// Anonymize anonymizes the event.
func (e *AccessListCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListCreate{
			AccessListCreate: &prehogv1a.AccessListCreateEvent{
				UserName: a.AnonymizeString(e.UserName),
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListUpdateEvent prehogv1a.AccessListUpdateEvent

// Anonymize anonymizes the event.
func (e *AccessListUpdateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListUpdate{
			AccessListUpdate: &prehogv1a.AccessListUpdateEvent{
				UserName: a.AnonymizeString(e.UserName),
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListDeleteEvent prehogv1a.AccessListDeleteEvent

// Anonymize anonymizes the event.
func (e *AccessListDeleteEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListDelete{
			AccessListDelete: &prehogv1a.AccessListDeleteEvent{
				UserName: a.AnonymizeString(e.UserName),
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListMemberCreateEvent prehogv1a.AccessListMemberCreateEvent

// Anonymize anonymizes the event.
func (e *AccessListMemberCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListMemberCreate{
			AccessListMemberCreate: &prehogv1a.AccessListMemberCreateEvent{
				UserName:   a.AnonymizeString(e.UserName),
				MemberKind: e.MemberKind,
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListMemberUpdateEvent prehogv1a.AccessListMemberUpdateEvent

// Anonymize anonymizes the event.
func (e *AccessListMemberUpdateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListMemberUpdate{
			AccessListMemberUpdate: &prehogv1a.AccessListMemberUpdateEvent{
				UserName:   a.AnonymizeString(e.UserName),
				MemberKind: e.MemberKind,
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListMemberDeleteEvent prehogv1a.AccessListMemberDeleteEvent

// Anonymize anonymizes the event.
func (e *AccessListMemberDeleteEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListMemberDelete{
			AccessListMemberDelete: &prehogv1a.AccessListMemberDeleteEvent{
				UserName:   a.AnonymizeString(e.UserName),
				MemberKind: e.MemberKind,
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListGrantsToUserEvent prehogv1a.AccessListGrantsToUserEvent

// Anonymize anonymizes the event.
func (e *AccessListGrantsToUserEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListGrantsToUser{
			AccessListGrantsToUser: &prehogv1a.AccessListGrantsToUserEvent{
				UserName:                    a.AnonymizeString(e.UserName),
				CountRolesGranted:           e.CountRolesGranted,
				CountTraitsGranted:          e.CountTraitsGranted,
				CountInheritedRolesGranted:  e.CountInheritedRolesGranted,
				CountInheritedTraitsGranted: e.CountInheritedTraitsGranted,
			},
		},
	}
}

type AccessListReviewCreateEvent prehogv1a.AccessListReviewCreateEvent

// Anonymize anonymizes the event.
func (e *AccessListReviewCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListReviewCreate{
			AccessListReviewCreate: &prehogv1a.AccessListReviewCreateEvent{
				UserName: a.AnonymizeString(e.UserName),
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
				DaysPastNextAuditDate:         e.DaysPastNextAuditDate,
				MembershipRequirementsChanged: e.MembershipRequirementsChanged,
				ReviewFrequencyChanged:        e.ReviewFrequencyChanged,
				ReviewDayOfMonthChanged:       e.ReviewDayOfMonthChanged,
				NumberOfRemovedMembers:        e.NumberOfRemovedMembers,
			},
		},
	}
}

type AccessListReviewDeleteEvent prehogv1a.AccessListReviewDeleteEvent

// Anonymize anonymizes the event.
func (e *AccessListReviewDeleteEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListReviewDelete{
			AccessListReviewDelete: &prehogv1a.AccessListReviewDeleteEvent{
				UserName: a.AnonymizeString(e.UserName),
				Metadata: &prehogv1a.AccessListMetadata{
					Id: a.AnonymizeString(e.Metadata.Id),
				},
			},
		},
	}
}

type AccessListReviewComplianceEvent prehogv1a.AccessListReviewComplianceEvent

// Anonymize anonymizes the event.
func (e *AccessListReviewComplianceEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessListReviewCompliance{
			AccessListReviewCompliance: &prehogv1a.AccessListReviewComplianceEvent{
				TotalAccessLists:      e.TotalAccessLists,
				AccessListsNeedReview: e.AccessListsNeedReview,
			},
		},
	}
}

// UserMetadata contains user metadata information which is used to contextualize events with user information.
type UserMetadata struct {
	// Username contains the user's name.
	Username string
	// IsSSO indicates if the user was created by an SSO provider.
	IsSSO bool
}

// DeviceAuthenticateEvent event is emitted after a successful device authentication ceremony.
type DeviceAuthenticateEvent prehogv1a.DeviceAuthenticateEvent

func (d *DeviceAuthenticateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DeviceAuthenticateEvent{
			DeviceAuthenticateEvent: &prehogv1a.DeviceAuthenticateEvent{
				DeviceId:     a.AnonymizeString(d.DeviceId),
				UserName:     a.AnonymizeString(d.UserName),
				DeviceOsType: d.DeviceOsType,
			},
		},
	}
}

// DeviceEnrollEvent event is emitted after a successful device enrollment.
type DeviceEnrollEvent prehogv1a.DeviceEnrollEvent

func (d *DeviceEnrollEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DeviceEnrollEvent{
			DeviceEnrollEvent: &prehogv1a.DeviceEnrollEvent{
				DeviceId:     a.AnonymizeString(d.DeviceId),
				UserName:     a.AnonymizeString(d.UserName),
				DeviceOsType: d.DeviceOsType,
				DeviceOrigin: d.DeviceOrigin,
			},
		},
	}
}

// FeatureRecommendationEvent emitted when a feature is recommended to user or
// when user completes the desired CTA for the feature.
type FeatureRecommendationEvent prehogv1a.FeatureRecommendationEvent

func (e *FeatureRecommendationEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_FeatureRecommendationEvent{
			FeatureRecommendationEvent: &prehogv1a.FeatureRecommendationEvent{
				UserName:                    a.AnonymizeString(e.UserName),
				Feature:                     e.Feature,
				FeatureRecommendationStatus: e.FeatureRecommendationStatus,
			},
		},
	}
}

// LicenseLimitEvent emitted when a feature is gated behind
// enterprise license.
type LicenseLimitEvent prehogv1a.LicenseLimitEvent

func (e *LicenseLimitEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_LicenseLimitEvent{
			LicenseLimitEvent: &prehogv1a.LicenseLimitEvent{
				LicenseLimit: e.LicenseLimit,
			},
		},
	}
}

// DesktopDirectoryShareEvent is emitted when a user shares a directory
// in a Windows desktop session.
type DesktopDirectoryShareEvent prehogv1a.DesktopDirectoryShareEvent

func (e *DesktopDirectoryShareEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DesktopDirectoryShare{
			DesktopDirectoryShare: &prehogv1a.DesktopDirectoryShareEvent{
				Desktop:       a.AnonymizeString(e.Desktop),
				UserName:      a.AnonymizeString(e.UserName),
				DirectoryName: a.AnonymizeString(e.DirectoryName),
			},
		},
	}
}

// DesktopClipboardEvent is emitted when a user transfers data
// between their local clipboard and the clipboard on a remote Windows
// desktop.
type DesktopClipboardEvent prehogv1a.DesktopClipboardEvent

func (e *DesktopClipboardEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DesktopClipboardTransfer{
			DesktopClipboardTransfer: &prehogv1a.DesktopClipboardEvent{
				Desktop:  a.AnonymizeString(e.Desktop),
				UserName: a.AnonymizeString(e.UserName),
			},
		},
	}
}

type TagExecuteQueryEvent prehogv1a.TAGExecuteQueryEvent

// Anonymize anonymizes the event.
func (e *TagExecuteQueryEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_TagExecuteQuery{
			TagExecuteQuery: &prehogv1a.TAGExecuteQueryEvent{
				UserName:   a.AnonymizeString(e.UserName),
				TotalEdges: e.TotalEdges,
				TotalNodes: e.TotalNodes,
				IsSuccess:  e.IsSuccess,
			},
		},
	}
}

// AccessGraphGitlabScanEvent is emitted when a user scans a GitLab repository
type AccessGraphGitlabScanEvent prehogv1a.AccessGraphGitlabScanEvent

// Anonymize anonymizes the event.
func (e *AccessGraphGitlabScanEvent) Anonymize(_ utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphGitlabScan{
			AccessGraphGitlabScan: &prehogv1a.AccessGraphGitlabScanEvent{
				TotalProjects: e.TotalProjects,
				TotalUsers:    e.TotalUsers,
				TotalGroups:   e.TotalGroups,
			},
		},
	}
}

// AccessGraphSecretsScanAuthorizedKeysEvent is emitted when hosts report authorized keys.
// This event is used to track the number of authorized keys reported by hosts. Keys are
// refreshed periodically.
type AccessGraphSecretsScanAuthorizedKeysEvent prehogv1a.AccessGraphSecretsScanAuthorizedKeysEvent

// Anonymize anonymizes the event.
func (e *AccessGraphSecretsScanAuthorizedKeysEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphSecretsScanAuthorizedKeys{
			AccessGraphSecretsScanAuthorizedKeys: &prehogv1a.AccessGraphSecretsScanAuthorizedKeysEvent{
				HostId:    a.AnonymizeString(e.HostId),
				TotalKeys: e.TotalKeys,
			},
		},
	}
}

// AccessGraphSecretsScanSSHPrivateKeysEvent is emitted when devices report private keys.
type AccessGraphSecretsScanSSHPrivateKeysEvent prehogv1a.AccessGraphSecretsScanSSHPrivateKeysEvent

// Anonymize anonymizes the event.
func (e *AccessGraphSecretsScanSSHPrivateKeysEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphSecretsScanSshPrivateKeys{
			AccessGraphSecretsScanSshPrivateKeys: &prehogv1a.AccessGraphSecretsScanSSHPrivateKeysEvent{
				DeviceId:     a.AnonymizeString(e.DeviceId),
				TotalKeys:    e.TotalKeys,
				DeviceOsType: e.DeviceOsType,
			},
		},
	}
}

// AccessGraphAWSScanEvent is emitted when a user scans an AWS account
type AccessGraphAWSScanEvent prehogv1a.AccessGraphAWSScanEvent

// Anonymize anonymizes the event.
func (e *AccessGraphAWSScanEvent) Anonymize(_ utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphAwsScan{
			AccessGraphAwsScan: &prehogv1a.AccessGraphAWSScanEvent{
				TotalEc2Instances:  e.TotalEc2Instances,
				TotalUsers:         e.TotalUsers,
				TotalGroups:        e.TotalGroups,
				TotalRoles:         e.TotalRoles,
				TotalPolicies:      e.TotalPolicies,
				TotalEksClusters:   e.TotalEksClusters,
				TotalRdsInstances:  e.TotalRdsInstances,
				TotalS3Buckets:     e.TotalS3Buckets,
				TotalSamlProviders: e.TotalSamlProviders,
				TotalOidcProviders: e.TotalOidcProviders,
				TotalAccounts:      e.TotalAccounts,
			},
		},
	}
}

// UIAccessGraphCrownJewelDiffViewEvent is emitted when a user reviews a diff of a Crown Jewel change.
type UIAccessGraphCrownJewelDiffViewEvent prehogv1a.UIAccessGraphCrownJewelDiffViewEvent

// Anonymize anonymizes the event.
func (e *UIAccessGraphCrownJewelDiffViewEvent) Anonymize(_ utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessGraphCrownJewelDiffView{
			UiAccessGraphCrownJewelDiffView: &prehogv1a.UIAccessGraphCrownJewelDiffViewEvent{
				AffectedResourceSource: e.AffectedResourceSource,
				AffectedResourceType:   e.AffectedResourceType,
			},
		},
	}
}

// AccessGraphAccessPathChangedEvent is emitted when a Crown Jewel Access Path changes.
type AccessGraphAccessPathChangedEvent prehogv1a.AccessGraphAccessPathChangedEvent

// Anonymize anonymizes the event.
func (e *AccessGraphAccessPathChangedEvent) Anonymize(_ utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphAccessPathChanged{
			AccessGraphAccessPathChanged: &prehogv1a.AccessGraphAccessPathChangedEvent{
				AffectedResourceType:   strings.ToLower(e.AffectedResourceType),
				AffectedResourceSource: strings.ToLower(e.AffectedResourceSource),
			},
		},
	}
}

// AccessGraphCrownJewelCreateEvent is emitted when a user creates a crown jewel object in Teleport.
type AccessGraphCrownJewelCreateEvent prehogv1a.AccessGraphCrownJewelCreateEvent

// Anonymize anonymizes the event.
func (e *AccessGraphCrownJewelCreateEvent) Anonymize(_ utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AccessGraphCrownJewelCreate{
			AccessGraphCrownJewelCreate: &prehogv1a.AccessGraphCrownJewelCreateEvent{},
		},
	}
}

// ExternalAuditStorageAuthenticateEvent is emitted when the External Audit
// Storage feature authenticates to the customer AWS account via OIDC connector.
// The purpose is to have a regularly emitted event indicating that the External
// Audit Storage feature is still in use.
type ExternalAuditStorageAuthenticateEvent prehogv1a.ExternalAuditStorageAuthenticateEvent

// Anonymize anonymizes the event. Since there is nothing to anonymize, it
// really just wraps itself in a [prehogv1a.SubmitEventRequest].
func (e *ExternalAuditStorageAuthenticateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_ExternalAuditStorageAuthenticate{
			ExternalAuditStorageAuthenticate: &prehogv1a.ExternalAuditStorageAuthenticateEvent{},
		},
	}
}

// SecurityReportGetResultEvent is emitted when a user requests a security report.
type SecurityReportGetResultEvent prehogv1a.SecurityReportGetResultEvent

// Anonymize anonymizes the event. Since there is nothing to anonymize, it
// really just wraps itself in a [prehogv1a.SubmitEventRequest].
func (e *SecurityReportGetResultEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SecurityReportGetResult{
			SecurityReportGetResult: &prehogv1a.SecurityReportGetResultEvent{
				UserName: a.AnonymizeString(e.UserName),
				Name:     e.Name,
				Days:     e.Days,
			},
		},
	}
}

// AuditQueryRunEvent is emitted when a user runs an audit query.
type AuditQueryRunEvent prehogv1a.AuditQueryRunEvent

// Anonymize anonymizes the event. Since there is nothing to anonymize, it
// really just wraps itself in a [prehogv1a.SubmitEventRequest].
func (e *AuditQueryRunEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_AuditQueryRun{
			AuditQueryRun: &prehogv1a.AuditQueryRunEvent{
				UserName:  a.AnonymizeString(e.UserName),
				Days:      e.Days,
				IsSuccess: e.IsSuccess,
			},
		},
	}
}

// DiscoveryFetchEvent is emitted when a DiscoveryService fetchs resources.
type DiscoveryFetchEvent prehogv1a.DiscoveryFetchEvent

// Anonymize anonymizes the event.
func (e *DiscoveryFetchEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DiscoveryFetchEvent{
			DiscoveryFetchEvent: &prehogv1a.DiscoveryFetchEvent{
				CloudProvider: e.CloudProvider,
				ResourceType:  e.ResourceType,
			},
		},
	}
}

// MFAAuthenticationEvent is emitted when a user performs MFA authentication.
type MFAAuthenticationEvent prehogv1a.MFAAuthenticationEvent

// Anonymize anonymizes the event.
func (e *MFAAuthenticationEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_MfaAuthenticationEvent{
			MfaAuthenticationEvent: &prehogv1a.MFAAuthenticationEvent{
				UserName:          a.AnonymizeString(e.UserName),
				DeviceId:          a.AnonymizeString(e.DeviceId),
				DeviceType:        e.DeviceType,
				MfaChallengeScope: e.MfaChallengeScope,
			},
		},
	}
}

// OktaAccessListSyncEvent is emitted when the Okta service syncs access lists from Okta.
type OktaAccessListSyncEvent prehogv1a.OktaAccessListSyncEvent

// Anonymize anonymizes the event.
func (u *OktaAccessListSyncEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_OktaAccessListSync{
			OktaAccessListSync: &prehogv1a.OktaAccessListSyncEvent{
				NumAppFilters:        u.NumAppFilters,
				NumGroupFilters:      u.NumGroupFilters,
				NumApps:              u.NumApps,
				NumGroups:            u.NumGroups,
				NumRoles:             u.NumRoles,
				NumAccessLists:       u.NumAccessLists,
				NumAccessListMembers: u.NumAccessListMembers,
			},
		},
	}
}

// DatabaseUserCreatedEvent is an event that is emitted after database service performs automatic user provisioning.
type DatabaseUserCreatedEvent prehogv1a.DatabaseUserCreatedEvent

// Anonymize anonymizes the event.
func (u *DatabaseUserCreatedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	event := &prehogv1a.DatabaseUserCreatedEvent{
		UserName: a.AnonymizeString(u.UserName),
		NumRoles: u.NumRoles,
	}

	if u.Database != nil {
		event.Database = &prehogv1a.SessionStartDatabaseMetadata{
			DbType:     u.Database.DbType,
			DbProtocol: u.Database.DbProtocol,
			DbOrigin:   u.Database.DbOrigin,
		}
	}

	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DatabaseUserCreated{
			DatabaseUserCreated: event,
		},
	}
}

// DatabaseUserPermissionsUpdateEvent is an event that is emitted after database service updates the permissions for the database user.
type DatabaseUserPermissionsUpdateEvent prehogv1a.DatabaseUserPermissionsUpdateEvent

// Anonymize anonymizes the event.
func (u *DatabaseUserPermissionsUpdateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_DatabaseUserPermissionsUpdated{
			DatabaseUserPermissionsUpdated: &prehogv1a.DatabaseUserPermissionsUpdateEvent{
				UserName:             a.AnonymizeString(u.UserName),
				NumTables:            u.NumTables,
				NumTablesPermissions: u.NumTablesPermissions,
				Database:             u.Database,
			},
		},
	}
}

// SPIFFESVIDIssuedEvent is an event emitted when a SPIFFE SVID has been
// issued.
type SPIFFESVIDIssuedEvent prehogv1a.SPIFFESVIDIssuedEvent

func (u *SPIFFESVIDIssuedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	e := &prehogv1a.SPIFFESVIDIssuedEvent{
		UserName:     a.AnonymizeString(u.UserName),
		UserKind:     u.UserKind,
		SpiffeId:     a.AnonymizeString(u.SpiffeId),
		IpSansCount:  u.IpSansCount,
		DnsSansCount: u.DnsSansCount,
		SvidType:     u.SvidType,
	}
	if u.BotInstanceId != "" {
		e.BotInstanceId = a.AnonymizeString(u.BotInstanceId)
	}
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SpiffeSvidIssued{
			SpiffeSvidIssued: e,
		},
	}
}

// UserTaskStateEvent is an event emitted when the state of a User Task changes.
type UserTaskStateEvent prehogv1a.UserTaskStateEvent

func (u *UserTaskStateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UserTaskState{
			UserTaskState: &prehogv1a.UserTaskStateEvent{
				TaskType:       u.TaskType,
				IssueType:      u.IssueType,
				State:          u.State,
				InstancesCount: u.InstancesCount,
			},
		},
	}
}

// SessionRecordingAccessEvent is an event that is emitted after an user access
// a session recording.
type SessionRecordingAccessEvent prehogv1a.SessionRecordingAccessEvent

func (s *SessionRecordingAccessEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SessionRecordingAccess{
			SessionRecordingAccess: &prehogv1a.SessionRecordingAccessEvent{
				SessionType: s.SessionType,
				UserName:    a.AnonymizeString(s.UserName),
				Format:      s.Format,
			},
		},
	}
}

// ConvertUsageEvent converts a usage event from an API object into an
// anonymizable event. All events that can be submitted externally via the Auth
// API need to be defined here.
func ConvertUsageEvent(event *usageeventsv1.UsageEventOneOf, userMD UserMetadata) (Anonymizable, error) {
	// Note: events (especially pre-registration) that embed a username of their
	// own should generally pass that through rather than using the identity
	// username provided to the function. It may be the username of a Teleport
	// component (e.g. proxy) rather than the end user.

	switch e := event.GetEvent().(type) {
	case *usageeventsv1.UsageEventOneOf_UiBannerClick:
		return &UIBannerClickEvent{
			UserName: userMD.Username,
			Alert:    e.UiBannerClick.Alert,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceClick:
		return &UIOnboardAddFirstResourceClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick:
		return &UIOnboardAddFirstResourceLaterClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick:
		return &UIOnboardCompleteGoToDashboardClickEvent{
			UserName: e.UiOnboardCompleteGoToDashboardClick.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardSetCredentialSubmit:
		return &UIOnboardSetCredentialSubmitEvent{
			UserName: e.UiOnboardSetCredentialSubmit.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardQuestionnaireSubmit:
		return &UIOnboardQuestionnaireSubmitEvent{
			UserName: e.UiOnboardQuestionnaireSubmit.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardRegisterChallengeSubmit:
		return &UIOnboardRegisterChallengeSubmitEvent{
			UserName:  e.UiOnboardRegisterChallengeSubmit.Username,
			MfaType:   e.UiOnboardRegisterChallengeSubmit.MfaType,
			LoginFlow: e.UiOnboardRegisterChallengeSubmit.LoginFlow,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiRecoveryCodesContinueClick:
		return &UIRecoveryCodesContinueClickEvent{
			UserName: e.UiRecoveryCodesContinueClick.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiRecoveryCodesCopyClick:
		return &UIRecoveryCodesCopyClickEvent{
			UserName: e.UiRecoveryCodesCopyClick.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiRecoveryCodesPrintClick:
		return &UsageUIRecoveryCodesPrintClick{
			UserName: e.UiRecoveryCodesPrintClick.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleClick:
		return &UICreateNewRoleClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleSaveClick:
		return &UICreateNewRoleSaveClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleCancelClick:
		return &UICreateNewRoleCancelClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleViewDocumentationClick:
		return &UICreateNewRoleViewDocumentationClickEvent{
			UserName: userMD.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStartEvent:
		ret := &UIIntegrationEnrollStartEvent{
			Metadata: integrationEnrollMetadataToPrehog(e.UiIntegrationEnrollStartEvent.Metadata, userMD),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiIntegrationEnrollCompleteEvent:
		ret := &UIIntegrationEnrollCompleteEvent{
			Metadata: integrationEnrollMetadataToPrehog(e.UiIntegrationEnrollCompleteEvent.Metadata, userMD),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStepEvent:
		ret := &UIIntegrationEnrollStepEvent{
			Metadata: integrationEnrollMetadataToPrehog(e.UiIntegrationEnrollStepEvent.Metadata, userMD),
			Step:     prehogv1a.IntegrationEnrollStep(e.UiIntegrationEnrollStepEvent.Step),
			Status: &prehogv1a.IntegrationEnrollStepStatus{
				Code:  prehogv1a.IntegrationEnrollStatusCode(e.UiIntegrationEnrollStepEvent.GetStatus().GetCode()),
				Error: e.UiIntegrationEnrollStepEvent.GetStatus().GetError(),
			},
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent:
		ret := &UIDiscoverStartedEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverStartedEvent.Metadata, userMD),
			Status:   discoverStatusToPrehog(e.UiDiscoverStartedEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent:
		ret := &UIDiscoverResourceSelectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverResourceSelectionEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverResourceSelectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverResourceSelectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverIntegrationAwsOidcConnectEvent:
		ret := &UIDiscoverIntegrationAWSOIDCConnectEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverIntegrationAwsOidcConnectEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverIntegrationAwsOidcConnectEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverIntegrationAwsOidcConnectEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseRdsEnrollEvent:
		ret := &UIDiscoverDatabaseRDSEnrollEvent{
			Metadata:               discoverMetadataToPrehog(e.UiDiscoverDatabaseRdsEnrollEvent.Metadata, userMD),
			Resource:               discoverResourceToPrehog(e.UiDiscoverDatabaseRdsEnrollEvent.Resource),
			Status:                 discoverStatusToPrehog(e.UiDiscoverDatabaseRdsEnrollEvent.Status),
			SelectedResourcesCount: e.UiDiscoverDatabaseRdsEnrollEvent.SelectedResourcesCount,
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverKubeEksEnrollEvent:
		ret := &UIDiscoverKubeEKSEnrollEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverKubeEksEnrollEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverKubeEksEnrollEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverKubeEksEnrollEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiCallToActionClickEvent:
		return &UICallToActionClickEvent{
			UserName: userMD.Username,
			Cta:      prehogv1a.CTA(e.UiCallToActionClickEvent.Cta),
		}, nil

	case *usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent:
		ret := &UIDiscoverDeployServiceEvent{
			Metadata:     discoverMetadataToPrehog(e.UiDiscoverDeployServiceEvent.Metadata, userMD),
			Resource:     discoverResourceToPrehog(e.UiDiscoverDeployServiceEvent.Resource),
			Status:       discoverStatusToPrehog(e.UiDiscoverDeployServiceEvent.Status),
			DeployMethod: prehogv1a.UIDiscoverDeployServiceEvent_DeployMethod(e.UiDiscoverDeployServiceEvent.DeployMethod),
			DeployType:   prehogv1a.UIDiscoverDeployServiceEvent_DeployType(e.UiDiscoverDeployServiceEvent.DeployType),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverCreateDiscoveryConfig:
		ret := &UIDiscoverCreateDiscoveryConfigEvent{
			Metadata:     discoverMetadataToPrehog(e.UiDiscoverCreateDiscoveryConfig.Metadata, userMD),
			Resource:     discoverResourceToPrehog(e.UiDiscoverCreateDiscoveryConfig.Resource),
			Status:       discoverStatusToPrehog(e.UiDiscoverCreateDiscoveryConfig.Status),
			ConfigMethod: prehogv1a.UIDiscoverCreateDiscoveryConfigEvent_ConfigMethod(e.UiDiscoverCreateDiscoveryConfig.ConfigMethod),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseRegisterEvent:
		ret := &UIDiscoverDatabaseRegisterEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseRegisterEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseRegisterEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseRegisterEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureMtlsEvent:
		ret := &UIDiscoverDatabaseConfigureMTLSEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryToolsInstallEvent:
		ret := &UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryConfigureEvent:
		ret := &UIDiscoverDesktopActiveDirectoryConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent:
		ret := &UIDiscoverAutoDiscoveredResourcesEvent{
			Metadata:       discoverMetadataToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Metadata, userMD),
			Resource:       discoverResourceToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Resource),
			Status:         discoverStatusToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Status),
			ResourcesCount: e.UiDiscoverAutoDiscoveredResourcesEvent.ResourcesCount,
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverEc2InstanceSelection:
		ret := &UIDiscoverEC2InstanceSelectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverEc2InstanceSelection.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverEc2InstanceSelection.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverEc2InstanceSelection.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDeployEice:
		ret := &UIDiscoverDeployEICEEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDeployEice.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDeployEice.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDeployEice.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverCreateNode:
		ret := &UIDiscoverCreateNodeEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverCreateNode.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverCreateNode.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverCreateNode.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverCreateAppServerEvent:
		ret := &UIDiscoverCreateAppServerEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverCreateAppServerEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverCreateAppServerEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverCreateAppServerEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureIamPolicyEvent:
		ret := &UIDiscoverDatabaseConfigureIAMPolicyEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverPrincipalsConfigureEvent:
		ret := &UIDiscoverPrincipalsConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverTestConnectionEvent:
		ret := &UIDiscoverTestConnectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverTestConnectionEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverTestConnectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverTestConnectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverCompletedEvent:
		ret := &UIDiscoverCompletedEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverCompletedEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverCompletedEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverCompletedEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AssistCompletion:
		ret := &AssistCompletionEvent{
			UserName:         userMD.Username,
			ConversationId:   e.AssistCompletion.ConversationId,
			TotalTokens:      e.AssistCompletion.TotalTokens,
			PromptTokens:     e.AssistCompletion.PromptTokens,
			CompletionTokens: e.AssistCompletion.CompletionTokens,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AssistExecution:
		ret := &AssistExecutionEvent{
			UserName:         userMD.Username,
			ConversationId:   e.AssistExecution.ConversationId,
			NodeCount:        e.AssistExecution.NodeCount,
			TotalTokens:      e.AssistExecution.TotalTokens,
			PromptTokens:     e.AssistExecution.PromptTokens,
			CompletionTokens: e.AssistExecution.CompletionTokens,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AssistNewConversation:
		ret := &AssistNewConversationEvent{
			UserName: userMD.Username,
			Category: e.AssistNewConversation.Category,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_ResourceCreateEvent:
		ret := &ResourceCreateEvent{
			ResourceType:   e.ResourceCreateEvent.ResourceType,
			ResourceOrigin: e.ResourceCreateEvent.ResourceOrigin,
			CloudProvider:  e.ResourceCreateEvent.CloudProvider,
		}
		if db := e.ResourceCreateEvent.Database; db != nil {
			ret.Database = &prehogv1a.DiscoveredDatabaseMetadata{
				DbType:     db.DbType,
				DbProtocol: db.DbProtocol,
			}
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_FeatureRecommendationEvent:
		ret := &FeatureRecommendationEvent{
			UserName:                    userMD.Username,
			Feature:                     prehogv1a.Feature(e.FeatureRecommendationEvent.Feature),
			FeatureRecommendationStatus: prehogv1a.FeatureRecommendationStatus(e.FeatureRecommendationEvent.FeatureRecommendationStatus),
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AssistAccessRequest:
		ret := &AssistAccessRequestEvent{
			UserName:         userMD.Username,
			ResourceType:     e.AssistAccessRequest.ResourceType,
			TotalTokens:      e.AssistAccessRequest.TotalTokens,
			PromptTokens:     e.AssistAccessRequest.PromptTokens,
			CompletionTokens: e.AssistAccessRequest.CompletionTokens,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AssistAction:
		ret := &AssistActionEvent{
			UserName:         userMD.Username,
			Action:           e.AssistAction.Action,
			TotalTokens:      e.AssistAction.TotalTokens,
			PromptTokens:     e.AssistAction.PromptTokens,
			CompletionTokens: e.AssistAction.CompletionTokens,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListCreate:
		ret := &AccessListCreateEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListCreate.Metadata.Id,
			},
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListUpdate:
		ret := &AccessListUpdateEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListUpdate.Metadata.Id,
			},
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListDelete:
		ret := &AccessListDeleteEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListDelete.Metadata.Id,
			},
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListMemberCreate:
		ret := &AccessListMemberCreateEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListMemberCreate.Metadata.Id,
			},
			MemberKind: e.AccessListMemberCreate.MemberMetadata.MembershipKind.String(),
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListMemberUpdate:
		ret := &AccessListMemberUpdateEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListMemberUpdate.Metadata.Id,
			},
			MemberKind: e.AccessListMemberUpdate.MemberMetadata.MembershipKind.String(),
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListMemberDelete:
		ret := &AccessListMemberDeleteEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListMemberDelete.Metadata.Id,
			},
			MemberKind: e.AccessListMemberDelete.MemberMetadata.MembershipKind.String(),
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListGrantsToUser:
		ret := &AccessListGrantsToUserEvent{
			UserName:                    userMD.Username,
			CountRolesGranted:           e.AccessListGrantsToUser.CountRolesGranted,
			CountTraitsGranted:          e.AccessListGrantsToUser.CountTraitsGranted,
			CountInheritedRolesGranted:  e.AccessListGrantsToUser.CountInheritedRolesGranted,
			CountInheritedTraitsGranted: e.AccessListGrantsToUser.CountInheritedTraitsGranted,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_TagExecuteQuery:
		ret := &TagExecuteQueryEvent{
			UserName:   userMD.Username,
			TotalEdges: e.TagExecuteQuery.TotalEdges,
			TotalNodes: e.TagExecuteQuery.TotalNodes,
			IsSuccess:  e.TagExecuteQuery.IsSuccess,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_SecurityReportGetResult:
		ret := &SecurityReportGetResultEvent{
			UserName: userMD.Username,
			Name:     e.SecurityReportGetResult.Name,
			Days:     e.SecurityReportGetResult.Days,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListReviewCreate:
		ret := &AccessListReviewCreateEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListReviewCreate.Metadata.Id,
			},
			DaysPastNextAuditDate:         e.AccessListReviewCreate.DaysPastNextAuditDate,
			MembershipRequirementsChanged: e.AccessListReviewCreate.MembershipRequirementsChanged,
			ReviewFrequencyChanged:        e.AccessListReviewCreate.ReviewFrequencyChanged,
			ReviewDayOfMonthChanged:       e.AccessListReviewCreate.ReviewDayOfMonthChanged,
			NumberOfRemovedMembers:        e.AccessListReviewCreate.NumberOfRemovedMembers,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessListReviewDelete:
		ret := &AccessListReviewDeleteEvent{
			UserName: userMD.Username,
			Metadata: &prehogv1a.AccessListMetadata{
				Id: e.AccessListReviewDelete.Metadata.Id,
			},
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_DiscoveryFetchEvent:
		ret := &DiscoveryFetchEvent{
			CloudProvider: e.DiscoveryFetchEvent.CloudProvider,
			ResourceType:  e.DiscoveryFetchEvent.ResourceType,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_AccessGraphAwsScanEvent:
		ret := &AccessGraphAWSScanEvent{
			TotalEc2Instances:  e.AccessGraphAwsScanEvent.TotalEc2Instances,
			TotalUsers:         e.AccessGraphAwsScanEvent.TotalUsers,
			TotalGroups:        e.AccessGraphAwsScanEvent.TotalGroups,
			TotalRoles:         e.AccessGraphAwsScanEvent.TotalRoles,
			TotalPolicies:      e.AccessGraphAwsScanEvent.TotalPolicies,
			TotalEksClusters:   e.AccessGraphAwsScanEvent.TotalEksClusters,
			TotalRdsInstances:  e.AccessGraphAwsScanEvent.TotalRdsInstances,
			TotalS3Buckets:     e.AccessGraphAwsScanEvent.TotalS3Buckets,
			TotalSamlProviders: e.AccessGraphAwsScanEvent.TotalSamlProviders,
			TotalOidcProviders: e.AccessGraphAwsScanEvent.TotalOidcProviders,
			TotalAccounts:      e.AccessGraphAwsScanEvent.TotalAccounts,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiAccessGraphCrownJewelDiffView:
		data := e.UiAccessGraphCrownJewelDiffView
		if data.AffectedResourceType == "" {
			return nil, trace.BadParameter("affected resource type is empty")
		}
		if data.AffectedResourceSource == "" {
			return nil, trace.BadParameter("affected resource source is empty")
		}
		ret := &UIAccessGraphCrownJewelDiffViewEvent{
			AffectedResourceSource: data.AffectedResourceSource,
			AffectedResourceType:   data.AffectedResourceType,
		}
		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UserTaskStateEvent:
		data := e.UserTaskStateEvent
		if data.TaskType == "" {
			return nil, trace.BadParameter("task type is empty")
		}
		if data.IssueType == "" {
			return nil, trace.BadParameter("issue type is empty")
		}
		if data.State == "" {
			return nil, trace.BadParameter("state is empty")
		}
		ret := &UserTaskStateEvent{
			TaskType:       data.TaskType,
			IssueType:      data.IssueType,
			State:          data.State,
			InstancesCount: data.InstancesCount,
		}
		return ret, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
