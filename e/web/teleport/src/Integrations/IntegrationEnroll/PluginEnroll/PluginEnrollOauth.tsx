import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router';

import {
  pluginTypeToIntegrationEnrollKind,
  type CloudHostablePlugin,
} from 'e-teleport/services/plugins';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { PluginEnrollFailedDialog } from './PluginEnrollFailedDialog';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';
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

export function PluginEnrollOAuth({ plugin }: { plugin: CloudHostablePlugin }) {
  const { search } = useLocation();
  const navigate = useNavigate();

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
    useState<OAuthPluginResponse | null>(() => {
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

  useEffect(() => {
    if (enrollResponse?.success && eventId) {
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
    if (!enrollResponse && eventId) {
      userEventService.captureIntegrationEnrollEvent({
        event: IntegrationEnrollEvent.Started,
        eventData: {
          id: eventId,
          kind: pluginTypeToIntegrationEnrollKind(plugin.type),
        },
      });
    }
    // Only send an event ID once.
  }, []);

  if (enrollResponse?.success) {
    return (
      <PluginEnrollSuccess
        plugin={plugin}
        oauthSuccessData={enrollResponse.success}
        installedPluginName={enrollResponse.success.name}
      />
    );
  }

  return (
    <>
      <SubmittablePluginForm plugin={plugin} eventId={eventId} />
      {enrollResponse?.error && (
        <PluginEnrollFailedDialog
          plugin={plugin}
          errorDescription={enrollResponse.errorDescription}
          clearError={clearPluginEnrollResponse}
        />
      )}
    </>
  );
}
