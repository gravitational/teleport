import { MemoryRouter, Route, Switch } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';
import { ContextProvider } from 'teleport/index';
import { UserContextProvider } from 'teleport/User';

export const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter initialEntries={props.initialEntries ?? []}>
      <InfoGuidePanelProvider>
        <UserContextProvider>
          <AccessGraphDemoProvider>
            <ContextProvider ctx={ctx}>
              <Switch>
                <AccessListManagementContextProvider>
                  <Route
                    path={cfg.routes.accessLists}
                    render={() => <>{props.children}</>}
                  />
                </AccessListManagementContextProvider>
              </Switch>
            </ContextProvider>
          </AccessGraphDemoProvider>
        </UserContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
};
