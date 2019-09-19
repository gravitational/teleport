import { generatePath } from 'react-router';
import teleCfg from 'teleport/config';

const cfg = {
  routes: {
    clusterAuthConnectors: '/web/cluster/:clusterId/auth',
    clusterRoles: '/web/cluster/:clusterId/roles',
    clusterTrustedClusters: '/web/cluster/:clusterId/trusted',
  },

  api: {
    resourcePath: '/v1/enterprise/sites/:clusterId/resources/:kind?',
    removeResourcePath: '/v1/enterprise/sites/:clusterId/resources/:kind/:id',
  },

  getResourcesUrl(kind) {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.api.resourcePath, { clusterId, kind });
  },

  getRemoveResourceUrl(kind, id) {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.api.removeResourcePath, { clusterId, kind, id });
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
