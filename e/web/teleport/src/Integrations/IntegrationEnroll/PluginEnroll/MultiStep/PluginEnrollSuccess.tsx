import { useEffect } from 'react';

import { pluginTypeToIntegrationEnrollKind } from 'e-teleport/services/plugins';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { PluginEnrollSuccess as Component } from '../PluginEnrollSuccess';
import { usePlugin } from './usePlugin';

export function PluginEnrollSuccess() {
  const {
    selectedPlugin,
    eventId,
    successPrimaryButtonUrl,
    successPrimaryButtonText,
    installedPlugin,
  } = usePlugin();

  useEffect(() => {
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Complete,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(selectedPlugin.type),
      },
    });

    // Only send an event ID once.
  }, []);
  return (
    <Component
      plugin={selectedPlugin}
      installedPluginName={installedPlugin.name}
      primaryButtonUrl={successPrimaryButtonUrl}
      primaryButtonText={successPrimaryButtonText}
    />
  );
}
