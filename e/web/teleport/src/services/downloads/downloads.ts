import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import { makeLicense, makeReleases } from './make';
import type { License } from './types';

export const downloadsService = {
  fetchReleases() {
    return api.get(cfg.api.releases).then(makeReleases);
  },
  fetchLicense(): Promise<License> {
    return api.get(cfg.api.license).then(makeLicense);
  },
};
