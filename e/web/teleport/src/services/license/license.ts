import api from 'teleport/services/api';

import { LicenseStatus } from './types';

const service = {
  fetchStatus() {
    return api
      .get('/v1/enterprise/license/status')
      .then(json => json as LicenseStatus);
  },
};

export default service;
