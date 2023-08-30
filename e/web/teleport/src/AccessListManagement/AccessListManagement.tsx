import React, { lazy } from 'react';
import { Switch, Route } from 'teleport/components/Router';

import cfg from 'e-teleport/config';

const CreateAccessList = lazy(() => import('./CreateAccessList'));
const AccessLists = lazy(() => import('./AccessLists'));
const ViewEditAccessList = lazy(() => import('./ViewEditAccessList'));

export function AccessListManagement() {
  return (
    <Switch>
      <Route
        key="access-list-create"
        exact
        path={cfg.routes.accessListNew}
        component={CreateAccessList}
      />
      <Route
        key="access-lists"
        exact
        path={cfg.getAccessListManagementRoute()}
        component={AccessLists}
      />
      <Route
        key="view-access-list"
        exact
        path={cfg.routes.accessLists}
        component={ViewEditAccessList}
      />
    </Switch>
  );
}
