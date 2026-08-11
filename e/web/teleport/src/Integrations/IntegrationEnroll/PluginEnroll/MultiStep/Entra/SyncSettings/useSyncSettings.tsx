import { useState } from 'react';

import type {
  Plugin,
  PluginEntraIdSpec,
  PluginEntraIdSyncIntervals,
} from 'teleport/services/integrations';

import { AccessListOwnersSource, Filters } from '../types';
import { emptyFilter } from './constants';
import type { UserOption } from './types';

export function useSyncSettings(plugin?: Plugin<PluginEntraIdSpec>) {
  function toUserOption(owners: string[]): UserOption[] {
    if (!owners) {
      return [];
    }
    return owners.map(u => ({ value: u, label: u }));
  }

  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>(
    toUserOption(plugin?.spec.defaultOwners)
  );

  const [filters, setFilters] = useState<Filters>(
    plugin?.spec?.groupFilters ? plugin?.spec?.groupFilters : emptyFilter
  );

  const [accessListOwnersSource, setAccessListOwnersSource] = useState(
    plugin?.spec?.accessListOwnersSource ?? AccessListOwnersSource.Plugin
  );

  const [importAll, setImportAll] = useState(
    hasZeroFilters(plugin?.spec?.groupFilters)
  );

  const [syncIntervals, setSyncIntervals] =
    useState<PluginEntraIdSyncIntervals>(
      makeSyncInterval(plugin?.spec.syncIntervals)
    );

  return {
    importAll,
    setImportAll,
    selectedOwners,
    setSelectedOwners,
    filters,
    setFilters,
    accessListOwnersSource,
    setAccessListOwnersSource,
    syncIntervals,
    setSyncIntervals,
  };
}

export function hasZeroFilters(filters: Filters): boolean {
  if (!filters) {
    return true;
  }
  const hasFilters =
    filters.id?.length > 0 ||
    filters.nameRegex?.length > 0 ||
    filters.excludeId?.length > 0 ||
    filters.excludeNameRegex?.length > 0;

  return !hasFilters;
}

export function makeSyncInterval(
  intervals?: Partial<PluginEntraIdSyncIntervals>
): PluginEntraIdSyncIntervals {
  return {
    full: intervals?.full || '0s',
    delta: intervals?.delta || '0s',
  };
}
