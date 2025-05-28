import type { UseQueryOptions, UseQueryResult } from '@tanstack/react-query';

import { pluginsService } from 'e-teleport/services/plugins/plugins';
import {
  Plugin,
  PluginNameToDetails,
  PluginNameToSpec,
} from 'teleport/services/integrations';
import { createQueryHook } from 'teleport/services/queryHelpers';

export const {
  useQuery: useCheckPluginRequiresCleanup,
  createQuery: createCheckPluginRequiresCleanupQuery,
} = createQueryHook(
  ['plugin', 'requiresCleanup'],
  pluginsService.checkPluginRequiresCleanup
);

const { useQuery: _useFetchPlugin, createQueryKey: createFetchPluginQueryKey } =
  createQueryHook(['plugin', 'fetch'], pluginsService.fetchPlugin);

export { createFetchPluginQueryKey };

export function useFetchPlugin<T extends string>(
  name: T,
  options?: Omit<
    UseQueryOptions<Plugin<PluginNameToSpec[T], PluginNameToDetails[T]>>,
    'queryKey' | 'queryFn'
  >
) {
  return _useFetchPlugin(name, options) as UseQueryResult<
    Plugin<PluginNameToSpec[T], PluginNameToDetails[T]>
  >;
}
