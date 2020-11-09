/*
Copyright 2019-2020 Gravitational, Inc.

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

import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Indicator, Flex, Box, ButtonPrimary, Text } from 'design';
import { Danger } from 'design/Alert';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';
import useTeleport from 'teleport/useTeleport';
import RoleList from './RoleList';
import DeleteRole from './DeleteRole';
import useRoles from './useRoles';
import templates from './templates';

export default function Container() {
  const ctx = useTeleport();
  const state = useRoles(ctx);
  return <Roles {...state} />;
}

export function Roles(props: ReturnType<typeof useRoles>) {
  const { items, canCreate, remove, save, attempt } = props;
  const resources = useResources(items, templates);
  const { message, isProcessing, isFailed, isSuccess } = attempt;
  const title =
    resources.status === 'creating' ? 'Create a new role' : 'Edit role';

  function handleRemove() {
    return remove(resources.item);
  }

  function handleSave(content: string) {
    const isNew = resources.status === 'creating';
    return save(content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Roles</FeatureHeaderTitle>
        {isSuccess && canCreate && (
          <ButtonPrimary
            ml="auto"
            width="240px"
            onClick={() => resources.create('role')}
          >
            CREATE NEW ROLE
          </ButtonPrimary>
        )}
      </FeatureHeader>
      {isFailed && <Danger>{message} </Danger>}
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isSuccess && (
        <Flex>
          <Box width="100%" mr="6" mb="4">
            <RoleList
              items={items}
              onEdit={resources.edit}
              onDelete={resources.remove}
            />
          </Box>
          <Box
            ml="auto"
            width="240px"
            color="text.primary"
            style={{ flexShrink: 0 }}
          >
            <Text typography="h6" mb={3} caps>
              Role based access control
            </Text>
            <Text typography="subtitle1" mb={3}>
              Kubernetes and SSH authentication in one place. A Teleport role
              can be assigned based on user identity when used with single
              sign-on (SSO).
            </Text>
            <Text>
              Learn more in{' '}
              <Text
                as="a"
                color="light"
                target="_blank"
                href="https://gravitational.com/teleport/docs/enterprise/ssh_rbac/"
              >
                cluster management (RBAC)
              </Text>{' '}
              section of online documentation.
            </Text>
          </Box>
        </Flex>
      )}
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          docsURL="https://gravitational.com/teleport/docs/enterprise/ssh_rbac/"
          title={title}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onSave={handleSave}
          onClose={resources.disregard}
          directions={<Directions />}
        />
      )}
      {resources.status === 'removing' && (
        <DeleteRole
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={handleRemove}
        />
      )}
    </FeatureBox>
  );
}

function Directions() {
  return (
    <>
      WARNING Roles are defined using{' '}
      <Text
        as="a"
        color="light"
        target="_blank"
        href="https://en.wikipedia.org/wiki/YAML"
      >
        YAML format
      </Text>
      . YAML is sensitive to white space, please be careful.
    </>
  );
}
