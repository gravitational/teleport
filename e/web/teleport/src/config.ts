import { generatePath } from 'react-router';
import teleCfg from 'teleport/config';
import { AccessRequestFilter } from 'e-teleport/services/workflow';

const cfg = {
  routes: {
    requests: '/web/requests',
    requestNew: '/web/requests/new',
  },

  api: {
    accessRequestPath: '/v1/enterprise/accessrequest/:requestId?',
    accessRequestFilterPath: '/v1/enterprise/accessrequest?user=:user?',
  },

  getAccessRequestUrl(requestId?: string) {
    return generatePath(cfg.api.accessRequestPath, { requestId });
  },

  getAccessRequestFilterUrl(filter: AccessRequestFilter) {
    return generatePath(cfg.api.accessRequestFilterPath, { ...filter });
  },

  init(json: object) {
    teleCfg.init({ isEnterprise: true, routes: cfg.routes, ...json });
  },
};

export default cfg;
