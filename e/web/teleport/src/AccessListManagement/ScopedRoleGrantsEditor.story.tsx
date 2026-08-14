import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useState } from 'react';

import { Box, Text } from 'design';
import Validation from 'shared/components/Validation';

import cfg from 'e-teleport/config';
import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import type {
  ScopedRoleGrant,
  ScopedRoleListItem,
} from 'e-teleport/services/accessmanagement';

import { ScopedRoleGrantsEditor } from './ScopedRoleGrantsEditor';

const rootScopedRolesPath = cfg.getRootScopedRolesUrl({}).split('?')[0];

const scopedRoles: ScopedRoleListItem[] = [
  {
    name: 'team-admin',
    scope: '/',
    assignableScopes: ['/dev/**', '/staging/**'],
  },
  {
    name: 'support',
    scope: '/',
    assignableScopes: ['/customers'],
  },
  {
    name: 'audit',
    scope: '/',
    assignableScopes: ['/'],
  },
];

export default {
  title: 'TeleportE/AccessLists/ScopedRoleGrantsEditor',
};

export const EmptyWithoutAvailableRoles: StoryObj = {
  beforeEach({ msw }) {
    msw.use(makeRootScopedRolesHandler([]));
  },

  render() {
    return <EditorStory initialGrants={[]} />;
  },
};

export const EmptyWithAvailableRoles: StoryObj = {
  beforeEach({ msw }) {
    msw.use(makeRootScopedRolesHandler(scopedRoles));
  },

  render() {
    return <EditorStory initialGrants={[]} />;
  },
};

export const ExistingGrant: StoryObj = {
  beforeEach({ msw }) {
    msw.use(makeRootScopedRolesHandler(scopedRoles));
  },

  render() {
    return (
      <EditorStory
        initialGrants={[{ role: '/::team-admin', scope: '/dev/platform' }]}
      />
    );
  },
};

export const ValidationError: StoryObj = {
  beforeEach({ msw }) {
    msw.use(makeRootScopedRolesHandler(scopedRoles));
  },

  render() {
    return (
      <EditorStory
        initialGrants={[{ role: '/::support', scope: '/dev/platform' }]}
      />
    );
  },
};

function EditorStory({
  initialGrants,
  userKind = 'Members',
}: {
  initialGrants: ScopedRoleGrant[];
  userKind?: 'Members' | 'Owners';
}) {
  const [scopedRoleGrants, setScopedRoleGrants] =
    useState<ScopedRoleGrant[]>(initialGrants);

  return (
    <TeleportProviderBasicE>
      <Validation>
        <Box width="720px" p={4}>
          <ScopedRoleGrantsEditor
            isDisabled={false}
            userKind={userKind}
            scopedRoleGrants={scopedRoleGrants}
            updateScopedRoleGrants={setScopedRoleGrants}
          />
          <Box mt={3}>
            <Text typography="body3" color="text.slightlyMuted">
              Current grants
            </Text>
            <Box as="pre" mt={2} p={3} backgroundColor="levels.sunken" mb={0}>
              {JSON.stringify(scopedRoleGrants, null, 2)}
            </Box>
          </Box>
        </Box>
      </Validation>
    </TeleportProviderBasicE>
  );
}

function makeRootScopedRolesHandler(roles: ScopedRoleListItem[]) {
  return http.get(rootScopedRolesPath, ({ request }) => {
    const filter =
      new URL(request.url).searchParams.get('filter')?.trim().toLowerCase() ||
      '';

    const filteredRoles = filter
      ? roles.filter(role => role.name.toLowerCase().includes(filter))
      : roles;

    return HttpResponse.json({
      roles: filteredRoles.map(role => ({
        name: role.name,
        scope: role.scope,
        assignableScopes: role.assignableScopes,
      })),
      startKey: '',
    });
  });
}
