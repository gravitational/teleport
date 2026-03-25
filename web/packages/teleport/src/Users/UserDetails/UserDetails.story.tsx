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

import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { User } from 'teleport/services/user';

import {
  UserDetails,
  UserDetailsAuthType,
  UserDetailsTitle,
} from './UserDetails';

export type UserDetailsStoryProps = {
  userType: UserDetailsAuthType;
  isBot: boolean;
  userName: string;
  rolesCount: number;
  traitsCount: number;
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

const meta: Meta<UserDetailsStoryProps> = {
  title: 'Teleport/Users/UserDetails',
  component: Story,
  excludeStories: ['createMockUser'],
  beforeEach: () => {
    queryClient.clear();
  },
  decorators: [
    Story => {
      const ctx = createTeleportContext();
      cfg.proxyCluster = 'localhost';

      return (
        <QueryClientProvider client={queryClient}>
          <ContextProvider ctx={ctx}>
            <Story />
          </ContextProvider>
        </QueryClientProvider>
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
    userType: 'local' as const,
    isBot: false,
    userName: 'john.the.user',
    rolesCount: 16,
    traitsCount: 5,
  },
};

export default meta;

type Story = StoryObj<UserDetailsStoryProps>;

function generateRoles(count: number) {
  if (!count || count < 0) return [];

  const adjectives = [
    'readonly',
    'enterprise',
    'dev',
    'jit',
    'admin',
    'system',
    'remote',
    'staging',
    'prod',
    'temp',
  ];
  const roleNouns = [
    'auditor',
    'editor',
    'operator',
    'viewer',
    'manager',
    'analyst',
    'developer',
    'security',
    'support',
    'backup',
  ];
  const baseRoles = ['access', 'auditor', 'editor'];

  const generateRole = (index: number) => {
    const adjIndex = index % adjectives.length;
    const nounIndex = index % roleNouns.length;
    return `${adjectives[adjIndex]}-${roleNouns[nounIndex]}`;
  };

  return [
    ...baseRoles.slice(0, Math.min(count, baseRoles.length)),
    ...Array.from({ length: Math.max(0, count - baseRoles.length) }, (_, i) =>
      generateRole(i)
    ),
  ];
}

function generateTraits(count: number) {
  if (!count || count < 0) return undefined;

  const traitKeys = [
    'logins',
    'databaseUsers',
    'databaseNames',
    'kubeUsers',
    'kubeGroups',
    'windowsLogins',
    'awsRoleArns',
  ];

  const traits: Record<string, string[]> = {};
  const selectedKeys = traitKeys.slice(0, Math.min(count, traitKeys.length));

  selectedKeys.forEach(key => {
    const valueCount = 1 + Math.floor(Math.random() * 3);
    traits[key] = Array.from(
      { length: valueCount },
      (_, i) => `${key}-${i + 1}`
    );
  });

  return traits;
}

function getUserConfig(userType: UserDetailsAuthType | 'local') {
  switch (userType) {
    case 'local':
      return {
        authType: 'local',
        origin: undefined,
        isLocal: true,
      };
    case 'github':
      return {
        authType: 'github',
        origin: undefined,
        isLocal: false,
      };
    case 'saml':
      return {
        authType: 'saml',
        origin: undefined,
        isLocal: false,
      };
    case 'oidc':
      return {
        authType: 'oidc',
        origin: undefined,
        isLocal: false,
      };
    case 'okta':
      return {
        authType: 'saml',
        origin: 'okta' as const,
        isLocal: false,
      };
    case 'scim':
      return {
        authType: 'saml',
        origin: 'scim' as const,
        isLocal: false,
      };
    default:
      return {
        authType: 'local',
        origin: undefined,
        isLocal: true,
      };
  }
}

export function createMockUser(props: UserDetailsStoryProps): User {
  const config = getUserConfig(props.userType);

  return {
    name: props.userName,
    authType: config.authType,
    origin: config.origin,
    isBot: props.isBot,
    isLocal: config.isLocal,
    roles: generateRoles(props.rolesCount),
    allTraits: generateTraits(props.traitsCount),
  };
}

function Story(props: UserDetailsStoryProps) {
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

export const BasicUser: Story = {
  parameters: {
    msw: {
      handlers: [
        http.get('/v2/webapi/sites/:clusterId/locks', () => {
          return HttpResponse.json({
            items: [],
          });
        }),
      ],
    },
  },
};
