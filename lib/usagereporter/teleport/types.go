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
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// Anonymizable is an event that can be anonymized.
type Anonymizable interface {
	// Anonymize uses the given anonymizer to anonymize the event and converts
	// it into a partially filled SubmitEventRequest.
	Anonymize(utils.Anonymizer) prehogv1.SubmitEventRequest
}

// UserLoginEvent is an event emitted when a user logs into Teleport,
// potentially via SSO.
type UserLoginEvent prehogv1.UserLoginEvent

func (u *UserLoginEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	var deviceID string
	if u.DeviceId != "" {
		deviceID = a.AnonymizeString(u.DeviceId)
	}
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UserLogin{
			UserLogin: &prehogv1.UserLoginEvent{
				UserName:      a.AnonymizeString(u.UserName),
				ConnectorType: u.ConnectorType,
				DeviceId:      deviceID,
			},
		},
	}
}

// SSOCreateEvent is emitted when an SSO connector has been created.
type SSOCreateEvent prehogv1.SSOCreateEvent

func (u *SSOCreateEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_SsoCreate{
			SsoCreate: &prehogv1.SSOCreateEvent{
				ConnectorType: u.ConnectorType,
			},
		},
	}
}

// SessionStartEvent is an event emitted when some Teleport session has started
// (ssh, etc).
type SessionStartEvent prehogv1.SessionStartEvent

func (u *SessionStartEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_SessionStartV2{
			SessionStartV2: &prehogv1.SessionStartEvent{
				UserName:    a.AnonymizeString(u.UserName),
				SessionType: u.SessionType,
			},
		},
	}
}

// ResourceCreateEvent is an event emitted when various resource types have been
// created.
type ResourceCreateEvent prehogv1.ResourceCreateEvent

func (u *ResourceCreateEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_ResourceCreate{
			ResourceCreate: &prehogv1.ResourceCreateEvent{
				ResourceType: u.ResourceType,
			},
		},
	}
}

// UIBannerClickEvent is a UI event sent when a banner is clicked.
type UIBannerClickEvent prehogv1.UIBannerClickEvent

func (u *UIBannerClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiBannerClick{
			UiBannerClick: &prehogv1.UIBannerClickEvent{
				UserName: a.AnonymizeString(u.UserName),
				Alert:    u.Alert,
			},
		},
	}
}

// UIOnboardCompleteGoToDashboardClickEvent is a UI event sent when
// onboarding is complete.
type UIOnboardCompleteGoToDashboardClickEvent prehogv1.UIOnboardCompleteGoToDashboardClickEvent

