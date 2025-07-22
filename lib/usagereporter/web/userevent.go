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
	"encoding/json"
	"slices"

	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
)

// these constants are 1:1 with user events found in the web directory
// web/packages/teleport/src/services/userEvent/types.ts
const (
	bannerClickEvent                = "tp.ui.banner.click"
	setCredentialSubmitEvent        = "tp.ui.onboard.setCredential.submit"
	registerChallengeSubmitEvent    = "tp.ui.onboard.registerChallenge.submit"
	addFirstResourceClickEvent      = "tp.ui.onboard.addFirstResource.click"
	addFirstResourceLaterClickEvent = "tp.ui.onboard.addFirstResourceLater.click"
	completeGoToDashboardClickEvent = "tp.ui.onboard.completeGoToDashboard.click"
	questionnaireSubmitEvent        = "tp.ui.onboard.questionnaire.submit"

	recoveryCodesContinueClickEvent          = "tp.ui.recoveryCodesContinue.click"
	recoveryCodesCopyClickEvent              = "tp.ui.recoveryCodesCopy.click"
	recoveryCodesPrintClickEvent             = "tp.ui.recoveryCodesPrint.click"
	createNewRoleClickEvent                  = "tp.ui.createNewRole.click"
	createNewRoleSaveClickEvent              = "tp.ui.createNewRoleSave.click"
	createNewRoleCancelClickEvent            = "tp.ui.createNewRoleCancel.click"
	createNewRoleViewDocumentationClickEvent = "tp.ui.createNewRoleViewDocumentation.click"

	uiDiscoverStartedEvent                            = "tp.ui.discover.started"
	uiDiscoverResourceSelectionEvent                  = "tp.ui.discover.resourceSelection"
	uiDiscoverIntegrationAWSOIDCConnectEvent          = "tp.ui.discover.integration.awsoidc.connect"
	uiDiscoverDatabaseRDSEnrollEvent                  = "tp.ui.discover.database.enroll.rds"
	uiDiscoverKubeEKSEnrollEvent                      = "tp.ui.discover.kube.enroll.eks"
	uiDiscoverDeployServiceEvent                      = "tp.ui.discover.deployService"
	uiDiscoverDatabaseRegisterEvent                   = "tp.ui.discover.database.register"
	uiDiscoverDatabaseConfigureMTLSEvent              = "tp.ui.discover.database.configure.mtls"
	uiDiscoverDatabaseConfigureIAMPolicyEvent         = "tp.ui.discover.database.configure.iampolicy"
	uiDiscoverDesktopActiveDirectoryToolsInstallEvent = "tp.ui.discover.desktop.activeDirectory.tools.install"
	uiDiscoverDesktopActiveDirectoryConfigureEvent    = "tp.ui.discover.desktop.activeDirectory.configure"
	uiDiscoverAutoDiscoveredResourcesEvent            = "tp.ui.discover.autoDiscoveredResources"
	uiDiscoverCreateAppServerEvent                    = "tp.ui.discover.createAppServer"
	uiDiscoverCreateDiscoveryConfigEvent              = "tp.ui.discover.createDiscoveryConfig"
	uiDiscoverPrincipalsConfigureEvent                = "tp.ui.discover.principals.configure"
	uiDiscoverTestConnectionEvent                     = "tp.ui.discover.testConnection"
	uiDiscoverCompletedEvent                          = "tp.ui.discover.completed"

	uiIntegrationEnrollStartEvent    = "tp.ui.integrationEnroll.start"
	uiIntegrationEnrollCompleteEvent = "tp.ui.integrationEnroll.complete"
	uiIntegrationEnrollStepEvent     = "tp.ui.integrationEnroll.step"

	uiCallToActionClickEvent = "tp.ui.callToAction.click"

	uiAccessGraphCrownJewelDiffViewEvent = "tp.ui.accessGraph.crownJewelDiffView"

	featureRecommendationEvent = "tp.ui.feature.recommendation"
)

