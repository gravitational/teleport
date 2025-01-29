import { AccessPathDiff } from 'e-teleport/AccessGraph/Diff';
import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import { Role } from 'teleport/services/resources';

import { QueryGraphResponse } from './types';

export const accessGraphService = {
  getRoleDiff(role: Role): Promise<AccessPathDiff> {
    return api.post(cfg.api.accessGraphRoleTesterPath, role);
  },
  queryAccessGraph(query: string): Promise<QueryGraphResponse> {
    return api.get(
      `${cfg.api.accessGraphQueryPath}?query=${encodeURIComponent(query)}`
    );
  },
};
