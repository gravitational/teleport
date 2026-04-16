import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';

import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../../Shared/Audit';
import { CreateAccessListContextProvider } from '../CreateAccessListContextProvider';
import { Spec } from '../types';
import { DeploymentMethods } from './DeploymentMethods';
import { ErrorCreatingAccessListDialog } from './ErrorCreatingAccessListDialog';

export default {
  title: 'TeleportE/AccessLists/Terraform/DeploymentMethods',
};

const listHandler = http.get(cfg.getAccessManagementListUrlV2({}), () =>
  HttpResponse.json({ accessLists: [] })
);

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
        <Info>
          Dev: click "Create Access List Now" to see the finished step
        </Info>
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

  return (
    <MemoryRouter
      initialEntries={[{ pathname: '/', state: { spec, preset: '' } }]}
    >
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <CreateAccessListContextProvider>
            {children}
          </CreateAccessListContextProvider>
        </AccessListManagementContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
