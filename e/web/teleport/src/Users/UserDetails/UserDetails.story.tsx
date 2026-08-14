/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { UserDetailsTitle } from 'teleport/Users/UserDetails';
import {
  createMockUser,
  UserDetailsStoryProps,
} from 'teleport/Users/UserDetails/UserDetails.story';

import { UserDetails } from './UserDetails';

type StoryProps = UserDetailsStoryProps;

const queryClient = new QueryClient({});

const meta: Meta<StoryProps> = {
  title: 'TeleportE/Users/UserDetailsE',
  component: Story,
  beforeEach: () => {
    queryClient.clear();
  },
  decorators: [
    Story => {
      const ctx = createTeleportContextE() as any;

      return (
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ContextProvider ctx={ctx}>
              <Story />
            </ContextProvider>
          </QueryClientProvider>
        </MemoryRouter>
      );
    },
  ],
  argTypes: {
    userType: {
      control: { type: 'select' },
      options: ['local', 'github', 'saml', 'oidc', 'okta', 'scim'],
    },
    isBot: {
      control: { type: 'boolean' },
    },
    userName: {
      control: { type: 'text' },
    },
    rolesCount: {
      control: { type: 'select' },
      options: [0, 5, 16, 128],
    },
    traitsCount: {
      control: { type: 'select' },
      options: [0, 5, 16, 128],
    },
  },
  args: {
    userType: 'okta' as const,
    isBot: false,
    userName: 'enterprise.user',
    rolesCount: 7,
    traitsCount: 8,
  },
};

export default meta;

type Story = StoryObj<StoryProps>;

function generateAccessLists(count: number) {
  return Array.from({ length: count }, (_, i) => ({
    metadata: { name: `access-list-${i + 1}` },
    spec: {
      title: `Access List ${i + 1}`,
      description: `Description for access list ${i + 1}`,
    },
    current_user_assignments: {
      membership_type: i % 2 === 0 ? 1 : 0,
      ownership_type: i % 3 === 0 ? 1 : 0,
    },
  }));
}

function Story(props: StoryProps) {
  const user = createMockUser(props);

  return (
    <SlidingSidePanel
      panelWidth={480}
      isVisible={true}
      slideFrom="right"
      zIndex={1}
      skipAnimation={false}
    >
      <InfoGuideContainer
        onClose={() => null}
        title={<UserDetailsTitle user={user} />}
      >
        <UserDetails user={user} />
      </InfoGuideContainer>
    </SlidingSidePanel>
  );
}

export const NoAccessLists: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get('/v2/webapi/sites/:clusterId/locks', () => {
        return HttpResponse.json({
          items: [],
        });
      }),
      http.get('/v1/enterprise/users/:username/accesslists', () => {
        return HttpResponse.json({
          accessLists: [],
          nextPageToken: '',
          totalCount: 0,
        });
      })
    );
  },
};

export const SomeAccessLists: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get('/v2/webapi/sites/:clusterId/locks', () => {
        return HttpResponse.json({
          items: [],
        });
      }),
      http.get('/v1/enterprise/users/:username/accesslists', () => {
        return HttpResponse.json({
          accessLists: generateAccessLists(5),
          nextPageToken: '',
          totalCount: 5,
        });
      })
    );
  },
};

export const ManyAccessLists: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get('/v2/webapi/sites/:clusterId/locks', () => {
        return HttpResponse.json({
          items: [],
        });
      }),
      http.get('/v1/enterprise/users/:username/accesslists', () => {
        return HttpResponse.json({
          accessLists: generateAccessLists(100),
          nextPageToken: 'next-page-token',
          totalCount: 25000,
        });
      })
    );
  },
};

export const LoadingAccessLists: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get('/v2/webapi/sites/:clusterId/locks', () => {
        return HttpResponse.json({
          items: [],
        });
      }),
      http.get('/v1/enterprise/users/:username/accesslists', () => {
        return new Promise(() => {});
      })
    );
  },
};
