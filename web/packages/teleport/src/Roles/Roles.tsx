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

import React, { useEffect, useState } from 'react';
import { Alert, Box, Button, Flex, H3, Indicator, Link } from 'design';
import { P } from 'design/Text/Text';
import { useAsync } from 'shared/hooks/useAsync';
import { Danger } from 'design/Alert';
import { useTheme } from 'styled-components';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import { HoverTooltip } from 'shared/components/ToolTip';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useTeleport from 'teleport/useTeleport';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { useServerSidePagination } from 'teleport/components/hooks';
import { storageService } from 'teleport/services/storageService';
import { RoleWithYaml, Role, RoleResource } from 'teleport/services/resources';
import useResources, {
  State as ResourcesState,
} from 'teleport/components/useResources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { RoleList } from './RoleList';
import DeleteRole from './DeleteRole';
import { useRoles, State } from './useRoles';
import { RoleEditor } from './RoleEditor';
import templates from './templates';

export function RolesContainer() {
  const ctx = useTeleport();
  const state = useRoles(ctx);
  return <Roles {...state} />;
}

const useNewRoleEditor = storageService.getUseNewRoleEditor();

export function Roles(props: State) {
  const { remove, create, update, fetch, rolesAcl } = props;
  const [search, setSearch] = useState('');

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
  }

  function handleRoleEditorDelete() {
    const id = resources.item?.id;
    if (id) {
      resources.remove(id);
    }
  }

  const canCreate = rolesAcl.create;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Roles</FeatureHeaderTitle>
        <HoverTooltip
          position="bottom"
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
      </FeatureHeader>
      {serverSidePagination.attempt.status === 'failed' && (
        <Alert children={serverSidePagination.attempt.statusText} />
      )}
      <Flex flex="1">
        <Box width="100%" mr="6" mb="4">
          <RoleList
            serversidePagination={serverSidePagination}
            onSearchChange={setSearch}
            search={search}
            onEdit={handleEdit}
            onDelete={resources.remove}
            rolesAcl={rolesAcl}
          />
        </Box>

        {/* New editor or descriptive text, depending on state. */}
        {useNewRoleEditor &&
        (resources.status === 'creating' || resources.status === 'editing') ? (
          <RoleEditorAdapter
            resources={resources}
            onSave={handleSave}
            onDelete={handleRoleEditorDelete}
          />
        ) : (
          <Box
            ml="auto"
            width="240px"
            color="text.main"
            style={{ flexShrink: 0 }}
          >
            <H3 mb={3}>Role-based access control</H3>
            <P mb={3}>
              Teleport Role-based access control (RBAC) provides fine-grained
              control over who can access resources and in which contexts. A
              Teleport role can be assigned automatically based on user identity
              when used with single sign-on (SSO).
            </P>
            <P>
              Learn more in{' '}
              <Link
                color="text.main"
                target="_blank"
                href="https://goteleport.com/docs/access-controls/guides/role-templates/"
              >
                the cluster management (RBAC)
              </Link>{' '}
              section of online documentation.
            </P>
          </Box>
        )}
      </Flex>

      {/* Old editor. */}
      {!useNewRoleEditor &&
        (resources.status === 'creating' || resources.status === 'editing') && (
          <ResourceEditor
            docsURL="https://goteleport.com/docs/access-controls/guides/role-templates/"
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

/**
 * This component is responsible for converting from the `Resource`
 * representation of a role to a more accurate `RoleWithYaml` structure. The
 * conversion is asynchronous and it's performed on the server side.
 */
function RoleEditorAdapter({
  resources,
  onSave,
  onDelete,
}: {
  resources: ResourcesState;
  onSave: (role: Partial<RoleWithYaml>) => Promise<void>;
  onDelete: () => void;
}) {
  const theme = useTheme();
  const [convertAttempt, convertToRole] = useAsync(
    async (yaml: string): Promise<RoleWithYaml | null> => {
      if (resources.status === 'creating' || !resources.item) {
        return null;
      }
      return {
        yaml,
        object: await yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
          yaml,
        }),
      };
    }
  );

  const originalContent = resources.item?.content ?? '';
  useEffect(() => {
    convertToRole(originalContent);
  }, [originalContent]);

  return (
    <Flex
      flexDirection="column"
      p={4}
      borderLeft={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      width="900px"
    >
      {convertAttempt.status === 'processing' && (
        <Flex
          flexDirection="column"
          alignItems="center"
          justifyContent="center"
          flex="1"
        >
          <Indicator />
        </Flex>
      )}
      {convertAttempt.status === 'error' && (
        <Danger>{convertAttempt.statusText}</Danger>
      )}
      {convertAttempt.status === 'success' && (
        <RoleEditor
          originalRole={convertAttempt.data}
          onCancel={resources.disregard}
          onSave={onSave}
          onDelete={onDelete}
        />
      )}
    </Flex>
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
