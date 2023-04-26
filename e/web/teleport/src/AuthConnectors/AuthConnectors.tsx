import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Indicator, Text, Box, Flex, Alert } from 'design';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';

import DeleteConnectorDialog from 'teleport/AuthConnectors/DeleteConnectorDialog';

import EmptyList from './EmptyList';
import ConnectorList from './ConnectorList';
import AddMenu from './AddMenu';
import useAuthConnectors, { State } from './useAuthConnectors';
import templates from './templates';

export default function Container() {
  const state = useAuthConnectors();
  return <AuthConnectors {...state} />;
}

export function AuthConnectors(props: State) {
  const { attempt, items, remove, save } = props;
  const isEmpty = items.length === 0;
  const resources = useResources(items, templates);

  const title =
    resources.status === 'creating'
      ? 'Creating a new auth connector'
      : 'Editing auth connector';

  function handleOnRemove() {
    return remove(resources.item);
  }

  function handleOnSave(content: string) {
    const kind = resources.item.kind;
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    return save(kind, name, content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <Box ml="auto" alignSelf="center" width="240px">
          <AddMenu onClick={resources.create} />
        </Box>
      </FeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <Flex alignItems="start">
          {isEmpty && (
            <Flex mt="4" width="100%" justifyContent="center">
              <EmptyList onCreate={resources.create} />
            </Flex>
          )}
          {!isEmpty && (
            <ConnectorList
              items={items}
              onEdit={resources.edit}
              onDelete={resources.remove}
            />
          )}
          <Box ml="4" width="240px" color="text.main" style={{ flexShrink: 0 }}>
            <Text typography="h6" mb={3}>
              AUTHENTICATION CONNECTORS
            </Text>
            <Text typography="subtitle1" mb={3}>
              Authentication connectors allow Teleport to authenticate users via
              an external identity source such as Okta, Active Directory,
              GitHub, etc. This authentication method is frequently called
              single sign-on (SSO).
            </Text>
            <Text typography="subtitle1" mb={2}>
              Please{' '}
              <Text
                as="a"
                color="text.main"
                href="https://goteleport.com/docs/enterprise/sso/"
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
          onSave={handleOnSave}
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
          onDelete={handleOnRemove}
        />
      )}
    </FeatureBox>
  );
}
