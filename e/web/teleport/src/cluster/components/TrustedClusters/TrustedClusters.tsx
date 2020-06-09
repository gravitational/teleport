import React from 'react';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from 'design/Layout';
import { Danger } from 'design/Alert';
import { Indicator, Text, Box, Flex, ButtonPrimary } from 'design';
import ResourceEditor from 'e-shared/components/ResourceEditor';
import useResources from 'e-shared/components/Resources/useResources';
import CardEmpty from 'teleport/components/CardEmpty';
import TrustedList from './TrustedList';
import DeleteTrustedClusterDialog from './DeleteTrustedClusterDialog';
import templates from './templates';
import useTrustedClusters from './useTrustedClusters';

export default function TrustedClusters() {
  const tclusters = useTrustedClusters();
  const isEmpty = tclusters.items.length === 0;
  const resources = useResources(tclusters.items, templates);

  const title =
    resources.status === 'creating'
      ? 'Add a new trusted cluster'
      : 'Edit trusted cluster';

  function remove() {
    return tclusters.remove(resources.item);
  }

  function save(content: string) {
    const isNew = resources.status === 'creating';
    return tclusters.save(content, isNew);
  }

  if (tclusters.isProcessing) {
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
        {tclusters.canCreate && (
          <ButtonPrimary
            ml="auto"
            width="240px"
            onClick={() => resources.create('trusted_cluster')}
          >
            Connect to Root Cluster
          </ButtonPrimary>
        )}
      </FeatureHeader>
      {tclusters.isFailed && <Danger>{tclusters.message} </Danger>}
      <Flex alignItems="start">
        {isEmpty && (
          <CardEmpty title="Not sharing cluster access to a root cluster" />
        )}
        {!isEmpty && (
          <TrustedList
            mt="4"
            flex="1"
            items={tclusters.items}
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
            TRUSTED CLUSTERS
          </Text>
          <Text typography="subtitle1" mb={3}>
            Trusted Clusters allows Teleport administrators to connect multiple
            clusters together and establish trust between them. Users of trusted
            clusters can seamlessly access the nodes of the cluster from the
            root cluster.
          </Text>
          <Text typography="subtitle1" mb={2}>
            Please{' '}
            <Text
              as="a"
              color="light"
              href="https://gravitational.com/teleport/docs/trustedclusters/"
              target="_blank"
            >
              view our documentation
            </Text>{' '}
            to learn more about trusted clusters.
          </Text>
        </Box>
      </Flex>
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          onSave={save}
          title={title}
          onClose={resources.disregard}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
        />
      )}
      {resources.status === 'removing' && (
        <DeleteTrustedClusterDialog
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={remove}
        />
      )}
    </FeatureBox>
  );
}
