import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import { makeLicense } from './make';
import type { License } from './types';

export const downloadsService = {
  fetchLicense(): Promise<License> {
    return api.get(cfg.api.license).then(makeLicense);
  },
};
