import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'e-teleport/useTeleportE';

export function useIntegrationEnroll() {
  const ctx = useTeleport();

  const hasPluginAccess = ctx.storeUser.getPluginsAccess().create;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;
  const { attempt, run } = useAttempt(hasPluginAccess ? 'processing' : '');

  const [types, setTypes] = useState<{
    availableTypes: string[];
    existingTypes: string[];
  }>({ availableTypes: [], existingTypes: [] });

  async function fetchTypes() {
    const [availableTypes, existingTypes] = await Promise.all([
      ctx.pluginsService.fetchAvailableTypes(),
      ctx.pluginsService
        .fetchPlugins()
        .then(response => response.map(p => p.kind)),
    ]);
    setTypes({ availableTypes, existingTypes });
  }

  useEffect(() => {
    if (hasPluginAccess) {
      run(() => fetchTypes());
    }
  }, []);

  return {
    availableTypes: types.availableTypes,
    existingTypes: types.existingTypes,
    attempt,
    run,
    hasPluginAccess,
    hasIntegrationAccess,
  };
}

export type State = ReturnType<typeof useIntegrationEnroll>;
