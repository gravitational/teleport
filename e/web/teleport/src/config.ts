import { generatePath } from 'react-router';
import teleCfg from 'teleport/config';
import { Resource } from 'e-shared/services/resources';

const cfg = {
  routes: {
    clusterAuthConnectors: '/web/cluster/:clusterId/auth',
    clusterRoles: '/web/cluster/:clusterId/roles',
    clusterTrustedClusters: '/web/cluster/:clusterId/trusted',
  },

  api: {
    licenseStatusPath: '/v1/enterprise/license/status',
    resourcePath: '/v1/enterprise/sites/:clusterId/resources/:kind?',
    removeResourcePath: '/v1/enterprise/sites/:clusterId/resources/:kind/:id',
  },

  isLeafCluster() {
    return teleCfg.proxyCluster !== teleCfg.clusterName;
  },

  getResourcesUrl(kind?: Resource['kind']) {
    const clusterId = teleCfg.clusterName;
    return generatePath(cfg.api.resourcePath, { clusterId, kind });
  },

  getRemoveResourceUrl(kind: Resource['kind'], id: string) {
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

  init(json: object) {
    teleCfg.init({ isEnterprise: true, routes: cfg.routes, ...json });
  },
};

export default cfg;
