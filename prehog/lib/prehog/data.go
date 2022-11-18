package prehog

import (
	"reflect"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/rs/zerolog/log"

	prehogv1 "github.com/gravitational/prehog/gen/proto/prehog/v1alpha"
	"github.com/gravitational/prehog/lib/license"
	"github.com/gravitational/prehog/lib/posthog"
)

const (
	userLoginEvent      posthog.EventName = "tp.user.login"
	ssoCreateEvent      posthog.EventName = "tp.sso.create"
	resourceCreateEvent posthog.EventName = "tp.resource.create"
	sessionStartEvent   posthog.EventName = "tp.session.start"

	uiBannerClickEvent                       posthog.EventName = "tp.ui.banner.click"
	uiOnboardDomainNameTCSubmitEvent         posthog.EventName = "tp.ui.onboard.domainNameTC.submit"
	uiOnboardGoToDashboardClickEvent         posthog.EventName = "tp.ui.onboard.goToDashboard.click"
	uiOnboardGetStartedClickEvent            posthog.EventName = "tp.ui.onboard.getStarted.click"
	uiOnboardCompleteGoToDashboardClickEvent posthog.EventName = "tp.ui.onboard.completeGoToDashboard.click"
	uiOnboardAddFirstResourceClickEvent      posthog.EventName = "tp.ui.onboard.addFirstResource.click"
	uiOnboardAddFirstResourceLaterClickEvent posthog.EventName = "tp.ui.onboard.addFirstResourceLater.click"
	uiOnboardSetCredentialSubmitEvent        posthog.EventName = "tp.ui.onboard.setCredential.submit" //#nosec G101 -- not hardcoded credentials
	uiOnboardRegisterChallengeSubmitEvent    posthog.EventName = "tp.ui.onboard.registerChallenge.submit"
	uiOnboardRecoveryCodesContinueClickEvent posthog.EventName = "tp.ui.onboard.recoveryCodesContinue.click"
)

const (
	clusterNameProperty posthog.EventProperty = "tp.cluster_name"
	licenseNameProperty posthog.EventProperty = "tp.license_name"
	accountIDProperty   posthog.EventProperty = "tp.account_id"
	isCloudProperty     posthog.EventProperty = "tp.is_cloud"

	userNameProperty      posthog.EventProperty = "tp.user_name"
	connectorTypeProperty posthog.EventProperty = "tp.connector_type"
	resourceTypeProperty  posthog.EventProperty = "tp.resource_type"
	sessionTypeProperty   posthog.EventProperty = "tp.session_type"
	alertProperty         posthog.EventProperty = "tp.alert"
)

const (
	firstLoginProperty    posthog.PersonProperty = "tp.first_login"
	firstSSOLoginProperty posthog.PersonProperty = "tp.first_sso_login"
	firstResourceProperty posthog.PersonProperty = "tp.first_resource"
	firstSessionProperty  posthog.PersonProperty = "tp.first_session"

	licenseNamePersonProperty posthog.PersonProperty = posthog.PersonProperty(licenseNameProperty)
	accountIDPersonProperty   posthog.PersonProperty = posthog.PersonProperty(accountIDProperty)
	isCloudPersonProperty     posthog.PersonProperty = posthog.PersonProperty(isCloudProperty)
	isTrialProperty           posthog.PersonProperty = "tp.is_trial"
)

const distinctIDPrefix = "tp."

