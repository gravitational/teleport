import { useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router';

import { getSuccessPrimaryButtonState } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/redirect';
import {
  pluginTypeToIntegrationEnrollKind,
  type CloudHostablePlugin,
} from 'e-teleport/services/plugins';
import { Plugin, PluginKind } from 'teleport/services/integrations';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { PluginEnrollSuccess } from './PluginEnrollSuccess';
import { SubmittablePluginForm } from './SubmittablePluginForm';

export function PluginEnrollSingleStep({
  plugin,
}: {
  plugin: CloudHostablePlugin;
}) {
  const { type: selectedPluginType } = useParams<{ type: PluginKind }>();
  const [eventId] = useState(() => crypto.randomUUID());

  const [enrollResponse, setEnrollResponse] = useState<Plugin>();

  const location = useLocation();

  const { successPrimaryButtonText, successPrimaryButtonUrl } = useMemo(
    () => getSuccessPrimaryButtonState(location.search),
    [location.search]
  );

  function setStaticPluginResponse(registeredPlugin: Plugin) {
    setEnrollResponse(registeredPlugin);
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(selectedPluginType),
      },
    });
  }

  useEffect(() => {
    if (!enrollResponse && eventId) {
      userEventService.captureIntegrationEnrollEvent({
        event: IntegrationEnrollEvent.Started,
        eventData: {
          id: eventId,
          kind: pluginTypeToIntegrationEnrollKind(selectedPluginType),
        },
      });
    }
    // Only send a start event ID once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (enrollResponse) {
    return (
      <PluginEnrollSuccess
        plugin={plugin}
        primaryButtonUrl={successPrimaryButtonUrl}
        primaryButtonText={successPrimaryButtonText}
        installedPluginName={enrollResponse.name}
      />
    );
  }

  return (
    <SubmittablePluginForm
      plugin={plugin}
      eventId={eventId}
      setStaticPluginResponse={setStaticPluginResponse}
    />
  );
}
