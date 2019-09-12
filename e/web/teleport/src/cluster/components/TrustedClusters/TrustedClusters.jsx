import React from 'react';
import PropTypes from 'prop-types';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'design/Layout';
import { withState } from 'shared/hooks';
import Indicator from 'design/Indicator';
import { Danger } from 'design/Alert';
import { Text, Box, Flex, ButtonPrimary } from 'design';
import ResourceEditor from 'e-shared/components/ResourceEditor';

import CardEmpty from 'gravity/components/CardEmpty';
import TrustedClusterList from './ClusterList';
import DeleteTrustedClusterDialog from './DeleteTrustedClusterDialog';
import useTrustedClusters from './useTrustedClusters';
import yaml from './templates';

export function TrustedClusters(props) {
  const { attempt, trustedClusters, canCreate } = props;
  const isEmpty = trustedClusters.length === 0;
  const [selected, selectedActions] = useSelectedItem(trustedClusters);

  function onDelete() {
    return props.onDelete(selected.connector);
  }

  function onSave(content) {
    return props.onSave(content, selected.isCreating);
  }

  const { message, isProcessing, isFailed } = attempt;

  const title = selected.isCreating
    ? 'Add a new trusted cluster'
    : 'Edit trusted cluster';

  if (isProcessing) {
    return (
      <Flex justifyContent="center">
        <Indicator />
      </Flex>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Trusted Clusters</FeatureHeaderTitle>
        {canCreate && (
          <ButtonPrimary
            ml="auto"
            width="260px"
            onClick={selectedActions.onCreate}
          >
            ADD TRUSTED CLUSTER
          </ButtonPrimary>
        )}
      </FeatureHeader>
      {isFailed && <Danger>{message} </Danger>}
      <Flex alignItems="start">
        {isEmpty && <CardEmpty title="No Trusted Clusters Found"></CardEmpty>}
        {!isEmpty && (
          <TrustedClusterList
            flex="1"
            items={trustedClusters}
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
            TRUSTED CLUSTERS
          </Text>
          <Text typography="subtitle1" mb={3}>
            Trusted Clusters allows Teleport administrators to connect multiple
            clusters together and establish trust between them. Users of trusted
            clusters can seamlessly SSH into the nodes of this cluster.
          </Text>
          <Text typography="subtitle1" mb={2}>
            Please{' '}
            <Text
              as="a"
              color="light"
              href="https://gravitational.co/teleport/docs/trustedclusters/"
              target="_blank"
            >
              view our documentation
            </Text>{' '}
            to learn more about trusted clusters.
          </Text>
        </Box>
      </Flex>
      {(selected.isCreating || selected.isEditing) && (
        <ResourceEditor
          onSave={onSave}
          title={title}
          onClose={selectedActions.onCancel}
          text={selected.item.content}
          name={selected.item.name}
          isNew={selected.isCreating}
        />
      )}
      {selected.isDeleting && (
        <DeleteTrustedClusterDialog
          name={selected.item.name}
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
  item: null,
};

function useSelectedItem(items) {
  const [state, setState] = React.useState({
    ...defaultState,
  });

  const onCreate = () => {
    setState({
      ...defaultState,
      isCreating: true,
      item: {
        content: yaml,
      },
    });
  };

  const onCancel = () => {
    setState({
      ...defaultState,
      item: null,
    });
  };

  const onEdit = id => {
    const item = items.find(c => c.id === id);
    setState({
      ...defaultState,
      isEditing: true,
      item,
    });
  };

  const onDelete = id => {
    const item = items.find(c => c.id === id);
    setState({
      ...defaultState,
      isDeleting: true,
      item,
    });
  };

  return [state, { onCreate, onEdit, onCancel, onDelete }];
}

TrustedClusters.propTypes = {
  trustedClusters: PropTypes.array.isRequired,
  canCreate: PropTypes.bool.isRequired,
  onSave: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
};

export default withState(() => {
  const trustedClusters = useTrustedClusters();
  return trustedClusters;
})(TrustedClusters);
