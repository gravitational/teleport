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
import { Danger } from 'design/Alert';
import { Indicator, Text, Box, Flex, ButtonPrimary, Link } from 'design';
import Card from 'design/Card';
import Image from 'design/Image';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';

import useResources from 'teleport/components/useResources';

import DeleteTrust from './DeleteTrust';
import templates from './templates';
import TrustedList from './TrustedList';
import useTrustedClusters from './useTrustedClusters';
import { emptyPng } from './assets';

export default function TrustedClusters() {
  const tclusters = useTrustedClusters();
  const isEmpty = tclusters.isSuccess && tclusters.items.length === 0;
  const hasClusters = tclusters.isSuccess && tclusters.items.length > 0;
  const resources = useResources(tclusters.items, templates);

  const title =
    resources.status === 'creating'
      ? 'Add a new trusted cluster'
      : 'Edit trusted cluster';

  function remove() {
    return tclusters.remove(resources.item.name);
  }

  function save(content: string) {
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    return tclusters.save(name, content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Trusted Clusters</FeatureHeaderTitle>
        {hasClusters && (
          <ButtonPrimary
            disabled={!tclusters.canCreate}
            ml="auto"
            width="240px"
            onClick={() => resources.create('trusted_cluster')}
          >
            Connect to Root Cluster
          </ButtonPrimary>
        )}
      </FeatureHeader>
      {tclusters.isFailed && <Danger>{tclusters.message} </Danger>}
      {tclusters.isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isEmpty && (
        <Empty
          disabled={!tclusters.canCreate}
          onCreate={() => resources.create('trusted_cluster')}
        />
      )}
      {hasClusters && (
        <Flex alignItems="start">
          <TrustedList
            mt="4"
            flex="1"
            items={tclusters.items}
            onEdit={resources.edit}
            onDelete={resources.remove}
          />
          <Info
            ml="4"
            width="240px"
            color="text.main"
            style={{ flexShrink: 0 }}
          />
        </Flex>
      )}
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
        <DeleteTrust
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={remove}
        />
      )}
    </FeatureBox>
  );
}

const Info = props => (
  <Box {...props}>
    <Text typography="h6" mb={3}>
      TRUSTED CLUSTERS
    </Text>
    <Text typography="subtitle1" mb={3}>
      Trusted Clusters allow Teleport administrators to connect multiple
      clusters together and establish trust between them. Users of Trusted
      Clusters can seamlessly access the nodes of the cluster from the root
      cluster.
    </Text>
    <Text typography="subtitle1" mb={2}>
      Please{' '}
      <Link
        color="light"
        href="https://goteleport.com/docs/setup/admin/trustedclusters/"
        target="_blank"
      >
        view our documentation
      </Link>{' '}
      to learn more about Trusted Clusters.
    </Text>
  </Box>
);

const Empty = (props: EmptyProps) => {
  return (
    <Card
      maxWidth="700px"
      mt={4}
      mx="auto"
      py={4}
      as={Flex}
      alignItems="center"
      flex="0 0 auto"
    >
      <Box mx="4">
        <Image width="180px" src={emptyPng} />
      </Box>
      <Box>
        <Info pr={4} mb={6} />
        <ButtonPrimary
          disabled={props.disabled}
          title={
            props.disabled
              ? 'You do not have access to add a trusted cluster'
              : ''
          }
          onClick={props.onCreate}
          mb="2"
          mx="auto"
          width="240px"
        >
          Connect to Root Cluster
        </ButtonPrimary>
      </Box>
    </Card>
  );
};

type EmptyProps = {
  onCreate(): void;
  disabled: boolean;
};
