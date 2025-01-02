import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { Route, Switch } from 'teleport/components/Router';

import { AccessLists } from './AccessLists';
import { ViewEditAccessList } from './ViewEditAccessList';

export function AccessListManagement() {
  return (
    <AccessListManagementContextProvider>
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
    </AccessListManagementContextProvider>
  );
}
