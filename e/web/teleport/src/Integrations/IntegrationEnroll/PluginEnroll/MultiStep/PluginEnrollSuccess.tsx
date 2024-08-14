import React, { useEffect } from 'react';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { pluginTypeToIntegrationEnrollKind } from 'e-teleport/services/plugins';

import { PluginEnrollSuccess as Component } from '../PluginEnrollSuccess';

import { usePlugin } from './usePlugin';

export function PluginEnrollSuccess() {
  const { selectedPlugin, eventId } = usePlugin();

  useEffect(() => {
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(selectedPlugin.type),
      },
    });

    // Only send an event ID once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return <Component plugin={selectedPlugin} />;
}
