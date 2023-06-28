/*
Copyright 2023 Gravitational, Inc.

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

package usagereporter

import (
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
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
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UserLogin{
			UserLogin: &prehogv1a.UserLoginEvent{
				UserName:      a.AnonymizeString(u.UserName),
				ConnectorType: u.ConnectorType,
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
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_SessionStartV2{
			SessionStartV2: &prehogv1a.SessionStartEvent{
				UserName:    a.AnonymizeString(u.UserName),
				SessionType: u.SessionType,
			},
		},
	}
}

// ResourceCreateEvent is an event emitted when various resource types have been
// created.
type ResourceCreateEvent prehogv1a.ResourceCreateEvent

func (u *ResourceCreateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_ResourceCreate{
			ResourceCreate: &prehogv1a.ResourceCreateEvent{
				ResourceType: u.ResourceType,
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

// UserCertificateIssuedEvent is an event emitted when a certificate has been
// issued, used to track the duration and restriction.
type UserCertificateIssuedEvent prehogv1a.UserCertificateIssuedEvent

func (u *UserCertificateIssuedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UserCertificateIssuedEvent{
			UserCertificateIssuedEvent: &prehogv1a.UserCertificateIssuedEvent{
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

// KubeRequestEvent is an event emitted when a Kubernetes API request is
// handled.
type KubeRequestEvent prehogv1a.KubeRequestEvent

func (u *KubeRequestEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_KubeRequest{
			KubeRequest: &prehogv1a.KubeRequestEvent{
				UserName: a.AnonymizeString(u.UserName),
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
			},
		},
	}
}

// ConvertUsageEvent converts a usage event from an API object into an
// anonymizable event. All events that can be submitted externally via the Auth
// API need to be defined here.
func ConvertUsageEvent(event *usageeventsv1.UsageEventOneOf, identityUsername string) (Anonymizable, error) {
	// Note: events (especially pre-registration) that embed a username of their
	// own should generally pass that through rather than using the identity
	// username provided to the function. It may be the username of a Teleport
	// component (e.g. proxy) rather than the end user.

	switch e := event.GetEvent().(type) {
	case *usageeventsv1.UsageEventOneOf_UiBannerClick:
		return &UIBannerClickEvent{
			UserName: identityUsername,
			Alert:    e.UiBannerClick.Alert,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceClick:
		return &UIOnboardAddFirstResourceClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick:
		return &UIOnboardAddFirstResourceLaterClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick:
		return &UIOnboardCompleteGoToDashboardClickEvent{
			UserName: e.UiOnboardCompleteGoToDashboardClick.Username,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiOnboardSetCredentialSubmit:
		return &UIOnboardSetCredentialSubmitEvent{
			UserName: e.UiOnboardSetCredentialSubmit.Username,
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
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleSaveClick:
		return &UICreateNewRoleSaveClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleCancelClick:
		return &UICreateNewRoleCancelClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiCreateNewRoleViewDocumentationClick:
		return &UICreateNewRoleViewDocumentationClickEvent{
			UserName: identityUsername,
		}, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent:
		ret := &UIDiscoverStartedEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverStartedEvent.Metadata, identityUsername),
			Status:   discoverStatusToPrehog(e.UiDiscoverStartedEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent:
		ret := &UIDiscoverResourceSelectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverResourceSelectionEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverResourceSelectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverResourceSelectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent:
		ret := &UIDiscoverDeployServiceEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDeployServiceEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDeployServiceEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDeployServiceEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseRegisterEvent:
		ret := &UIDiscoverDatabaseRegisterEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseRegisterEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseRegisterEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseRegisterEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureMtlsEvent:
		ret := &UIDiscoverDatabaseConfigureMTLSEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureMtlsEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryToolsInstallEvent:
		ret := &UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryToolsInstallEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryConfigureEvent:
		ret := &UIDiscoverDesktopActiveDirectoryConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDesktopActiveDirectoryConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent:
		ret := &UIDiscoverAutoDiscoveredResourcesEvent{
			Metadata:       discoverMetadataToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Metadata, identityUsername),
			Resource:       discoverResourceToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Resource),
			Status:         discoverStatusToPrehog(e.UiDiscoverAutoDiscoveredResourcesEvent.Status),
			ResourcesCount: e.UiDiscoverAutoDiscoveredResourcesEvent.ResourcesCount,
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureIamPolicyEvent:
		ret := &UIDiscoverDatabaseConfigureIAMPolicyEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDatabaseConfigureIamPolicyEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverPrincipalsConfigureEvent:
		ret := &UIDiscoverPrincipalsConfigureEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverPrincipalsConfigureEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverTestConnectionEvent:
		ret := &UIDiscoverTestConnectionEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverTestConnectionEvent.Metadata, identityUsername),
			Resource: discoverResourceToPrehog(e.UiDiscoverTestConnectionEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverTestConnectionEvent.Status),
		}
		if err := ret.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return ret, nil
	case *usageeventsv1.UsageEventOneOf_UiDiscoverCompletedEvent:
		ret := &UIDiscoverCompletedEvent{
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
