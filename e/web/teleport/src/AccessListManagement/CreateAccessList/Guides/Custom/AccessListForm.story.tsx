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

export default {
  title: 'TeleportE/AccessLists/Guide/Custom',
};

export const Default: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
        http.post(cfg.getAccessManagementListUrl(), () => {
          return HttpResponse.json({});
        }),
      ],
    },
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
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getAccessManagementListUrlV2({}), () => {
          return HttpResponse.json({ accessLists: [] });
        }),
        http.post(cfg.getAccessManagementListUrl(), () => {
          return delay('infinite');
        }),
        http.get(cfg.oss.api.usersPath, () => {
          return HttpResponse.json([{ name: 'alice' }]);
        }),
      ],
    },
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
  parameters: {
    msw: {
      handlers: [
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
      ],
    },
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
