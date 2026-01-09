import { http, HttpHandler, HttpResponse } from 'msw';
import { PropsWithChildren, useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import { ContextProvider } from 'teleport/index';

import { CreateAccessListContextProvider } from '../../CreateAccessListContextProvider';

export const ComponentWithPreset: React.FC<
  PropsWithChildren<{
    preset: AccessListPreset;
  }>
> = ({ preset, children }) => {
  const { guideEditor } = useAccessListManagementContext();
  useEffect(() => {
    guideEditor.setPreset(preset);
  }, []);
  return <>{children}</>;
};

export const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <CreateAccessListContextProvider>
            {props.children}
          </CreateAccessListContextProvider>
        </AccessListManagementContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export const sharedHandlers: HttpHandler[] = [
  http.get(cfg.oss.api.usersPath, () => {
    return HttpResponse.json([{ name: 'alice' }]);
  }),
  http.get(cfg.getAccessManagementListUrlV2({}), () => {
    return HttpResponse.json({
      accessLists: [
        {
          metadata: { name: 'aaa' },
          spec: {
            title: 'Interns',
            description: 'lorem ipsum description',
            audit: { frequency: '', next_audit_date: new Date() },
            grants: { roles: ['access', 'editor'] },
            ownership_requires: { roles: [] },
            owners: [],
          },
          membersCount: 0,
        },
      ],
    });
  }),
];
