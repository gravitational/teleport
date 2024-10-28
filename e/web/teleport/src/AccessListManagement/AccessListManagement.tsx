import React from 'react';
import { Switch, Route } from 'teleport/components/Router';

import cfg from 'e-teleport/config';

import { AccessLists } from './AccessLists';
import { ViewEditAccessList } from './ViewEditAccessList';

export function AccessListManagement() {
  return (
    <Switch>
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
