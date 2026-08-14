import { PropsWithChildren, useEffect } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { CreateAccessListContextProvider } from 'e-teleport/AccessListManagement/CreateAccessList/CreateAccessListContextProvider';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { UserContextProvider } from 'teleport/User';

import { sampleSelectedAccounts, sampleSelectedPermissionSet } from './mocks';

export const ComponentWithAccountsSelected: React.FC<
  PropsWithChildren<{
    wantWildcard?: boolean;
  }>
> = ({ wantWildcard, children }) => {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;
  useEffect(() => {
    if (wantWildcard) {
      awsIcRoleState.addAccountWildcard(sampleSelectedPermissionSet);
    } else {
      awsIcRoleState.updateAccount(
        sampleSelectedAccounts,
        sampleSelectedPermissionSet
      );
    }
  }, []);
  return <>{children}</>;
};

export const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  const router = createMemoryRouter([
    {
      path: '*',
      element: (
        <InfoGuidePanelProvider>
          <UserContextProvider>
            <ContextProvider ctx={ctx}>
              <AccessListManagementContextProvider>
                <CreateAccessListContextProvider>
                  {props.children}
                </CreateAccessListContextProvider>
              </AccessListManagementContextProvider>
            </ContextProvider>
          </UserContextProvider>
        </InfoGuidePanelProvider>
      ),
    },
  ]);

  return <RouterProvider router={router} />;
};