// encodeSubmitEvent returns the PostHog encoding of the event in the
// SubmitEventRequest. Errors are all *connect.Error for now.
func encodeSubmitEvent(lic license.License, req *prehogv1.SubmitEventRequest) (*posthog.Event, error) {
	distinctID := distinctIDPrefix + lic.AccountID()

	clusterName := req.GetClusterName()
	if clusterName == "" {
		return nil, invalidArgument("missing cluster_name")
	}

	if req.GetEvent() == nil {
		return nil, invalidArgument("missing event")
	}

	var timestamp time.Time
	if req.GetTimestamp() != nil {
		// TODO(espadolini): sanity check on timestamp?
		timestamp = req.GetTimestamp().AsTime()
	} else {
		timestamp = time.Now().UTC()
	}

	e := &posthog.Event{
		DistinctID: distinctID,
		Timestamp:  timestamp,
		Properties: map[posthog.EventProperty]any{
			clusterNameProperty: clusterName,
			licenseNameProperty: lic.Metadata.Name,
			accountIDProperty:   lic.Spec.AccountID,
			isCloudProperty:     lic.Spec.Cloud,
		},
	}

	switch oneof := req.GetEvent().(type) {
	case *prehogv1.SubmitEventRequest_UserLogin:
		userName := oneof.UserLogin.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = userLoginEvent
		e.AddProperty(userNameProperty, userName)
		e.AddSetOnce(firstLoginProperty, timestamp)

		if ct := oneof.UserLogin.GetConnectorType(); ct != "" {
			e.AddProperty(connectorTypeProperty, ct)
			e.AddSetOnce(firstSSOLoginProperty, timestamp)
		}

	case *prehogv1.SubmitEventRequest_SsoCreate:
		connectorType := oneof.SsoCreate.GetConnectorType()
		if connectorType == "" {
			return nil, invalidArgument("missing connector_type")
		}

		e.Event = ssoCreateEvent
		e.AddProperty(connectorTypeProperty, connectorType)

	case *prehogv1.SubmitEventRequest_ResourceCreate:
		resourceType := oneof.ResourceCreate.GetResourceType()
		if resourceType == "" {
			return nil, invalidArgument("missing resource_type")
		}

		e.Event = resourceCreateEvent
		e.AddProperty(resourceTypeProperty, resourceType)

	case *prehogv1.SubmitEventRequest_SessionStart:
		userName := oneof.SessionStart.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}
		sessionType := oneof.SessionStart.GetSessionType()
		if sessionType == "" {
			return nil, invalidArgument("missing session_type")
		}

		e.Event = sessionStartEvent
		e.AddProperty(userNameProperty, userName)
		e.AddProperty(sessionTypeProperty, sessionType)
		e.AddSetOnce(firstSessionProperty, timestamp)

	case *prehogv1.SubmitEventRequest_UiBannerClick:
		userName := oneof.UiBannerClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		alert := oneof.UiBannerClick.GetAlert()
		if alert == "" {
			return nil, invalidArgument("missing alert")
		}

		e.Event = uiBannerClickEvent
		e.AddProperty(userNameProperty, userName)
		e.AddProperty(alertProperty, alert)

	case *prehogv1.SubmitEventRequest_UiOnboardGetStartedClick:
		userName := oneof.UiOnboardGetStartedClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardGetStartedClickEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardCompleteGoToDashboardClick:
		userName := oneof.UiOnboardCompleteGoToDashboardClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardCompleteGoToDashboardClickEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceClick:
		userName := oneof.UiOnboardAddFirstResourceClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardAddFirstResourceClickEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardAddFirstResourceLaterClick:
		userName := oneof.UiOnboardAddFirstResourceLaterClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardAddFirstResourceLaterClickEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardSetCredentialSubmit:
		userName := oneof.UiOnboardSetCredentialSubmit.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardSetCredentialSubmitEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardRegisterChallengeSubmit:
		userName := oneof.UiOnboardRegisterChallengeSubmit.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardRegisterChallengeSubmitEvent
		e.AddProperty(userNameProperty, userName)

	case *prehogv1.SubmitEventRequest_UiOnboardRecoveryCodesContinueClick:
		userName := oneof.UiOnboardRecoveryCodesContinueClick.GetUserName()
		if userName == "" {
			return nil, invalidArgument("missing user_name")
		}

		e.Event = uiOnboardRecoveryCodesContinueClickEvent
		e.AddProperty(userNameProperty, userName)
	}

	if e.Event == "" {
		log.Error().Stringer("type", reflect.TypeOf(req.GetEvent())).Msg("Unknown event type")
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	return e, nil
}

// encodeSubmitSalesEvent returns the PostHog encoding of the event in the
// SubmitSalesEventRequest. Errors are all *connect.Error for now.
func encodeSubmitSalesEvent(req *prehogv1.SubmitSalesEventRequest) (*posthog.Event, error) {
	accountID := req.GetAccountId()
	if accountID == "" {
		return nil, invalidArgument("missing account_id")
	}

	if req.GetEvent() == nil {
		return nil, invalidArgument("missing event")
	}

	var timestamp time.Time
	if req.GetTimestamp() != nil {
		// TODO(espadolini): sanity check on timestamp?
		timestamp = req.GetTimestamp().AsTime()
	} else {
		timestamp = time.Now().UTC()
	}

	e := &posthog.Event{
		DistinctID: distinctIDPrefix + accountID,
		Timestamp:  timestamp,
		Properties: map[posthog.EventProperty]any{
			accountIDProperty: accountID,
		},
	}

	switch req.GetEvent().(type) {
	case *prehogv1.SubmitSalesEventRequest_UiOnboardDomainNameTcSubmit:
		e.Event = uiOnboardDomainNameTCSubmitEvent

	case *prehogv1.SubmitSalesEventRequest_UiOnboardGoToDashboardClick:
		e.Event = uiOnboardGoToDashboardClickEvent
	}

	if e.Event == "" {
		log.Error().Stringer("type", reflect.TypeOf(req.GetEvent())).Msg("Unknown event type")
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	return e, nil
}
