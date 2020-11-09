/*
Copyright 2020 Gravitational, Inc.

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
import { Danger } from 'design/Alert';
import { Indicator, Text, Box, Flex } from 'design';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';
import EmptyList from './EmptyList';
import ConnectorList from './ConnectorList';
import DeleteConnectorDialog from './DeleteConnectorDialog';
import AddMenu from './AddMenu';
import useAuthConnectors from './useAuthConnectors';
import templates from './templates';

export default function AuthConnectors() {
  const connectors = useAuthConnectors();
  const { message, isProcessing, isFailed, isSuccess } = connectors.attempt;
  const isEmpty = connectors.items.length === 0;
  const resources = useResources(connectors.items, templates);

  const title =
    resources.status === 'creating'
      ? 'Creating a new auth connector'
      : 'Editing auth connector';

  function remove() {
    return connectors.remove(resources.item);
  }

  function save(content: string) {
    const isNew = resources.status === 'creating';
    return connectors.save(content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <Box ml="auto" alignSelf="center" width="240px">
          <AddMenu onClick={resources.create} />
        </Box>
      </FeatureHeader>
      {isFailed && <Danger>{message} </Danger>}
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isSuccess && (
        <Flex alignItems="start">
          {isEmpty && (
            <Flex mt="4" width="100%" justifyContent="center">
              <EmptyList onCreate={resources.create} />
            </Flex>
          )}
          {!isEmpty && (
            <ConnectorList
              flex="1"
              items={connectors.items}
              onEdit={resources.edit}
              onDelete={resources.remove}
            />
          )}
          <Box
            ml="4"
            width="240px"
            color="text.primary"
            style={{ flexShrink: 0 }}
          >
            <Text typography="h6" mb={3}>
              AUTHENTICATION CONNECTORS
            </Text>
            <Text typography="subtitle1" mb={3}>
              Authentication connectors allow Teleport to authenticate users via
              an external identity source such as Okta, Active Directory,
              Github, etc. This authentication method is frequently called
              single sign-on (SSO).
            </Text>
            <Text typography="subtitle1" mb={2}>
              Please{' '}
              <Text
                as="a"
                color="light"
                href="https://gravitational.com/teleport/docs/enterprise/ssh_sso/"
                target="_blank"
              >
                view our documentation
              </Text>{' '}
              for samples of each connector.
            </Text>
          </Box>
        </Flex>
      )}
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          title={title}
          onSave={save}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onClose={resources.disregard}
        />
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={remove}
        />
      )}
    </FeatureBox>
  );
}
