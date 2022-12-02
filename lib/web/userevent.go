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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/httplib"
)

// these constants are 1:1 with user events found in the webapps codebase
// packages/teleport/src/services/userEvent/UserEvents/userEvents.ts
const (
	bannerClickEvent                = "tp.ui.banner.click"
	getStartedClickEvent            = "tp.ui.onboard.getStarted.click"
	setCredentialSubmitEvent        = "tp.ui.onboard.setCredential.submit"
	registerChallengeSubmitEvent    = "tp.ui.onboard.registerChallenge.submit"
	recoveryCodesContinueClickEvent = "tp.ui.onboard.recoveryCodesContinue.click"
	addFirstResourceClickEvent      = "tp.ui.onboard.addFirstResource.click"
	addFirstResourceLaterClickEvent = "tp.ui.onboard.addFirstResourceLater.click"
)

// createPreUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
// the username is required for pre-user events
type createPreUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Alert is a banner click event property
	Alert string `json:"alert"`
	// Username token is set for unauthenticated event requests
	Username string `json:"username"`
}

// createUserEventRequest contains the event and properties associated with a user event
// the usageReporter convert event function will later set the timestamp
// and anonymize/set the cluster name
type createUserEventRequest struct {
	// Event describes the event being capture
	Event string `json:"event"`
	// Alert is a banner click event property
	Alert string `json:"alert"`
}

// CheckAndSetDefaults validates the Request has the required fields.
func (r *createUserEventRequest) CheckAndSetDefaults() error {
	if r.Event == "" {
		return trace.BadParameter("missing required parameter Event")
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
	case getStartedClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardGetStartedClick{
			UiOnboardGetStartedClick: &v1.UIOnboardGetStartedClickEvent{
				Username: req.Username,
			},
		}
	case setCredentialSubmitEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardSetCredentialSubmit{
			UiOnboardSetCredentialSubmit: &v1.UIOnboardSetCredentialSubmitEvent{
				Username: req.Username,
			},
		}
	case registerChallengeSubmitEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardRegisterChallengeSubmit{
			UiOnboardRegisterChallengeSubmit: &v1.UIOnboardRegisterChallengeSubmitEvent{
				Username: req.Username,
			},
		}
	case recoveryCodesContinueClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiRecoveryCodesContinueClick{
			UiRecoveryCodesContinueClick: &v1.UIRecoveryCodesContinueClickEvent{
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

	typedEvent := v1.UsageEventOneOf{}
	switch req.Event {
	case bannerClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiBannerClick{
			UiBannerClick: &v1.UIBannerClickEvent{
				Alert: req.Alert,
			},
		}
	case addFirstResourceClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardAddFirstResourceClick{
			UiOnboardAddFirstResourceClick: &v1.UIOnboardAddFirstResourceClickEvent{},
		}
	case addFirstResourceLaterClickEvent:
		typedEvent.Event = &v1.UsageEventOneOf_UiOnboardAddFirstResourceLaterClick{
			UiOnboardAddFirstResourceLaterClick: &v1.UIOnboardAddFirstResourceLaterClickEvent{},
		}
	default:
		return nil, trace.BadParameter("invalid event %s", req.Event)
	}

	event := &proto.SubmitUsageEventRequest{
		Event: &typedEvent,
	}

	err = client.SubmitUsageEvent(r.Context(), event)
	if err != nil {
		return nil, trace.Wrap(err)

	}

	return nil, nil
}
