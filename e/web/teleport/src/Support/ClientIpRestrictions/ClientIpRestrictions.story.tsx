import { StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';

import { ClientIpRestrictions as ClientIpRestrictionsComponent } from './ClientIpRestrictions';

type Access = { list: boolean; edit: boolean; create: boolean };

export default {
  title: 'TeleportE/ClientIpRestrictions',
  component: ClientIpRestrictionsComponent,
  args: {
    clusterId: 'cluster-123',
    allowList: ['10.0.0.0/8', '192.168.0.0/16'],
    access: { list: true, edit: true, create: true },
    hangFetch: false,
    failFetch: false,
    failSave: false,
  },
  argTypes: {
    allowList: { control: 'object', description: 'Initial server state' },
    access: { control: 'object', description: 'Access flags' },
    hangFetch: { control: 'boolean', description: 'Simulate loading' },
    failFetch: { control: 'boolean', description: 'Simulate fetch error' },
    failSave: { control: 'boolean', description: 'Simulate save error' },
    clusterId: { control: 'text' },
  },
  decorators: [
    (Story, storyCtx) => {
      const ctx = createTeleportContextE() as any;

      const hangFetch: boolean = storyCtx.parameters.hangFetch ?? false;

      const initialAllowList: string[] = storyCtx.parameters.allowList ?? [
        '10.0.0.0/8',
        '192.168.0.0/16',
      ];

      const access: Access = storyCtx.parameters.access ?? {
        list: true,
        edit: true,
        create: true,
      };

      const failFetch: boolean = storyCtx.parameters.failFetch ?? false;
      const failSave: boolean = storyCtx.parameters.failSave ?? false;

      let ret = [...initialAllowList];

      ctx.storeUser.geClientIpRestrictionAccess = () => access;

      ctx.clientIpRestrictionsService = {
        async fetchClientIpRestrictions(): Promise<string[]> {
          if (hangFetch) {
            return new Promise<string[]>(() => {});
          }

          if (failFetch) {
            const err: any = new Error('Failed to load allowlist');
            err.statusText = 'Failed to load allowlist';
            throw err;
          }
          await new Promise(r => setTimeout(r, 150));
          return [...ret];
        },
        async saveClientIpRestrictions(
          _clusterId: string,
          list: string[]
        ): Promise<void> {
          if (failSave) {
            const err: any = new Error('Failed to save allowlist');
            err.statusText = 'Failed to save allowlist';
            throw err;
          }
          await new Promise(r => setTimeout(r, 150));
          ret = [...list];
        },
      };

      return (
        <MemoryRouter>
          <ContextProvider ctx={ctx}>
            <InfoGuidePanelProvider>
              <ContentMinWidth>
                <CollapsibleInfoSectionComponent
                  openLabel="Devs Instructions"
                  mb="3"
                >
                  <Info kind="info">
                    You can toggle the Edit/Save button and try saving changes.
                    Different stories change permissions and simulate fetch/save
                    failures without MSW.
                  </Info>
                </CollapsibleInfoSectionComponent>
                <Story />
              </ContentMinWidth>
            </InfoGuidePanelProvider>
          </ContextProvider>
        </MemoryRouter>
      );
    },
  ],
};

export const LoadingList = {
  parameters: {
    hangFetch: true,
    access: { list: true, edit: true, create: true },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;

export const EditableWithData = {
  parameters: {
    allowList: ['216.239.32.0/19', '8.8.8.0/24', '8.8.4.0/24'],
    access: { list: true, edit: true, create: true },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;

export const EmptyList = {
  parameters: {
    allowList: [],
    access: { list: true, edit: true, create: true },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;

export const ReadOnly = {
  parameters: {
    allowList: ['203.0.113.0/24'],
    access: { list: true, edit: false, create: false },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;

export const ListError = {
  parameters: {
    failFetch: true,
    access: { list: true, edit: true, create: true },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;

export const SaveError = {
  parameters: {
    allowList: ['10.10.0.0/16'],
    failSave: true,
    access: { list: true, edit: true, create: true },
  },
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
} satisfies StoryObj<typeof ClientIpRestrictionsComponent>;
