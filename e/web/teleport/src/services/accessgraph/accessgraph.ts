import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import { QueryGraphResponse } from './types';

export const accessGraphService = {
  queryAccessGraph(query: string): Promise<QueryGraphResponse> {
    return api.get(
      `${cfg.api.accessGraphQueryPath}?query=${encodeURIComponent(query)}`
    );
  },
};
