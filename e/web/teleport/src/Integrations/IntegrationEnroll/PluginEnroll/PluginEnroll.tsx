import React, { useEffect, useState } from 'react';
import { useParams, useLocation, useHistory } from 'react-router';
import { NotFound } from 'design/CardError';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';
import { Plugin, PluginKind } from 'teleport/services/integrations';

import { pluginMap, pluginTypeToIntegrationEnrollKind } from './plugins';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';
import { PluginEnrollFailedDialog } from './PluginEnrollFailedDialog';
import { SubmittablePluginForm } from './SubmittablePluginForm';

export type OAuthPluginRegistered = {
  name: string;
  eventId: string;
  slack?: { fallback_channel: string };
};

type OAuthPluginResponse = {
  kind: 'oauth';
  error?: string | null;
  errorDescription: string | null;
  success?: OAuthPluginRegistered;
};

type StaticPluginResponse = {
  kind: 'static';
  plugin?: Plugin;
};

export type PluginEnrollResponse = StaticPluginResponse | OAuthPluginResponse;

export function PluginEnroll() {
  const { type: selectedPluginType } = useParams<{ type: PluginKind }>();
  const { search } = useLocation();
  const history = useHistory();

  // URL params are only apparent when a user got redirected back to this
  // view after getting a response from an oauth provider callback.
  const params = new URLSearchParams(search);

  const [eventId] = useState(() => {
    // We want state `eventId` to be null when it re-renders from
    // removing the `event_id` param from the search state of the URL,
    // so it doesn't attempt to send another event.
    if (params.toString().length) {
      return params.get('event_id');
    }
    return crypto.randomUUID();
  });

  const [enrollResponse, setEnrollResponse] =
    useState<PluginEnrollResponse | null>(() => {
      if (!params.toString().length) {
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
    setEnrollResponse({ kind: 'static', plugin: registeredPlugin });
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(selectedPluginType),
      },
    });
  }

  useEffect(() => {
    if (enrollResponse?.kind === 'oauth' && enrollResponse.success && eventId) {
      userEventService.captureIntegrationEnrollEvent({
        event: IntegrationEnrollEvent.Complete,
        eventData: {
          id: eventId,
          kind: pluginTypeToIntegrationEnrollKind(selectedPluginType),
        },
      });

      params.delete('event_id');
      history.replace({
        search: params.toString(),
      });
    }

    if (!enrollResponse && eventId) {
      userEventService.captureIntegrationEnrollEvent({
        event: IntegrationEnrollEvent.Started,
        eventData: {
          id: eventId,
          kind: pluginTypeToIntegrationEnrollKind(selectedPluginType),
        },
      });
    }
    // Only send an event ID once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const plugin = pluginMap[selectedPluginType];
  if (!plugin || !plugin.cloudHostable) {
    return (
      <NotFound
        message={`plugin type ${selectedPluginType} is either not \
        defined in pluginMap or not hostable`}
      />
    );
  }

  if (enrollResponse?.kind === 'static') {
    return <PluginEnrollSuccess plugin={plugin} />;
  }

  if (enrollResponse?.kind === 'oauth' && enrollResponse.success) {
    return (
      <PluginEnrollSuccess
        plugin={plugin}
        oauthSuccessData={enrollResponse.success}
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
