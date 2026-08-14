import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { Route, Switch } from 'teleport/components/Router';

import { AccessLists } from './AccessLists';
import { ViewEditAccessList } from './ViewEditAccessList';

const accessListDetailsRoute = cfg.routes.accessLists.replace(
  '/:accessListId?',
  '/:accessListId'
);

export function AccessListManagement() {
  return (
    <AccessListManagementContextProvider>
      <Switch>
        <Route
          key="access-lists"
          exact
          path={cfg.getAccessListManagementRoute()}
          element={<AccessLists />}
        />
        <Route
          key="view-access-list"
          exact
          path={accessListDetailsRoute}
          element={<ViewEditAccessList />}
        />
      </Switch>
    </AccessListManagementContextProvider>
  );
}
