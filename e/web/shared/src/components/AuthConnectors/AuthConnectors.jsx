import React from 'react';
import PropTypes from 'prop-types';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from 'design/Layout';
import Indicator from 'design/Indicator';
import { Danger } from 'design/Alert';
import { Text, Box, Flex } from 'design';
import { useState } from 'shared/hooks';
import ResourceEditor from 'e-shared/components/ResourceEditor';
import AddMenu from './AddMenu';
import { getTemplate } from './templates';
import EmptyList from './EmptyList';
import ConnectorList from './ConnectorList';
import DeleteConnectorDialog from './DeleteConnectorDialog';

export default function AuthConnectors(props) {
  const { attempt, connectors, canCreate } = props;
  const isEmpty = connectors.length === 0;
  const [selected, selectedActions] = useSelection(connectors);

  function onDelete() {
    return props.onDelete(selected.connector);
  }

  function onSave(content) {
    return props.onSave(content, selected.isCreating);
  }

  const { message, isProcessing, isFailed } = attempt;

  const title = selected.isCreating
    ? 'Creating a new auth connector'
    : 'Editing auth connector';

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
          <AddMenu onClick={selectedActions.onCreate} disabled={!canCreate} />
        </Box>
      </FeatureHeader>
      {isFailed && <Danger>{message} </Danger>}
      <Flex alignItems="start">
        {isEmpty && (
          <Flex mt="6" width="100%" justifyContent="center">
            <EmptyList onCreate={selectedActions.onCreate} />
          </Flex>
        )}
        {!isEmpty && (
          <ConnectorList
            flex="1"
            items={connectors}
            onEdit={selectedActions.onEdit}
            onDelete={selectedActions.onDelete}
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
            Authentication connectors allow Gravity to authenticate users via an
            external identity source such as Okta, Active Directory, Github,
            etc. This authentication method is frequenty called single sign-on
            (SSO).
          </Text>
          <Text typography="subtitle1" mb={2}>
            Please{' '}
            <Text
              as="a"
              color="light"
              href="https://gravitational.com/gravity/docs/cluster/#configuring-openid-connect"
              target="_blank"
            >
              view our documentation
            </Text>{' '}
            for samples of each connector.
          </Text>
        </Box>
      </Flex>
      {(selected.isCreating || selected.isEditing) && (
        <ResourceEditor
          onSave={onSave}
          title={title}
          onClose={selectedActions.onCancel}
          text={selected.connector.content}
          name={selected.connector.name}
          isNew={selected.isCreating}
        />
      )}
      {selected.isDeleting && (
        <DeleteConnectorDialog
          name={selected.connector.name}
          onClose={selectedActions.onCancel}
          onDelete={onDelete}
        />
      )}
    </FeatureBox>
  );
}

const defaultState = {
  isCreating: false,
  isEditing: false,
  isDeleting: false,
  connector: null,
};

function useSelection(connectors) {
  const [state, setState] = useState({
    ...defaultState,
  });

  const onCreate = kind => {
    const content = getTemplate(kind);
    setState({
      ...defaultState,
      isCreating: true,
      connector: {
        content,
      },
    });
  };

  const onCancel = () => {
    setState({
      ...defaultState,
      connector: null,
    });
  };

  const onEdit = id => {
    const connector = connectors.find(c => c.id === id);
    setState({
      ...defaultState,
      isEditing: true,
      connector,
    });
  };

  const onDelete = id => {
    const connector = connectors.find(c => c.id === id);
    setState({
      ...defaultState,
      isDeleting: true,
      connector,
    });
  };

  return [state, { onCreate, onEdit, onCancel, onDelete }];
}

AuthConnectors.propTypes = {
  connectors: PropTypes.array.isRequired,
  canCreate: PropTypes.bool.isRequired,
  onSave: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
};
