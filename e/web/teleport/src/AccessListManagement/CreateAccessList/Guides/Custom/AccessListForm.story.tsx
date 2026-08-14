import { StoryObj } from '@storybook/react-vite';
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { CreateAccessListContextProvider } from '../../CreateAccessListContextProvider';
import { AccessListForm } from './AccessListForm';

const rootScopedRolesPath = cfg.getRootScopedRolesUrl({}).split('?')[0];

const getRootScopedRolesHandler = http.get(rootScopedRolesPath, () => {
  return HttpResponse.json({
    roles: [
      {
        name: 'team-admin',
        scope: '/',
        assignableScopes: ['/dev', '/staging/**'],
      },
      {
        name: 'audit',
        scope: '/',
        assignableScopes: ['/**'],
      },
    ],
    startKey: '',
  });
});

const getEmptyRootScopedRolesHandler = http.get(rootScopedRolesPath, () => {
  return HttpResponse.json({
    roles: [],
    startKey: '',
  });
});

export default {
  title: 'TeleportE/AccessLists/Guide/Custom',
};

export const Default: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({ accessLists: [] });
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return HttpResponse.json({});
      }),
      getRootScopedRolesHandler
    );
  },

  render() {
    return (
      <Provider>
        <AccessListForm />
      </Provider>
    );
  },
};

export const DefaultWithoutScopedRoles: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({ accessLists: [] });
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return HttpResponse.json({});
      }),
      getEmptyRootScopedRolesHandler
    );
  },

  render() {
    return (
      <Provider>
        <AccessListForm />
      </Provider>
    );
  },
};

export const Processing: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({ accessLists: [] });
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return delay('infinite');
      }),
      http.get(cfg.oss.api.usersPath, () => {
        return HttpResponse.json([{ name: 'alice' }]);
      }),
      getRootScopedRolesHandler
    );
  },

  render() {
    return (
      <Provider>
        <Info>Dev: click next to see loading state</Info>
        <AccessListForm />
      </Provider>
    );
  },
};

export const Failed: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.getAccessManagementListUrlV2({}), () => {
        return HttpResponse.json({ accessLists: [] });
      }),
      http.get(cfg.oss.api.usersPath, () => {
        return HttpResponse.json([{ name: 'alice' }]);
      }),
      http.post(cfg.getAccessManagementListUrl(), () => {
        return HttpResponse.json(
          {
            error: { message: 'Whoops, creating list error' },
          },
          { status: 500 }
        );
      }),
      getRootScopedRolesHandler
    );
  },

  render() {
    return (
      <Provider>
        <Info>Dev: click next to see failed state</Info>
        <AccessListForm />
      </Provider>
    );
  },
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <CreateAccessListContextProvider>
              {props.children}
            </CreateAccessListContextProvider>
          </AccessListManagementContextProvider>
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
};
