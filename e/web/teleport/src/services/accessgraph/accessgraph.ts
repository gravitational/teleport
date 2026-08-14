import { AccessPathDiff } from 'e-teleport/AccessGraph/Diff';
import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import { Role } from 'teleport/services/resources';

import { QueryGraphResponse } from './types';

export type AccessGraphSettings = {
  // enable_demo_mode is true if demo mode has been enabled for the cluster.
  enable_demo_mode: boolean;
  status: {
    // initial_sync_complete is true if the cluster has synced its resources with TAG at least once.
    initial_sync_complete: boolean;
    http_ready: boolean;
  };
};

export const accessGraphService = {
  getAccessGraphSettings(
    abortSignal?: AbortSignal
  ): Promise<AccessGraphSettings> {
    return api.get(cfg.api.accessGraphSettingsPath, abortSignal);
  },
  enableDemoMode(abortSignal?: AbortSignal): Promise<boolean> {
    return api.post(
      cfg.api.accessGraphSettingsPath,
      { enable_demo_mode: true },
      abortSignal
    );
  },
  getRoleDiff(role: Role, abortSignal?: AbortSignal): Promise<AccessPathDiff> {
    return api.post(cfg.api.accessGraphRoleTesterPath, role, abortSignal);
  },
  queryAccessGraph(query: string): Promise<QueryGraphResponse> {
    return api.get(
      `${cfg.api.accessGraphQueryPath}?query=${encodeURIComponent(query)}`
    );
  },
};