// Events that require extra metadata.
var eventsWithDataRequired = []string{
	uiDiscoverStartedEvent,
	uiDiscoverResourceSelectionEvent,
	uiDiscoverDeployServiceEvent,
	uiDiscoverDatabaseRegisterEvent,
	uiDiscoverDatabaseConfigureMTLSEvent,
	uiDiscoverDatabaseConfigureIAMPolicyEvent,
	uiDiscoverDesktopActiveDirectoryToolsInstallEvent,
	uiDiscoverDesktopActiveDirectoryConfigureEvent,
	uiDiscoverAutoDiscoveredResourcesEvent,
	uiDiscoverPrincipalsConfigureEvent,
	uiDiscoverTestConnectionEvent,
	uiDiscoverCompletedEvent,
	uiDiscoverIntegrationAWSOIDCConnectEvent,
	uiDiscoverDatabaseRDSEnrollEvent,
	uiDiscoverKubeEKSEnrollEvent,
	uiIntegrationEnrollStartEvent,
	uiIntegrationEnrollCompleteEvent,
	uiIntegrationEnrollStepEvent,
	uiDiscoverCreateDiscoveryConfigEvent,
	uiAccessGraphCrownJewelDiffViewEvent,
}

// CreatePreUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
// the username is required for pre-user events
type CreatePreUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Username token is set for unauthenticated event requests
	Username string `json:"username"`

	// Alert is the alert clicked via the UI banner
	// Alert is only set for bannerClick events
	Alert string `json:"alert"`
	// MfaType is the type of MFA used
	// MfaType is only set for registerChallenge events
	MfaType string `json:"mfaType"`
	// LoginFlow is the login flow used
	// LoginFlow is only set for registerChallenge events
	LoginFlow string `json:"loginFlow"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *CreatePreUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	if r.Username == "" {
		return trace.BadParameter("missing required parameter Username")
	}

	return nil
}

func ConvertPreUserEventRequestToUsageEvent(req CreatePreUserEventRequest) (*usageeventsv1.UsageEventOneOf, error) {
	typedEvent := usageeventsv1.UsageEventOneOf{}
	switch req.Event {
	case setCredentialSubmitEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &usageeventsv1.UIOnboardSetCredentialSubmitEvent{
				Username: req.Username,
			},
		}
	case registerChallengeSubmitEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiOnboardRegisterChallengeSubmit{
			UiOnboardRegisterChallengeSubmit: &usageeventsv1.UIOnboardRegisterChallengeSubmitEvent{
				Username:  req.Username,
				MfaType:   req.MfaType,
				LoginFlow: req.LoginFlow,
			},
		}
	case recoveryCodesContinueClickEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &usageeventsv1.UIRecoveryCodesContinueClickEvent{
				Username: req.Username,
			},
		}
	case recoveryCodesCopyClickEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiRecoveryCodesCopyClick{
			UiRecoveryCodesCopyClick: &usageeventsv1.UIRecoveryCodesCopyClickEvent{
				Username: req.Username,
			},
		}
	case recoveryCodesPrintClickEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiRecoveryCodesPrintClick{
			UiRecoveryCodesPrintClick: &usageeventsv1.UIRecoveryCodesPrintClickEvent{
				Username: req.Username,
			},
		}
	case questionnaireSubmitEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiOnboardQuestionnaireSubmit{
			UiOnboardQuestionnaireSubmit: &usageeventsv1.UIOnboardQuestionnaireSubmitEvent{
				Username: req.Username,
			},
		}
	case completeGoToDashboardClickEvent:
		typedEvent.Event = &usageeventsv1.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick{
			UiOnboardCompleteGoToDashboardClick: &usageeventsv1.UIOnboardCompleteGoToDashboardClickEvent{
				Username: req.Username,
			},
		}
	default:
		return nil, trace.BadParameter("invalid event %s", req.Event)
	}

	return &typedEvent, nil
}

// CreateUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
type CreateUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Alert is a banner click event property
	Alert string `json:"alert"`

	// EventData contains the event's metadata.
	// This field dependes on the Event name, hence the json.RawMessage
	EventData *json.RawMessage `json:"eventData"`
}

// IntegrationEnrollEventData contains the required properties
// to create a IntegrationEnroll UsageEvent.
type IntegrationEnrollEventData struct {
	// ID is a unique ID per wizard session
	ID string `json:"id"`

	// Kind is the integration type that the user selected to enroll.
	// Values should be the string version of the enum names as found
	// in usageevents.IntegrationEnrollKind.
	// Example: "INTEGRATION_ENROLL_KIND_AWS_OIDC"
	Kind string `json:"kind"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *CreateUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	if slices.Contains(eventsWithDataRequired, r.Event) && r.EventData == nil {
		return trace.BadParameter("eventData is required")
	}

	return nil
}

