/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { User, UserOrigin } from 'teleport/services/user';

import { UserDetails } from './UserDetails';

type UserType = 'local' | 'github' | 'saml' | 'oidc' | 'okta' | 'scim' | 'bot';

type StoryProps = {
  userType: UserType;
  userName: string;
  rolesCount: number;
  hasTraits: boolean;
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

const meta: Meta<StoryProps> = {
  title: 'Teleport/Users/UserDetails',
  component: Story,
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
      options: ['local', 'github', 'saml', 'oidc', 'okta', 'scim', 'bot'],
    },
    userName: {
      control: { type: 'text' },
    },
    rolesCount: {
      control: { type: 'select' },
      options: [3, 7, 10, 200],
    },
    hasTraits: {
      control: { type: 'boolean' },
    },
  },
  args: {
    userType: 'local',
    userName: 'john.doe',
    rolesCount: 3,
    hasTraits: true,
  },
};

export default meta;

type Story = StoryObj<StoryProps>;

function Story(props: StoryProps) {
  const generateRoles = (count: number) => {
    const adjectives = ['readonly', 'enterprise', 'dev', 'jit', 'admin', 'system', 'remote', 'staging', 'prod', 'temp'];
    const roleNouns = ['auditor', 'editor', 'operator', 'viewer', 'manager', 'analyst', 'developer', 'security', 'support', 'backup'];
    const baseRoles = ['access', 'auditor', 'editor'];
    
    const generateRole = (index: number) => {
      const adjIndex = index % adjectives.length;
      const nounIndex = index % roleNouns.length;
      return `${adjectives[adjIndex]}-${roleNouns[nounIndex]}`;
    };
    
    return [
      ...baseRoles.slice(0, Math.min(count, baseRoles.length)),
      ...Array.from({ length: Math.max(0, count - baseRoles.length) }, (_, i) => generateRole(i))
    ];
  };

  const getUserConfig = (userType: UserType) => {
    switch (userType) {
      case 'local':
        return { authType: 'local', origin: undefined, isBot: false, isLocal: true };
      case 'github':
        return { authType: 'github', origin: undefined, isBot: false, isLocal: false };
      case 'saml':
        return { authType: 'saml', origin: undefined, isBot: false, isLocal: false };
      case 'oidc':
        return { authType: 'oidc', origin: undefined, isBot: false, isLocal: false };
      case 'okta':
        return { authType: 'saml', origin: 'okta' as const, isBot: false, isLocal: false };
      case 'scim':
        return { authType: 'saml', origin: 'scim' as const, isBot: false, isLocal: false };
      case 'bot':
        return { authType: undefined, origin: undefined, isBot: true, isLocal: false };
      default:
        return { authType: 'local', origin: undefined, isBot: false, isLocal: true };
    }
  };

  const config = getUserConfig(props.userType);

  const user: User = {
    name: props.userName,
    authType: config.authType,
    origin: config.origin,
    isBot: config.isBot,
    isLocal: config.isLocal,
    roles: generateRoles(props.rolesCount),
    allTraits: props.hasTraits ? {
      logins: ['john', 'root', 'ubuntu'],
      groups: ['admin', 'developers', 'security'],
      department: ['engineering'],
      team: ['platform'],
      environment: ['production'],
    } : undefined,
  };

  return (
    <UserDetails
      isVisible={true}
      onClose={() => {}}
      user={user}
    />
  );
}

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        http.get('/v1/webapi/users/:username', ({ params }) => {
          const { username } = params;
          
          // Generate 10 roles to show the "+ 3 more" button
          const adjectives = ['readonly', 'enterprise', 'dev', 'jit', 'admin', 'system', 'remote'];
          const roleNouns = ['auditor', 'editor', 'operator', 'viewer', 'manager', 'analyst', 'developer'];
          const baseRoles = ['access', 'auditor', 'editor'];
          
          const generateRole = (index: number) => {
            const adjIndex = index % adjectives.length;
            const nounIndex = index % roleNouns.length;
            return `${adjectives[adjIndex]}-${roleNouns[nounIndex]}`;
          };
          
          const roles = [
            ...baseRoles,
            ...Array.from({ length: 7 }, (_, i) => generateRole(i))
          ];
          
          const mockUser: User = {
            name: username as string,
            authType: 'local',
            isLocal: true,
            roles,
            allTraits: {
              logins: ['john', 'root', 'ubuntu'],
              groups: ['admin', 'developers', 'security'],
              department: ['engineering'],
              team: ['platform'],
              environment: ['production'],
            },
          };
          
          return HttpResponse.json(mockUser);
        }),
      ],
    },
  },
};