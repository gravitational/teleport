import cfg from 'gravity/config';
import { generatePath } from "react-router";

cfg.init({

  isEnterprise: true,

  routes: {
    // cluster
    clusterRoles: '/web/site/:siteId/roles',
    clusterAuthConnectors: '/web/site/:siteId/auth',

    // default app entry point
    defaultEntry: '/web/portal',

    // hub
    hubBase: '/web/portal',
    hubClusters: '/web/portal/clusters',
    hubLicenses: '/web/portal/licenses',
    hubCatalog: '/web/portal/catalog',
    hubSettings: '/web/portal/settings',
    hubSettingCert: '/web/portal/settings/cert',
    hubSettingAccount: '/web/portal/settings/account',
    hubAccess: '/web/portal/access',
    hubAccessRoles: '/web/portal/access/roles',
    hubAccessUsers: '/web/portal/access/users',
    hubAccessAuth: '/web/portal/access/auth',
  },

  api: {
    licenseGeneratorPath: '/portalapi/v1/license',
  },

  getClusterRolesRoute(siteId){
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.clusterRoles, { siteId });
  },

  getClusterAuthConnectorsRoute(siteId){
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.clusterAuthConnectors, { siteId })
  },
});

export default cfg;