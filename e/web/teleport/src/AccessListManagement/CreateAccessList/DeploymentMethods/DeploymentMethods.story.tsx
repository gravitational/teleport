import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { createMemoryRouter, RouterProvider } from 'react-router';

import { Info } from 'design/Alert';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { defaultStandardRoleConditions } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/conditions';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessListMemberKind,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';

import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../../Shared/Audit';
import { CreateAccessListContextProvider } from '../CreateAccessListContextProvider';
import { Members, Owners, Spec } from '../types';
import { DeploymentMethods } from './DeploymentMethods';
import { ErrorCreatingAccessListDialog } from './ErrorCreatingAccessListDialog';

export default {
  title: 'TeleportE/AccessLists/Terraform/DeploymentMethods',
};

const listHandler = http.get(cfg.getAccessManagementListUrlV2({}), () =>
  HttpResponse.json({ accessLists: [] })
);

const owners: Owners = {
  selectedRolesRequired: [{ label: 'access', value: 'access' }],
  eligibleOwners: [],
  selectedOwners: [
    {
      label: 'alice',
      value: { name: 'alice', membershipKind: AccessListMemberKind.User },
    },
    {
      label: 'bob',
      value: { name: 'bob', membershipKind: AccessListMemberKind.User },
    },
  ],
  traitLabels: [{ name: 'team', value: 'engineering' }],
  traitLookup: {},
};

const members: Members = {
  selectedRolesRequired: [{ label: 'requester', value: 'requester' }],
  eligibleMembers: [],
  selectedMembers: [
    {
      label: 'carol',
      value: { name: 'carol', membershipKind: AccessListMemberKind.User },
    },
    {
      label: 'dave',
      value: { name: 'dave', membershipKind: AccessListMemberKind.User },
    },
  ],
  traitLabels: [{ name: 'location', value: 'us-west' }],
  traitLookup: {},
};

const standardRoleConditions = {
  ...defaultStandardRoleConditions(),
  app_labels: { env: ['prod'] },
  db_labels: { env: ['staging'] },
};

export const Default: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        listHandler,
        http.post(cfg.getAccessManagementListUrl(), () =>
          HttpResponse.json({})
        ),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <Info>Dev: click buttons to see the next step</Info>
        <DeploymentMethods />
      </Provider>
    );
  },
};

export const CreateFailed: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        listHandler,
        http.post(cfg.getAccessManagementListUrl(), () =>
          HttpResponse.json(
            { error: { message: 'Failed to create access list' } },
            { status: 500 }
          )
        ),
      ],
    },
  },
  render() {
    return (
      <Provider>
        <Info>Dev: click "Create Access List Now" to see the error dialog</Info>
        <DeploymentMethods />
      </Provider>
    );
  },
};

export function ErrorDialog() {
  return (
    <ErrorCreatingAccessListDialog
      error="Something went wrong while creating the Access List"
      onCancel={() => {}}
      retry={() => {}}
    />
  );
}

function Provider({ children }: { children: React.ReactNode }) {
  const spec: Spec = {
    title: 'Engineering Access',
    description: 'Access for the engineering team',
    reviewDayOfMonth: reviewDayOfMonthOpts.find(
      o => o.value === ReviewDayOfMonth.FirstDayOfMonth
    ),
    reviewFrequency: reviewFrequencyOpts.find(
      o => o.value === ReviewFrequency.SixMonths
    ),
    auditStartDate: new Date('2026-06-01'),
  };

  const ctx = createTeleportContextE();

  const router = createMemoryRouter(
    [
      {
        path: '*',
        element: (
          <ContextProvider ctx={ctx}>
            <AccessListManagementContextProvider>
              <CreateAccessListContextProvider>
                {children}
              </CreateAccessListContextProvider>
            </AccessListManagementContextProvider>
          </ContextProvider>
        ),
      },
    ],
    {
      initialEntries: [
        {
          pathname: '/',
          state: { spec, owners, members, standardRoleConditions, preset: '' },
        },
      ],
    }
  );

  return <RouterProvider router={router} />;
}
