import { ClusterListPage } from 'teleport/Clusters/Clusters';
import { Route, Switch } from 'teleport/components/Router';

import cfg from 'e-teleport/config';

import { ManageCluster } from './ManageCluster';

export function Clusters() {
  return (
    <Switch>
      <Route
        key="cluster-list"
        exact
        path={cfg.oss.routes.clusters}
        component={ClusterListPage}
      />
      <Route
        key="cluster-management"
        path={cfg.oss.routes.manageCluster}
        component={ManageCluster}
      />
    </Switch>
  );
}
