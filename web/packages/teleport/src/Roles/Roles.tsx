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

import { useEffect, useState } from 'react';

import { Alert, Box, Button, Flex, Link } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { useToastNotifications } from 'shared/components/ToastNotification';
import { Attempt } from 'shared/hooks/useAsync';

import { useServerSidePagination } from 'teleport/components/hooks';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';
import { Role, RoleResource, RoleWithYaml } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import DeleteRole from './DeleteRole';
import { RoleEditorDialog } from './RoleEditor/RoleEditorDialog';
import { RoleList } from './RoleList';
import templates from './templates';
import { State, useRoles } from './useRoles';

export enum RoleDiffState {
  Disabled,
  Error,
  PolicyEnabled,
  LoadingSettings,
  WaitingForSync,
  DemoReady,
}

/** Optional set of props to render the role diff visualizer. */
export type RoleDiffProps = {
  roleDiffElement: React.ReactNode;
  updateRoleDiff: (role: Role) => void;

  /**
   * State of the attempt to fetch the information required by the role diff
   * visualizer. Required to show an error message in the editor UI.
   */
  // TODO(bl-nero): Make this property required once the Enterprise code is
  // updated.
  roleDiffAttempt?: Attempt<unknown>;
  clearRoleDiffAttempt?: () => void;
  enableDemoMode?: () => void;
  roleDiffState?: RoleDiffState;
  roleDiffErrorMessage?: string;
};

export type RolesProps = {
  roleDiffProps?: RoleDiffProps;
};

export function RolesContainer({ roleDiffProps }: RolesProps) {
  const ctx = useTeleport();
  const state = useRoles(ctx);
  return <Roles {...state} roleDiffProps={roleDiffProps} />;
}

const useNewRoleEditor = storageService.getUseNewRoleEditor();

