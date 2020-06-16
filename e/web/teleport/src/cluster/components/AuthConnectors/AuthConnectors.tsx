import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Danger } from 'design/Alert';
import { Indicator, Text, Box, Flex } from 'design';
import ResourceEditor from 'e-shared/components/ResourceEditor';
import useResources from 'e-shared/components/Resources/useResources';
import {
  EmptyList,
  ConnectorList,
  DeleteConnectorDialog,
  AddMenu,
} from 'e-shared/components/AuthConnectors';
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

  if (isProcessing) {
    return (
      <Flex justifyContent="center">
        <Indicator />
      </Flex>
    );
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
