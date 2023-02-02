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

package web

import (
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/client/proto"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/httplib"
)

// these constants are 1:1 with user events found in the webapps codebase
// packages/teleport/src/services/userEvent/UserEvents/userEvents.ts
const (
	bannerClickEvent                = "tp.ui.banner.click"
	setCredentialSubmitEvent        = "tp.ui.onboard.setCredential.submit"
	registerChallengeSubmitEvent    = "tp.ui.onboard.registerChallenge.submit"
	addFirstResourceClickEvent      = "tp.ui.onboard.addFirstResource.click"
	addFirstResourceLaterClickEvent = "tp.ui.onboard.addFirstResourceLater.click"
	recoveryCodesContinueClickEvent = "tp.ui.recoveryCodesContinue.click"
	recoveryCodesCopyClickEvent     = "tp.ui.recoveryCodesCopy.click"
	recoveryCodesPrintClickEvent    = "tp.ui.recoveryCodesPrint.click"
	completeGoToDashboardClickEvent = "tp.ui.onboard.completeGoToDashboard.click"

	uiDiscoverStartedEvent           = "tp.ui.discover.started.click"
	uiDiscoverResourceSelectionEvent = "tp.ui.discover.resourceSelection.click"
)

// Events that require extra metadata.
var eventsWithDataRequired = []string{
	uiDiscoverStartedEvent,
	uiDiscoverResourceSelectionEvent,
}

// createPreUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
// the username is required for pre-user events
type createPreUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Username token is set for unauthenticated event requests
	Username string `json:"username"`

	// Alert is the alert clicked via the UI banner
	// Alert is only set for bannerClick events
	Alert string `json:"alert"`
	// MfaType is the type of MFA used
	// MfaType is only set for registerChallenge events
	MfaType string `json:"mfa_type"`
	// LoginFlow is the login flow used
	// LoginFlow is only set for registerChallenge events
	LoginFlow string `json:"login_flow"`
}

// createUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
type createUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Alert is a banner click event property
	Alert string `json:"alert"`

	// EventData contains the event's metadata.
	// This field dependes on the Event name, hence the json.RawMessage
	EventData *json.RawMessage `json:"eventData"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *createUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	if slices.Contains(eventsWithDataRequired, r.Event) && r.EventData == nil {
		return trace.BadParameter("eventData is required")
	}

	return nil
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *createPreUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
	}

	if r.Username == "" {
		return trace.BadParameter("missing required parameter Username")
	}

	return nil
}

// createPreUserEventHandle sends a user event to the UserEvent service
// this handler is for on-boarding user events pre-session
func (h *Handler) createPreUserEventHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req createPreUserEventRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client := h.cfg.ProxyClient

	typedEvent := v1.UsageEventOneOf{}
	switch req.Event {
	case setCredentialSubmitEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &v1.UIOnboardSetCredentialSubmitEvent{
				Username: req.Username,
			},
		}
	case registerChallengeSubmitEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardRegisterChallengeSubmit{
			UiOnboardRegisterChallengeSubmit: &v1.UIOnboardRegisterChallengeSubmitEvent{
				Username:  req.Username,
				MfaType:   req.MfaType,
				LoginFlow: req.LoginFlow,
			},
		}
	case recoveryCodesContinueClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &v1.UIRecoveryCodesContinueClickEvent{
				Username: req.Username,
			},
		}
	case recoveryCodesCopyClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiRecoveryCodesCopyClick{
			UiRecoveryCodesCopyClick: &v1.UIRecoveryCodesCopyClickEvent{
				Username: req.Username,
			},
		}
	case recoveryCodesPrintClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiRecoveryCodesPrintClick{
			UiRecoveryCodesPrintClick: &v1.UIRecoveryCodesPrintClickEvent{
				Username: req.Username,
			},
		}
	case completeGoToDashboardClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardCompleteGoToDashboardClick{
			UiOnboardCompleteGoToDashboardClick: &v1.UIOnboardCompleteGoToDashboardClickEvent{
				Username: req.Username,
			},
		}
	default:
		return nil, trace.BadParameter("invalid event %s", req.Event)
	}

	event := &proto.SubmitUsageEventRequest{
		Event: &typedEvent,
	}

	err := client.SubmitUsageEvent(r.Context(), event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// convertUserEventRequestToUsageEvent receives a createUserEventRequest and creates a new *v1.UsageEventOneOf.
// Based on the event's name, it creates the corresponding *v1.UsageEventOneOf adding the required fields.
func convertUserEventRequestToUsageEvent(req createUserEventRequest) (*v1.UsageEventOneOf, error) {
	switch req.Event {
	case bannerClickEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiBannerClick{
				UiBannerClick: &v1.UIBannerClickEvent{
					Alert: req.Alert,
				},
			}},
			nil

	case addFirstResourceClickEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiOnboardAddFirstResourceClick{
				UiOnboardAddFirstResourceClick: &v1.UIOnboardAddFirstResourceClickEvent{},
			}},
			nil

	case addFirstResourceLaterClickEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick{
				UiOnboardAddFirstResourceLaterClick: &v1.UIOnboardAddFirstResourceLaterClickEvent{},
			}},
			nil

	case uiDiscoverStartedEvent,
		uiDiscoverResourceSelectionEvent:

		var discoverEvent DiscoverEventData
		if err := json.Unmarshal([]byte(*req.EventData), &discoverEvent); err != nil {
			return nil, trace.BadParameter("eventData is invalid: %v", err)
		}

		event, err := discoverEvent.ToUsageEvent(req.Event)
		if err != nil {
			return nil, trace.BadParameter("failed to convert eventData: %v", err)
		}
		return event, nil

	}

	return nil, trace.BadParameter("invalid event %s", req.Event)
}

// createUserEventHandle sends a user event to the UserEvent service
// this handler is for user events with a session
func (h *Handler) createUserEventHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (interface{}, error) {
	var req createUserEventRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	typedEvent, err := convertUserEventRequestToUsageEvent(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &proto.SubmitUsageEventRequest{
		Event: typedEvent,
	}

	err = client.SubmitUsageEvent(r.Context(), event)
	if err != nil {
		return nil, trace.Wrap(err)

	}

	return nil, nil
}
