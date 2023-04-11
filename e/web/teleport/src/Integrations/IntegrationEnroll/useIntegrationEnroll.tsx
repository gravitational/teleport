import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'e-teleport/useTeleportE';

export function useIntegrationEnroll() {
  const ctx = useTeleport();
  const [types, setTypes] = useState<{
    availableTypes: string[];
    existingTypes: string[];
  }>({ availableTypes: [], existingTypes: [] });
  const { attempt, run } = useAttempt('processing');

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
    run(() => fetchTypes());
  }, []);

  return {
    availableTypes: types.availableTypes,
    existingTypes: types.existingTypes,
    attempt,
    run,
  };
}

export type State = ReturnType<typeof useIntegrationEnroll>;
