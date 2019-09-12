import { generatePath } from 'react-router';
import teleCfg from 'teleport/config';

const cfg = {
  routes: {
    clusterAuthConnectors: '/web/cluster/:clusterId/auth',
    clusterRoles: '/web/cluster/:clusterId/roles',
    clusterTrustedClusters: '/web/cluster/:clusterId/trusted',
  },

  api: {
    resourcePath: '/v1/enterprise/resources/:kind?',
    removeResourcePath: '/v1/enterprise/resources/:kind/:id',
  },

  getResourcesUrl(kind) {
    return generatePath(cfg.api.resourcePath, { kind });
  },

  getRemoveResourceUrl(kind, id) {
    return generatePath(cfg.api.removeResourcePath, { kind, id });
  },

  getAuthConnectorsRoute() {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.routes.clusterAuthConnectors, { clusterId });
  },

  getRolesRoute() {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.routes.clusterRoles, { clusterId });
  },

  getTrustedClustersRoute() {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.routes.clusterTrustedClusters, { clusterId });
  },

  init(json) {
    teleCfg.init(json);
  },
};

export default cfg;
