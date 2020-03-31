import api from 'teleport/services/api';
import cfg from './../../config';
import { LicenseStatus } from './types';

const service = {
  fetchStatus() {
    return api
      .get(cfg.api.licenseStatusPath)
      .then(json => json as LicenseStatus);
  },
};

export default service;
