import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';
import { Plugin, PluginKind } from 'teleport/services/integrations';

import {
  CloudHostablePlugin,
  pluginTypeToIntegrationEnrollKind,
} from './plugins';
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
    return <PluginEnrollSuccess plugin={plugin} />;
  }

  return (
    <SubmittablePluginForm
      plugin={plugin}
      eventId={eventId}
      setStaticPluginResponse={setStaticPluginResponse}
    />
  );
}
