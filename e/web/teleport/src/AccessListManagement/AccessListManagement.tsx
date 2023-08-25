import React, { lazy } from 'react';
import { Switch, Route } from 'teleport/components/Router';

import cfg from 'e-teleport/config';

const CreateAccessList = lazy(() => import('./CreateAccessList'));
const AccessLists = lazy(() => import('./AccessLists'));

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
    </Switch>
  );
}
