/*
Copyright 2019-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useEffect, useState } from 'react';

import { Alert, Box, ButtonPrimary, Flex, Link, Text } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';
import useTeleport from 'teleport/useTeleport';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import { useServerSidePagination } from 'teleport/components/hooks';

import { RoleList } from './RoleList';
import DeleteRole from './DeleteRole';
import { useRoles, State } from './useRoles';

import templates from './templates';

export default function Container() {
  const ctx = useTeleport();
  const state = useRoles(ctx);
  return <Roles {...state} />;
}

export function Roles(props: State) {
  const { remove, save, fetch } = props;
  const [search, setSearch] = useState('');

  const serverSidePagination = useServerSidePagination({
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

  async function handleSave(content: string): Promise<void> {
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    const response = await save(name, content, isNew);
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

  async function handleDelete(): Promise<void> {
    await remove(resources.item.name);
    modifyFetchedData(p => ({
      ...p,
      agents: p.agents.filter(r => r.id !== resources.item.id),
    }));
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Roles</FeatureHeaderTitle>
        <ButtonPrimary ml="auto" width="240px" onClick={handleCreate}>
          CREATE NEW ROLE
        </ButtonPrimary>
      </FeatureHeader>
      {serverSidePagination.attempt.status === 'failed' && (
        <Alert children={serverSidePagination.attempt.statusText} />
      )}
      <Flex>
        <Box width="100%" mr="6" mb="4">
          <RoleList
            serversidePagination={serverSidePagination}
            onSearchChange={setSearch}
            search={search}
            onEdit={resources.edit}
            onDelete={resources.remove}
          />
        </Box>
        <Box
          ml="auto"
          width="240px"
          color="text.main"
          style={{ flexShrink: 0 }}
        >
          <Text typography="h6" mb={3} caps>
            Role-based access control
          </Text>
          <Text typography="subtitle1" mb={3}>
            Teleport Role-based access control (RBAC) provides fine-grained
            control over who can access resources and in which contexts. A
            Teleport role can be assigned automatically based on user identity
            when used with single sign-on (SSO).
          </Text>
          <Text>
            Learn more in{' '}
            <Link
              color="text.main"
              target="_blank"
              href="https://goteleport.com/docs/access-controls/guides/role-templates/"
            >
              the cluster management (RBAC)
            </Link>{' '}
            section of online documentation.
          </Text>
        </Box>
      </Flex>
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          docsURL="https://goteleport.com/docs/access-controls/guides/role-templates/"
          title={title}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onSave={handleSave}
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