export function Roles(props: State & RolesProps) {
  const { remove, create, update, fetch, rolesAcl } = props;
  const [search, setSearch] = useState('');
  const toastNotification = useToastNotifications();

  const serverSidePagination = useServerSidePagination<RoleResource>({
    pageSize: 20,
    fetchFunc: async (_, params) => {
      const { items, startKey } = await fetch(params);
      return { agents: items, startKey };
    },
    clusterId: '',
    params: { search },
  });
  const { modifyFetchedData } = serverSidePagination;

  const resources = useResources(
    serverSidePagination.fetchedData.agents,
    templates
  );
  const title =
    resources.status === 'creating' ? 'Create a new role' : 'Edit role';

  async function handleSave(role: Partial<RoleWithYaml>): Promise<void> {
    const response: RoleResource = await (resources.status === 'creating'
      ? create(role)
      : update(resources.item.name, role));

    toastNotification.add({
      severity: 'success',
      content:
        resources.status === 'creating'
          ? `Role ${response.name} has been created`
          : `Role ${response.name} has been updated`,
    });

    if (useNewRoleEditor) {
      // We don't really disregard anything, since we already saved the role;
      // this is done just to hide the new editor.
      resources.disregard();
    }

    // We cannot refetch the data right after saving because this backend
    // operation is not atomic.
    // There is a short delay between updating the resource
    // and having the updated value propagate to the cache.
    // Because of that, we have to update the current page manually.
    // TODO(gzdunek): Refactor this into a reusable hook, like `useResourceUpdate`.
    modifyFetchedData(p => {
      const index = p.agents.findIndex(a => a.id === response.id);
      if (index >= 0) {
        const newAgents = [...p.agents];
        newAgents[index] = response;
        return {
          ...p,
          agents: newAgents,
        };
      } else {
        return {
          ...p,
          agents: [response, ...p.agents],
        };
      }
    });
  }

  useEffect(() => {
    serverSidePagination.fetch();
  }, [search]);

  const handleCreate = () => {
    resources.create('role');

    userEventService.captureUserEvent({
      event: CaptureEvent.CreateNewRoleClickEvent,
    });
  };

  async function handleEdit(id: string) {
    resources.edit(id);
  }

  async function handleDelete(): Promise<void> {
    await remove(resources.item.name);
    modifyFetchedData(p => ({
      ...p,
      agents: p.agents.filter(r => r.id !== resources.item.id),
    }));
    // The new editor doesn't use `resources` to delete, so we need to close it
    // by resetting the state here.
    resources.disregard();
  }

  const canCreate = rolesAcl.create;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Roles</FeatureHeaderTitle>
        <InfoGuideButton config={{ guide: <InfoGuide /> }}>
          <HoverTooltip
            placement="bottom"
            tipContent={
              !canCreate ? (
                <MissingPermissionsTooltip
                  missingPermissions={['roles.create']}
                />
              ) : (
                ''
              )
            }
          >
            <Button
              data-testid="create_new_role_button"
              intent="primary"
              fill={
                serverSidePagination.attempt.status === 'success' &&
                serverSidePagination.fetchedData.agents.length === 0
                  ? 'filled'
                  : 'border'
              }
              ml="auto"
              width="240px"
              disabled={!canCreate}
              onClick={handleCreate}
            >
              Create New Role
            </Button>
          </HoverTooltip>
        </InfoGuideButton>
      </FeatureHeader>
      {serverSidePagination.attempt.status === 'failed' && (
        <Alert>{serverSidePagination.attempt.statusText}</Alert>
      )}
      <Flex flex="1">
        <Box flex="1" mb="4">
          <RoleList
            serversidePagination={serverSidePagination}
            onSearchChange={setSearch}
            search={search}
            onEdit={handleEdit}
            onDelete={resources.remove}
            rolesAcl={rolesAcl}
          />
        </Box>

        {/* New editor. */}
        {useNewRoleEditor && (
          <RoleEditorDialog
            open={
              resources.status === 'creating' || resources.status === 'editing'
            }
            onClose={() => {
              resources.disregard();
              props.roleDiffProps?.clearRoleDiffAttempt();
            }}
            resources={resources}
            onSave={handleSave}
            roleDiffProps={props.roleDiffProps}
          />
        )}
      </Flex>

      {/* Old editor. */}
      {!useNewRoleEditor &&
        (resources.status === 'creating' || resources.status === 'editing') && (
          <ResourceEditor
            docsURL="https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/"
            title={title}
            text={resources.item.content}
            name={resources.item.name}
            isNew={resources.status === 'creating'}
            onSave={yaml => handleSave({ yaml })}
            onClose={resources.disregard}
            directions={<Directions />}
            kind={resources.item.kind}
          />
        )}

      {resources.status === 'removing' && (
        <DeleteRole
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={handleDelete}
        />
      )}
    </FeatureBox>
  );
}

function Directions() {
  return (
    <>
      WARNING Roles are defined using{' '}
      <Link
        color="text.main"
        target="_blank"
        href="https://en.wikipedia.org/wiki/YAML"
      >
        YAML format
      </Link>
      . YAML is sensitive to white space, so please be careful.
    </>
  );
}

const InfoGuideReferenceLinks = {
  PresetRoles: {
    title: 'Teleport Preset Roles',
    href: 'https://goteleport.com/docs/reference/access-controls/roles/#preset-roles',
  },
  RoleTemplates: {
    title: 'Teleport Role Templates',
    href: 'https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/',
  },
};

function InfoGuide() {
  return (
    <Box>
      <InfoParagraph>
        Teleport Role-based access control (RBAC) provides fine-grained control
        over who can access resources and in which contexts. A Teleport role can
        be assigned automatically based on user identity when used with single
        sign-on (SSO).
      </InfoParagraph>
      <InfoParagraph>
        New clusters have several{' '}
        <InfoExternalTextLink href={InfoGuideReferenceLinks.PresetRoles.href}>
          preset roles
        </InfoExternalTextLink>
        . These are convenient for getting started but are very permissive, and
        we recommend you follow our{' '}
        <InfoExternalTextLink href={InfoGuideReferenceLinks.RoleTemplates.href}>
          best practices guide
        </InfoExternalTextLink>{' '}
        to create your own.
      </InfoParagraph>
      <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
    </Box>
  );
}
