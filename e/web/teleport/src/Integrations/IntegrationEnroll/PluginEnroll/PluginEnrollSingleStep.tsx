import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router';

import { getSuccessPrimaryButtonState } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/redirect';
import {
  pluginTypeToIntegrationEnrollKind,
  type CloudHostablePlugin,
} from 'e-teleport/services/plugins';
import { Plugin, PluginKindToSpec } from 'teleport/services/integrations';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { PluginEnrollFailedDialog } from './PluginEnrollFailedDialog';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';
import { SubmittablePluginForm } from './SubmittablePluginForm';

/**
 * PluginRegistered contains data from a successful
 * enrollment. This is mainly used for `NextSteps` view
 * to specify additional steps based on the plugin configuration.
 */
export type PluginRegistered = {
  name: string;
  slack?: { fallback_channel: string };
};

/**
 * OAuthPluginResponse specifies enrollment response
 * using OAuth 2.0 flow (eg. slack).
 */
type OAuthPluginResponse = {
  kind: 'oauth';
  error?: string | null;
  errorDescription: string | null;
  success?: PluginRegistered;
};

/**
 * StaticPluginResponse specifies enrollment response
 * using static credentials.
 */
type StaticPluginResponse = {
  kind: 'static';
  success?: PluginRegistered;
};

type PluginEnrollResponse = StaticPluginResponse | OAuthPluginResponse;

/**
 * isOauthRedirect checks that the URL params populated
 * from a redirect contain oauth response data.
 */
function isOauthRedirect(params: URLSearchParams) {
  return params.has('success') || params.has('error');
}

export function PluginEnrollSingleStep({
  plugin,
}: {
  plugin: CloudHostablePlugin;
}) {
  const location = useLocation();
  const navigate = useNavigate();

  const { successPrimaryButtonText, successPrimaryButtonUrl } = useMemo(
    () => getSuccessPrimaryButtonState(location.search),
    [location.search]
  );

  // URL params are only apparent when a user got redirected back to this
  // view after getting a response from an oauth provider callback.
  const params = new URLSearchParams(location.search);

  const [eventId] = useState(() => {
    // We want state `eventId` to be null when it re-renders from
    // removing the `event_id` param from the search state of the URL,
    // so it doesn't attempt to send another event.
    if (isOauthRedirect(params)) {
      return params.get('event_id');
    }
    return crypto.randomUUID();
  });

  const [enrollResponse, setEnrollResponse] =
    useState<PluginEnrollResponse | null>(() => {
      // Set an oauth enrollment response if the user got redirected
      // back to this view.
      if (!isOauthRedirect(params)) {
        return;
      }

      const error = params.get('error');
      const errorDescription = params.get('error_description');

      let success;
      try {
        success = JSON.parse(params.get('success'));
      } catch (e) {
        console.error(
          `[PluginEnroll.tsx] failed to parse 'success' JSON from URL: ${e}`
        );
      }

      return {
        kind: 'oauth',
        error,
        errorDescription,
        success,
      };
    });

  function clearPluginEnrollResponse() {
    setEnrollResponse(null);
  }

  function setStaticPluginResponse(registeredPlugin: Plugin) {
    let success: PluginRegistered = {
      name: registeredPlugin.name,
    };

    // Set plugin-specific success response, used for `NextSteps` view.
    switch (registeredPlugin.kind) {
      case 'slack':
        success.slack = {
          fallback_channel: (registeredPlugin.spec as PluginKindToSpec['slack'])
            .fallbackChannel,
        };
        break;
    }

    setEnrollResponse({ kind: 'static', success: success });
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(plugin.type),
      },
    });
  }

  useEffect(() => {
    if (enrollResponse?.kind === 'oauth' && enrollResponse.success && eventId) {
      userEventService.captureIntegrationEnrollEvent({
        event: IntegrationEnrollEvent.Complete,
        eventData: {
          id: eventId,
          kind: pluginTypeToIntegrationEnrollKind(plugin.type),
        },
      });

      params.delete('event_id');
      navigate({ search: params.toString() }, { replace: true });
    }

    if (enrollResponse || !eventId || plugin.type === 'okta') {
      return;
    }
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Started,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(plugin.type),
      },
    });
    // Only send a start event ID once.
  }, []);

  if (enrollResponse?.success) {
    return (
      <PluginEnrollSuccess
        plugin={plugin}
        primaryButtonUrl={successPrimaryButtonUrl}
        primaryButtonText={successPrimaryButtonText}
        successData={enrollResponse.success}
        installedPluginName={enrollResponse.success.name}
      />
    );
  }

  return (
    <>
      <SubmittablePluginForm
        plugin={plugin}
        eventId={eventId}
        setStaticPluginResponse={setStaticPluginResponse}
      />
      {enrollResponse?.kind === 'oauth' && enrollResponse.error && (
        <PluginEnrollFailedDialog
          plugin={plugin}
          errorDescription={enrollResponse.errorDescription}
          clearError={clearPluginEnrollResponse}
        />
      )}
    </>
  );
}
