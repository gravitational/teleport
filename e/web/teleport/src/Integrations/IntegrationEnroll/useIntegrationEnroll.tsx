import { useEffect, useState } from 'react';
import { useParams, useLocation } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';

import { IntegrationKind } from 'teleport/services/integrations';

import useTeleport from 'e-teleport/useTeleportE';

import { PluginTypes } from '../data';

export function useIntegrationEnroll() {
  const ctx = useTeleport();

  const hasPluginAccess = ctx.storeUser.getPluginsAccess().create;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;
  const { attempt, run } = useAttempt(hasPluginAccess ? 'processing' : '');

  const [types, setTypes] = useState<{
    availableTypes: string[];
    existingTypes: string[];
  }>({ availableTypes: [], existingTypes: [] });

  const { type: selectedType } =
    useParams<{ type: PluginTypes | IntegrationKind }>();

  const { search } = useLocation();
  const params = new URLSearchParams(search);
  const [error, setError] = useState(params.get('error'));
  const errorDescription = params.get('error_description');
  const success = params.get('success');
  const enrollResponse: PluginEnrollResponse = {
    error,
    errorDescription,
    clearError: () => setError(null),
    success,
  };

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
    selectedType,
    enrollResponse,
    hasPluginAccess,
    hasIntegrationAccess,
  };
}

export type State = ReturnType<typeof useIntegrationEnroll>;
export type PluginEnrollResponse = {
  error: string | null;
  errorDescription: string | null;
  clearError(): void;
  success: string | null;
};