func (u *UIOnboardCompleteGoToDashboardClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardCompleteGoToDashboardClick{
			UiOnboardCompleteGoToDashboardClick: &prehogv1.UIOnboardCompleteGoToDashboardClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardAddFirstResourceClickEvent is a UI event sent when a user
// clicks the "add first resource" button.
type UIOnboardAddFirstResourceClickEvent prehogv1.UIOnboardAddFirstResourceClickEvent

func (u *UIOnboardAddFirstResourceClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceClick{
			UiOnboardAddFirstResourceClick: &prehogv1.UIOnboardAddFirstResourceClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardAddFirstResourceLaterClickEvent is a UI event sent when a user
// clicks the "add first resource later" button.
type UIOnboardAddFirstResourceLaterClickEvent prehogv1.UIOnboardAddFirstResourceLaterClickEvent

func (u *UIOnboardAddFirstResourceLaterClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceLaterClick{
			UiOnboardAddFirstResourceLaterClick: &prehogv1.UIOnboardAddFirstResourceLaterClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardSetCredentialSubmitEvent is an UI event sent during registration
// when the user configures login credentials.
type UIOnboardSetCredentialSubmitEvent prehogv1.UIOnboardSetCredentialSubmitEvent

func (u *UIOnboardSetCredentialSubmitEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &prehogv1.UIOnboardSetCredentialSubmitEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIOnboardRegisterChallengeSubmitEvent is a UI event sent during registration
// when the MFA challenge is completed.
type UIOnboardRegisterChallengeSubmitEvent prehogv1.UIOnboardRegisterChallengeSubmitEvent

func (u *UIOnboardRegisterChallengeSubmitEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
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

// UIRecoveryCodesContinueClickEvent is a UI event sent when a user configures recovery codes.
type UIRecoveryCodesContinueClickEvent prehogv1.UIRecoveryCodesContinueClickEvent

func (u *UIRecoveryCodesContinueClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &prehogv1.UIRecoveryCodesContinueClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UIRecoveryCodesCopyClickEvent is a UI event sent when a user copies recovery codes.
type UIRecoveryCodesCopyClickEvent prehogv1.UIRecoveryCodesCopyClickEvent

func (u *UIRecoveryCodesCopyClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
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

// RoleCreateEvent is an event emitted when a custom role is created.
type RoleCreateEvent prehogv1.RoleCreateEvent

func (u *RoleCreateEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
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

// UICreateNewRoleClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleClickEvent prehogv1.UICreateNewRoleClickEvent

func (u *UICreateNewRoleClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleClick{
			UiCreateNewRoleClick: &prehogv1.UICreateNewRoleClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleSaveClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleSaveClickEvent prehogv1.UICreateNewRoleSaveClickEvent

func (u *UICreateNewRoleSaveClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleSaveClick{
			UiCreateNewRoleSaveClick: &prehogv1.UICreateNewRoleSaveClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleCancelClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleCancelClickEvent prehogv1.UICreateNewRoleCancelClickEvent

func (u *UICreateNewRoleCancelClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleCancelClick{
			UiCreateNewRoleCancelClick: &prehogv1.UICreateNewRoleCancelClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UICreateNewRoleViewDocumentationClickEvent is a UI event sent when a user prints recovery codes.
type UICreateNewRoleViewDocumentationClickEvent prehogv1.UICreateNewRoleViewDocumentationClickEvent

func (u *UICreateNewRoleViewDocumentationClickEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiCreateNewRoleViewDocumentationClick{
			UiCreateNewRoleViewDocumentationClick: &prehogv1.UICreateNewRoleViewDocumentationClickEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// UserCertificateIssuedEvent is an event emitted when a certificate has been
// issued, used to track the duration and restriction.
type UserCertificateIssuedEvent prehogv1.UserCertificateIssuedEvent

func (u *UserCertificateIssuedEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
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

// KubeRequestEvent is an event emitted when a Kubernetes API request is
// handled.
type KubeRequestEvent prehogv1.KubeRequestEvent

func (u *KubeRequestEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_KubeRequest{
			KubeRequest: &prehogv1.KubeRequestEvent{
				UserName: a.AnonymizeString(u.UserName),
			},
		},
	}
}

// SFTPEvent is an event emitted for each file operation in a SFTP connection.
type SFTPEvent prehogv1.SFTPEvent

func (u *SFTPEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_Sftp{
			Sftp: &prehogv1.SFTPEvent{
				UserName: a.AnonymizeString(u.UserName),
				Action:   u.Action,
			},
		},
	}
}

// AgentMetadataEvent is an event emitted after an agent first connects to the auth server.
type AgentMetadataEvent prehogv1.AgentMetadataEvent

func (u *AgentMetadataEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_AgentMetadataEvent{
			AgentMetadataEvent: &prehogv1.AgentMetadataEvent{
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

type ResourceKind = prehogv1.ResourceKind

const (
	ResourceKindNode           = prehogv1.ResourceKind_RESOURCE_KIND_NODE
	ResourceKindAppServer      = prehogv1.ResourceKind_RESOURCE_KIND_APP_SERVER
	ResourceKindKubeServer     = prehogv1.ResourceKind_RESOURCE_KIND_KUBE_SERVER
	ResourceKindDBServer       = prehogv1.ResourceKind_RESOURCE_KIND_DB_SERVER
	ResourceKindWindowsDesktop = prehogv1.ResourceKind_RESOURCE_KIND_WINDOWS_DESKTOP
	ResourceKindNodeOpenSSH    = prehogv1.ResourceKind_RESOURCE_KIND_NODE_OPENSSH
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
	Kind   prehogv1.ResourceKind
	Static bool
}

func (u *ResourceHeartbeatEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_ResourceHeartbeat{
			ResourceHeartbeat: &prehogv1.ResourceHeartbeatEvent{
				ResourceName: a.AnonymizeNonEmpty(u.Name),
				ResourceKind: u.Kind,
				Static:       u.Static,
			},
		},
	}
}

// UserMetadata contains user metadata information which is used to contextualize events with user information.
type UserMetadata struct {
	Username string
	IsSSO    bool
}

// ConvertUsageEvent converts a usage event from an API object into an
// anonymizable event. All events that can be submitted externally via the Auth
// API need to be defined here.
func ConvertUsageEvent(ctx context.Context, event *usageeventsv1.UsageEventOneOf, userMD UserMetadata) (Anonymizable, error) {
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
	case *usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent:
		ret := &UIDiscoverDeployServiceEvent{
			Metadata: discoverMetadataToPrehog(e.UiDiscoverDeployServiceEvent.Metadata, userMD),
			Resource: discoverResourceToPrehog(e.UiDiscoverDeployServiceEvent.Resource),
			Status:   discoverStatusToPrehog(e.UiDiscoverDeployServiceEvent.Status),
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
	default:
		return nil, trace.BadParameter("invalid usage event type %T", event.GetEvent())
	}
}
