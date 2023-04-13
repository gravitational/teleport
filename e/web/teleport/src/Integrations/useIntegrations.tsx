import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { integrationService } from 'teleport/services/integrations';

import useTeleport from 'e-teleport/useTeleportE';

import type { Integration, Plugin } from 'teleport/services/integrations';

export function useIntegrations() {
  const ctx = useTeleport();
  const [items, setItems] = useState<(Plugin | Integration)[]>([]);
  const { attempt, run, setAttempt } = useAttempt('processing');
  // warning is used when a user has permissions to list both the
  // "integration" and "plugin" resource, but when fetching
  // only one resolved. This lets the user know why the listing
  // may not be complete.
  const [warning, setWarning] = useState('');
  const [operation, setOperation] = useState({
    type: 'none',
  } as Operation);

  useEffect(() => {
    // At least one of these access flag will be true since
    // either access will render the nav item that renders
    // the integrations screen. This means we will always
    // be fetching at least one resource.
    const hasPluginAccess = ctx.getFeatureFlags().plugins;
    const hasIntegrationAccess = ctx.getFeatureFlags().integrations;
    const hasAllAccess = hasPluginAccess && hasIntegrationAccess;

    // If the user had all list accesses and one of the fetch failed,
    // we will render a warning for user, while rendering the list
    // from the resolved promise. There can be two failure points:
    //   1) network error
    //   2) access error: user is missing a read access in one or both resources.
    // If both failed to fetch, we will render an error instead.
    if (hasAllAccess) {
      setAttempt({ status: 'processing' });
      Promise.allSettled([
        ctx.pluginsService.fetchPlugins(),
        integrationService.fetchIntegrations(),
      ]).then(responses => {
        const plugins = responses[0];
        const integrations = responses[1];
        let fetchedItems;

        if (
          plugins.status === 'fulfilled' &&
          integrations.status === 'fulfilled'
        ) {
          // Merge the responses into one
          fetchedItems = [...plugins.value, ...integrations.value];
        } else if (
          plugins.status === 'fulfilled' &&
          integrations.status === 'rejected'
        ) {
          fetchedItems = plugins.value;
          setWarning(`Failed to fetch rest of integrations (try refreshing browser or \
                  check your "integration" access): ${integrations.reason}`);
        } else if (
          integrations.status === 'fulfilled' &&
          plugins.status === 'rejected'
        ) {
          fetchedItems = integrations.value;
          setWarning(`Failed to fetch plugin integrations (try refreshing browser or \
                  check your "plugin" access): ${plugins.reason}`);
        } else if (
          // Explicitly check for rejected to satisfy typescript.
          plugins.status === 'rejected' &&
          integrations.status === 'rejected'
        ) {
          const pluginsErr = plugins.reason;
          const integegrationsErr = integrations.reason;
          setAttempt({
            status: 'failed',
            statusText: `An error has occurred. PLUGINS: ${pluginsErr}, INTEGRATIONS: ${integegrationsErr}`,
          });
          return;
        }

        if (fetchedItems) {
          setAttempt({ status: 'success' });
          setItems(fetchedItems);
        } else {
          // Should never reach here, but just in case.
          setAttempt({
            status: 'failed',
            statusText: `Failed to fetch. Try refreshing the browser`,
          });
        }
      });
      return;
    }

    if (hasPluginAccess) {
      run(() => ctx.pluginsService.fetchPlugins().then(setItems));
      return;
    }

    if (hasIntegrationAccess) {
      run(() => integrationService.fetchIntegrations().then(setItems));
      return;
    }
  }, []);

  function onCancelDelete() {
    setOperation({ type: 'none' });
  }

  function onDelete(plugin: Plugin) {
    return ctx.pluginsService.deletePlugin(plugin.name).then(() => {
      const updatedItems = items.filter(p => p.name !== plugin.name);
      setItems(updatedItems);
    });
  }

  function onStartDelete(plugin: Plugin) {
    setOperation({ type: 'delete', plugin });
  }

  return {
    items,
    attempt,
    run,
    operation,
    warning,
    onCancelDelete,
    onDelete,
    onStartDelete,
    canCreateIntegrations:
      ctx.storeUser.getPluginsAccess().create ||
      ctx.storeUser.getIntegrationsAccess().create,
  };
}

export type State = ReturnType<typeof useIntegrations>;

type Operation = { type: 'delete'; plugin: Plugin } | { type: 'none' };
