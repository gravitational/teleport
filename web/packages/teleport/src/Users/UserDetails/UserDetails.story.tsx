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

import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { User } from 'teleport/services/user';

import { UserAuthType } from './types';
import {
  UserDetails,
  UserDetailsTitle,
  UserRoles,
  UserTraits,
} from './UserDetails';

export type UserDetailsStoryProps = {
  userType: UserAuthType;
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

export const userDetailsArgTypes = {
  userType: {
    control: { type: 'select' },
    options: ['local', 'github', 'saml', 'oidc', 'okta', 'scim', 'bot'],
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
};

export const userDetailsDefaultArgs = {
  userType: 'local' as UserAuthType,
  userName: 'john.the.user',
  rolesCount: 16,
  traitsCount: 5,
};

const meta: Meta<UserDetailsStoryProps> = {
  title: 'Teleport/Users/UserDetails',
  component: Story,
  excludeStories: [
    'userDetailsArgTypes',
    'userDetailsDefaultArgs',
    'createMockUser',
  ],
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
  argTypes: userDetailsArgTypes,
  args: userDetailsDefaultArgs,
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

  const traitTypes = [
    {
      key: 'logins',
      values: ['john', 'root', 'ubuntu', 'admin', 'service', 'deploy'],
    },
    {
      key: 'groups',
      values: ['admin', 'developers', 'security', 'qa', 'devops', 'support'],
    },
    {
      key: 'department',
      values: ['engineering', 'security', 'sales', 'marketing', 'finance'],
    },
    {
      key: 'team',
      values: ['platform', 'infrastructure', 'frontend', 'backend', 'data'],
    },
    {
      key: 'environment',
      values: ['production', 'staging', 'development', 'testing'],
    },
    {
      key: 'cost_center',
      values: ['eng-001', 'sec-002', 'ops-003', 'data-004'],
    },
    {
      key: 'manager',
      values: ['john.doe', 'jane.smith', 'alice.wilson', 'bob.johnson'],
    },
    {
      key: 'location',
      values: ['us-east', 'us-west', 'eu-central', 'ap-southeast'],
    },
    {
      key: 'clearance',
      values: ['public', 'internal', 'confidential', 'restricted'],
    },
    { key: 'project', values: ['alpha', 'beta', 'gamma', 'delta', 'omega'] },
  ];

  if (count === 0) return undefined;

  const traits: Record<string, string[]> = {};
  const selectedTraits = traitTypes.slice(
    0,
    Math.min(count, traitTypes.length)
  );

  selectedTraits.forEach(trait => {
    // For most traits, select 1-3 values
    const valueCount = Math.min(
      1 + Math.floor(Math.random() * 3),
      trait.values.length
    );
    traits[trait.key] = trait.values.slice(0, valueCount);
  });

  return traits;
}

function getUserConfig(userType: UserAuthType) {
  switch (userType) {
    case 'local':
      return {
        authType: 'local',
        origin: undefined,
        isBot: false,
        isLocal: true,
      };
    case 'github':
      return {
        authType: 'github',
        origin: undefined,
        isBot: false,
        isLocal: false,
      };
    case 'saml':
      return {
        authType: 'saml',
        origin: undefined,
        isBot: false,
        isLocal: false,
      };
    case 'oidc':
      return {
        authType: 'oidc',
        origin: undefined,
        isBot: false,
        isLocal: false,
      };
    case 'okta':
      return {
        authType: 'saml',
        origin: 'okta' as const,
        isBot: false,
        isLocal: false,
      };
    case 'scim':
      return {
        authType: 'saml',
        origin: 'scim' as const,
        isBot: false,
        isLocal: false,
      };
    case 'bot':
      return {
        authType: undefined,
        origin: undefined,
        isBot: true,
        isLocal: false,
      };
    default:
      return {
        authType: 'local',
        origin: undefined,
        isBot: false,
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
    isBot: config.isBot,
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
        <UserDetails user={user} sections={[UserRoles, UserTraits]} />
      </InfoGuideContainer>
    </SlidingSidePanel>
  );
}

export const BasicUser: Story = {
  args: userDetailsDefaultArgs,
};
