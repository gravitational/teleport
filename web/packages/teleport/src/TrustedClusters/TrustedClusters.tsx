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

import { Box, Button, ButtonPrimary, Flex, H3, Indicator, Link } from 'design';
import { Danger } from 'design/Alert';
import Card from 'design/Card';
import Image from 'design/Image';
import { P } from 'design/Text/Text';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';

import { emptyPng } from './assets';
import DeleteTrust from './DeleteTrust';
import templates from './templates';
import TrustedList from './TrustedList';
import useTrustedClusters from './useTrustedClusters';

export default function TrustedClusters() {
  const { items, canCreate, remove, save, attempt } = useTrustedClusters();
  const isEmpty = attempt.status === 'success' && items.length === 0;
  const hasClusters = attempt.status === 'success' && items.length > 0;
  const resources = useResources(items, templates);

  const title =
    resources.status === 'creating'
      ? 'Add a new trusted root cluster'
      : 'Edit trusted root cluster';

  function onRemove() {
    return remove(resources.item.name);
  }

  function onSave(content: string) {
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    return save(name, content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Trusted Root Clusters</FeatureHeaderTitle>
        {hasClusters && (
          <Button
            intent="primary"
            fill="border"
            disabled={!canCreate}
            ml="auto"
            width="240px"
            onClick={() => resources.create('trusted_cluster')}
          >
            Connect to Root Cluster
          </Button>
        )}
      </FeatureHeader>
      {attempt.status === 'failed' && <Danger>{attempt.statusText} </Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isEmpty && (
        <Empty
          disabled={!canCreate}
          onCreate={() => resources.create('trusted_cluster')}
        />
      )}
      {hasClusters && (
        <Flex alignItems="start">
          <TrustedList
            mt="4"
            flex="1"
            items={items}
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
          onSave={onSave}
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
          onDelete={onRemove}
        />
      )}
    </FeatureBox>
  );
}

const Info = props => (
  <Box {...props}>
    <H3 mb={3}>Trusted Clusters</H3>
    <P>
      Trusted Clusters allow Teleport administrators to connect multiple
      clusters together and establish trust between them. Users of Trusted
      Clusters can seamlessly access the resources of the leaf cluster from the
      root cluster.
    </P>
    <P mb={2}>
      Please{' '}
      <Link
        color="text.main"
        href="https://goteleport.com/docs/admin-guides/management/admin/trustedclusters/"
        target="_blank"
      >
        view our documentation
      </Link>{' '}
      to learn more about Trusted Clusters.
    </P>
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