// ConvertUserEventRequestToUsageEvent receives a CreateUserEventRequest and
// creates a new *usageeventsv1.UsageEventOneOf. Based on the event's name, it
// creates the corresponding *usageeventsv1.UsageEventOneOf adding the required
// fields.
func ConvertUserEventRequestToUsageEvent(req CreateUserEventRequest) (*usageeventsv1.UsageEventOneOf, error) {
	switch req.Event {
	case bannerClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiBannerClick{
				UiBannerClick: &usageeventsv1.UIBannerClickEvent{
					Alert: req.Alert,
				},
			}},
			nil

	case addFirstResourceClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceClick{
				UiOnboardAddFirstResourceClick: &usageeventsv1.UIOnboardAddFirstResourceClickEvent{},
			}},
			nil

	case addFirstResourceLaterClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick{
				UiOnboardAddFirstResourceLaterClick: &usageeventsv1.UIOnboardAddFirstResourceLaterClickEvent{},
			}},
			nil

	case uiIntegrationEnrollStartEvent,
		uiIntegrationEnrollCompleteEvent:

		var event IntegrationEnrollEventData
		if err := json.Unmarshal([]byte(*req.EventData), &event); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		kindEnum, ok := usageeventsv1.IntegrationEnrollKind_value[event.Kind]
		if !ok {
			return nil, trace.BadParameter("invalid integration enroll kind %s", event.Kind)
		}

		switch req.Event {
		case uiIntegrationEnrollStartEvent:
			return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStartEvent{
				UiIntegrationEnrollStartEvent: &usageeventsv1.UIIntegrationEnrollStartEvent{
					Metadata: &usageeventsv1.IntegrationEnrollMetadata{
						Id:   event.ID,
						Kind: usageeventsv1.IntegrationEnrollKind(kindEnum),
					},
				},
			}}, nil
		case uiIntegrationEnrollCompleteEvent:
			return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollCompleteEvent{
				UiIntegrationEnrollCompleteEvent: &usageeventsv1.UIIntegrationEnrollCompleteEvent{
					Metadata: &usageeventsv1.IntegrationEnrollMetadata{
						Id:   event.ID,
						Kind: usageeventsv1.IntegrationEnrollKind(kindEnum),
					},
				},
			}}, nil
		}
	case uiIntegrationEnrollStepEvent:
		eventData := struct {
			ID     string `json:"id"`
			Kind   string `json:"kind"`
			Step   string `json:"step"`
			Status struct {
				Code  string `json:"code"`
				Error string `json:"error"`
			} `json:"status"`
		}{}
		if err := json.Unmarshal([]byte(*req.EventData), &eventData); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		kindEnum, ok := usageeventsv1.IntegrationEnrollKind_value[eventData.Kind]
		if !ok {
			return nil, trace.BadParameter("invalid integration enroll kind %s", eventData.Kind)
		}

		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStepEvent{
			UiIntegrationEnrollStepEvent: &usageeventsv1.UIIntegrationEnrollStepEvent{
				Metadata: &usageeventsv1.IntegrationEnrollMetadata{
					Id:   eventData.ID,
					Kind: usageeventsv1.IntegrationEnrollKind(kindEnum),
				},
				Step: usageeventsv1.IntegrationEnrollStep(usageeventsv1.IntegrationEnrollStep_value[eventData.Step]),
				Status: &usageeventsv1.IntegrationEnrollStepStatus{
					Code:  usageeventsv1.IntegrationEnrollStatusCode(usageeventsv1.IntegrationEnrollStatusCode_value[eventData.Status.Code]),
					Error: eventData.Status.Error,
				},
			},
		}}, nil
	case uiDiscoverStartedEvent,
		uiDiscoverResourceSelectionEvent,
		uiDiscoverIntegrationAWSOIDCConnectEvent,
		uiDiscoverDatabaseRDSEnrollEvent,
		uiDiscoverKubeEKSEnrollEvent,
		uiDiscoverDeployServiceEvent,
		uiDiscoverDatabaseRegisterEvent,
		uiDiscoverDatabaseConfigureMTLSEvent,
		uiDiscoverDatabaseConfigureIAMPolicyEvent,
		uiDiscoverDesktopActiveDirectoryToolsInstallEvent,
		uiDiscoverDesktopActiveDirectoryConfigureEvent,
		uiDiscoverAutoDiscoveredResourcesEvent,
		uiDiscoverPrincipalsConfigureEvent,
		uiDiscoverTestConnectionEvent,
		uiDiscoverCreateAppServerEvent,
		uiDiscoverCreateDiscoveryConfigEvent,
		uiDiscoverCompletedEvent:

		var discoverEvent DiscoverEventData
		if err := json.Unmarshal([]byte(*req.EventData), &discoverEvent); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		event, err := discoverEvent.ToUsageEvent(req.Event)
		if err != nil {
			return nil, trace.BadParameter("failed to convert eventData: %v", err)
		}
		return event, nil

	case createNewRoleClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiCreateNewRoleClick{
				UiCreateNewRoleClick: &usageeventsv1.UICreateNewRoleClickEvent{},
			}},
			nil

	case createNewRoleSaveClickEvent:
		roleEvent := struct {
			StandardUsed               bool     `json:"standardUsed"`
			YAMLUsed                   bool     `json:"yamlUsed"`
			ModeWhenSaved              string   `json:"modeWhenSaved"`
			FieldsWithConversionErrors []string `json:"fieldsWithConversionErrors"`
		}{}
		if err := json.Unmarshal([]byte(*req.EventData), &roleEvent); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiCreateNewRoleSaveClick{
				UiCreateNewRoleSaveClick: &usageeventsv1.UICreateNewRoleSaveClickEvent{
					StandardUsed:               roleEvent.StandardUsed,
					YamlUsed:                   roleEvent.YAMLUsed,
					ModeWhenSaved:              roleEvent.ModeWhenSaved,
					FieldsWithConversionErrors: roleEvent.FieldsWithConversionErrors,
				},
			}},
			nil

	case createNewRoleCancelClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiCreateNewRoleCancelClick{
				UiCreateNewRoleCancelClick: &usageeventsv1.UICreateNewRoleCancelClickEvent{},
			}},
			nil

	case createNewRoleViewDocumentationClickEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiCreateNewRoleViewDocumentationClick{
				UiCreateNewRoleViewDocumentationClick: &usageeventsv1.UICreateNewRoleViewDocumentationClickEvent{},
			}},
			nil

	case uiCallToActionClickEvent:
		var cta int32
		if err := json.Unmarshal([]byte(*req.EventData), &cta); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiCallToActionClickEvent{
				UiCallToActionClickEvent: &usageeventsv1.UICallToActionClickEvent{
					Cta: usageeventsv1.CTA(cta),
				}}},
			nil

	case questionnaireSubmitEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiOnboardQuestionnaireSubmit{
				UiOnboardQuestionnaireSubmit: &usageeventsv1.UIOnboardQuestionnaireSubmitEvent{},
			}},
			nil

	case featureRecommendationEvent:
		event := struct {
			Feature                     int32 `json:"feature"`
			FeatureRecommendationStatus int32 `json:"featureRecommendationStatus"`
		}{}

		if err := json.Unmarshal([]byte(*req.EventData), &event); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_FeatureRecommendationEvent{
				FeatureRecommendationEvent: &usageeventsv1.FeatureRecommendationEvent{
					Feature:                     usageeventsv1.Feature(event.Feature),
					FeatureRecommendationStatus: usageeventsv1.FeatureRecommendationStatus(event.FeatureRecommendationStatus),
				},
			}},
			nil
	case uiAccessGraphCrownJewelDiffViewEvent:
		event := struct {
			AffectedResourceSource string `json:"affected_resource_source,omitempty"`
			AffectedResourceType   string `json:"affected_resource_type,omitempty"`
		}{}
		if err := json.Unmarshal(*req.EventData, &event); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}
		if event.AffectedResourceType == "" {
			return nil, trace.BadParameter("affected resource type is empty")
		}
		if event.AffectedResourceSource == "" {
			return nil, trace.BadParameter("affected resource source is empty")
		}
		return &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_UiAccessGraphCrownJewelDiffView{
				UiAccessGraphCrownJewelDiffView: &usageeventsv1.UIAccessGraphCrownJewelDiffViewEvent{
					AffectedResourceSource: event.AffectedResourceSource,
					AffectedResourceType:   event.AffectedResourceType,
				},
			},
		}, nil
	}

	return nil, trace.BadParameter("invalid event %s", req.Event)
}
