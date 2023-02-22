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
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	usageevents "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
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
	// SubmitAnonymizedUsageEvents submits a usage event. The payload will be
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
				UserName:  a.AnonymizeString(u.UserName),
				MfaType:   u.MfaType,
				LoginFlow: u.LoginFlow,
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

// UsageRoleCreate is an event emitted when a custom role is created.
type UsageRoleCreate prehogv1.RoleCreateEvent

func (u *UsageRoleCreate) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	role := u.RoleName
	if !slices.Contains(teleport.PresetRoles, u.RoleName) {
		role = a.AnonymizeString(u.RoleName)
	}

	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_RoleCreate{
			RoleCreate: &prehogv1.RoleCreateEvent{
				UserName: a.AnonymizeString(u.UserName),
				RoleName: role,
			},
		},
	}
}

// UsageUICreateNewRoleClickEvent is a UI event sent when a user prints recovery codes.
type UsageUICreateNewRoleClickEvent prehogv1.UICreateNewRoleClickEvent

func (u *UsageUICreateNewRoleClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleClick{
			UiCreateNewRoleClick: &prehogv1.UICreateNewRoleClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUICreateNewRoleSaveClickEvent is a UI event sent when a user prints recovery codes.
type UsageUICreateNewRoleSaveClickEvent prehogv1.UICreateNewRoleSaveClickEvent

func (u *UsageUICreateNewRoleSaveClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleSaveClick{
			UiCreateNewRoleSaveClick: &prehogv1.UICreateNewRoleSaveClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUICreateNewRoleCancelClickEvent is a UI event sent when a user prints recovery codes.
type UsageUICreateNewRoleCancelClickEvent prehogv1.UICreateNewRoleCancelClickEvent

func (u *UsageUICreateNewRoleCancelClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleCancelClick{
			UiCreateNewRoleCancelClick: &prehogv1.UICreateNewRoleCancelClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageUICreateNewRoleViewDocumentationClickEvent is a UI event sent when a user prints recovery codes.
type UsageUICreateNewRoleViewDocumentationClickEvent prehogv1.UICreateNewRoleViewDocumentationClickEvent

func (u *UsageUICreateNewRoleViewDocumentationClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleViewDocumentationClick{
			UiCreateNewRoleViewDocumentationClick: &prehogv1.UICreateNewRoleViewDocumentationClickEvent{
				UserName: a.AnonymizeString(u.UserName),
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

// UsageKubeRequest is an event emitted when a Kubernetes API request is
// handled.
type UsageKubeRequest prehogv1.KubeRequestEvent

func (u *UsageKubeRequest) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_KubeRequest{
			KubeRequest: &prehogv1.KubeRequestEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UsageSFTP is an event emitted for each file operation in a SFTP connection.
type UsageSFTP prehogv1.SFTPEvent

func (u *UsageSFTP) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_Sftp{
			Sftp: &prehogv1.SFTPEvent{
				UserName: a.AnonymizeString(u.UserName),
				Action:   u.Action,
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
	case *usageevents.UsageEventOneOf_UiCreateNewRoleClick:
		return &UsageUICreateNewRoleClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiCreateNewRoleSaveClick:
		return &UsageUICreateNewRoleSaveClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiCreateNewRoleCancelClick:
		return &UsageUICreateNewRoleCancelClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageevents.UsageEventOneOf_UiCreateNewRoleViewDocumentationClick:
		return &UsageUICreateNewRoleViewDocumentationClickEvent{
			UserName: identityUsername,
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
	case *usageevents.UsageEventOneOf_UiDiscoverDeployServiceEvent:
		ret := &UsageUIDiscoverDeployServiceEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDeployServiceEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDeployServiceEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDeployServiceEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverDatabaseRegisterEvent:
		ret := &UsageUIDiscoverDatabaseRegisterEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseRegisterEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseRegisterEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseRegisterEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverDatabaseConfigureMtlsEvent:
		ret := &UsageUIDiscoverDatabaseConfigureMTLSEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryToolsInstallEvent:
		ret := &UsageUIDiscoverDesktopActiveDirectoryToolsInstallEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryConfigureEvent:
		ret := &UsageUIDiscoverDesktopActiveDirectoryConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent:
		ret := &UsageUIDiscoverAutoDiscoveredResourcesEvent{
			Metadata:       discoverMetadataToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Metadata, identityUsername),
			Resource:       discoverResourceToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Resource),
			Status:         discoverStatusToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Status),
			ResourcesCount: e.UiDiscoverAutoDiscoveredResourcesEvent.ResourcesCount,
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverDatabaseConfigureIamPolicyEvent:
		ret := &UsageUIDiscoverDatabaseConfigureIAMPolicyEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverPrincipalsConfigureEvent:
		ret := &UsageUIDiscoverPrincipalsConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverTestConnectionEvent:
		ret := &UsageUIDiscoverTestConnectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverTestConnectionEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverTestConnectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverTestConnectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageevents.UsageEventOneOf_UiDiscoverCompletedEvent:
		ret := &UsageUIDiscoverCompletedEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverCompletedEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverCompletedEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverCompletedEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
