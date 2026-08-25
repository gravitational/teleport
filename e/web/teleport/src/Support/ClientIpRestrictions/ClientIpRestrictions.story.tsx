import { StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  ToastNotificationProvider,
  ToastNotifications,
} from 'shared/components/ToastNotification';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  ClientIpRestriction,
  SaveClientIpRestrictionRequest,
} from 'e-teleport/services/clientiprestrictions';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Access } from 'teleport/services/user';

import { ClientIpRestrictions as ClientIpRestrictionsComponent } from './ClientIpRestrictions';

const fullAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

type StoryParams = {
  resource?: Partial<ClientIpRestriction>;
  access?: Access;
  hangFetch?: boolean;
  failFetch?: boolean;
  failSave?: boolean;
};

const baseResource: ClientIpRestriction = {
  cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
  mode: 'draft',
  status: 'draft',
  revision: 'rev-1',
};

export default {
  title: 'TeleportE/ClientIpRestrictions',
  component: ClientIpRestrictionsComponent,
  decorators: [
    (Story, storyCtx) => {
      const p: StoryParams = storyCtx.parameters ?? {};
      const ctx = createTeleportContextE();

      const [queryClient] = useState(
        () =>
          new QueryClient({
            defaultOptions: {
              queries: { refetchOnWindowFocus: false, retry: false },
            },
          })
      );

      const access = p.access ?? fullAccess;
      ctx.storeUser.geClientIpRestrictionAccess = () => access;

      let current: ClientIpRestriction = { ...baseResource, ...p.resource };
      let settleToActive = false;

      ctx.clientIpRestrictionsService = {
        ...ctx.clientIpRestrictionsService,
        async fetchClientIpRestriction(): Promise<ClientIpRestriction> {
          if (p.hangFetch) {
            return new Promise<ClientIpRestriction>(() => {});
          }
          if (p.failFetch) {
            throw new Error('Failed to load allowlist');
          }
          await new Promise(r => setTimeout(r, 150));
          // Simulate the controller advancing pending -> active between polls.
          if (settleToActive && current.status === 'pending') {
            current = { ...current, status: 'active' };
            settleToActive = false;
          }
          return { ...current };
        },
        async saveClientIpRestriction(
          _clusterId: string,
          req: SaveClientIpRestrictionRequest
        ): Promise<ClientIpRestriction> {
          if (p.failSave) {
            throw new Error('Failed to save allowlist');
          }
          await new Promise(r => setTimeout(r, 150));
          const status = req.mode === 'draft' ? 'draft' : 'pending';
          current = {
            cidrs: req.cidrs,
            mode: req.mode ?? '',
            expires: req.expires,
            status,
            revision: `rev-${Math.floor(Math.random() * 1e6)}`,
          };
          settleToActive = true;
          return { ...current };
        },
      };

      return (
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ContextProvider ctx={ctx}>
              <ToastNotificationProvider>
                <InfoGuidePanelProvider>
                  <ContentMinWidth>
                    <CollapsibleInfoSectionComponent
                      openLabel="Devs Instructions"
                      mb="3"
                    >
                      <Info kind="info">
                        Stories seed different server states. Actions call a
                        mock service that mimics the real write/derive behavior,
                        so you can drive the panel through its transitions.
                      </Info>
                    </CollapsibleInfoSectionComponent>
                    <Story />
                  </ContentMinWidth>
                </InfoGuidePanelProvider>
                <ToastNotifications />
              </ToastNotificationProvider>
            </ContextProvider>
          </MemoryRouter>
        </QueryClientProvider>
      );
    },
  ],
};

const story = (parameters: StoryParams): StoryObj => ({
  parameters,
  render: () => <ClientIpRestrictionsComponent clusterId="cluster-123" />,
});

const inMin = (minutes: number) =>
  new Date(Date.now() + minutes * 60 * 1000).toISOString();

export const Draft = story({ resource: { status: 'draft', mode: 'draft' } });

export const Pending = story({
  resource: { status: 'pending', mode: 'enforced' },
});

export const Active = story({
  resource: { status: 'active', mode: 'enforced' },
});

export const TestRunApplying = story({
  resource: { status: 'pending', mode: 'enforced', expires: inMin(30) },
});

export const TestRunActive = story({
  resource: { status: 'active', mode: 'enforced', expires: inMin(30) },
});

// The deadline passed, but the rules stay programmed until Cloud removes them.
export const TestRunEnding = story({
  resource: { status: 'active', mode: 'enforced', expires: inMin(-1) },
});

export const Expired = story({
  resource: { status: 'expired', mode: 'enforced', expires: inMin(-60) },
});

// A cancel or deactivate whose teardown has not been reported yet.
export const ReturningToDraft = story({
  resource: { status: 'pending', mode: 'draft' },
});

// A tenant before anything is configured, nil revision and all.
export const NotConfigured = story({
  resource: {
    status: '',
    mode: '',
    cidrs: [],
    revision: '00000000-0000-0000-0000-000000000000',
  },
});

export const Unknown = story({
  resource: { status: 'unknown', mode: 'enforced', cidrs: ['10.0.0.0/8'] },
});

export const Loading = story({ hangFetch: true });

export const FetchError = story({ failFetch: true });

export const SaveError = story({
  resource: { status: 'draft', mode: 'draft' },
  failSave: true,
});

export const ReadOnly = story({
  resource: { status: 'active', mode: 'enforced' },
  access: { ...fullAccess, edit: false, create: false },
});
