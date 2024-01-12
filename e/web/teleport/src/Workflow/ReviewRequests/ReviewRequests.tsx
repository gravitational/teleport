import React from 'react';
import { Link } from 'react-router-dom';
import { useParams } from 'react-router';
import { Text, Flex, ButtonPrimary } from 'design';
import { ArrowBack } from 'design/Icon';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';

import cfg from 'e-teleport/config';

import RequestList from './RequestList';
import RequestView from './RequestView';

const NewRequestButton = ({ clusterId }: { clusterId: string }) => {
  return (
    <Link
      to={{
        pathname: `${cfg.getNewAccessRequestRoute(clusterId)}`,
      }}
      style={{ textDecoration: 'none', marginLeft: 'auto' }}
    >
      <ButtonPrimary
        textTransform="none"
        title="New Access Request"
        width="240px"
      >
        New Access Request
      </ButtonPrimary>
    </Link>
  );
};

export default function Workflow() {
  const { requestId } = useParams<{ requestId?: string }>();
  const { clusterId } = useStickyClusterId();

  if (!requestId) {
    return (
      <FeatureBox>
        <FeatureHeader alignItems="center" justifyContent="space-between">
          <FeatureHeaderTitle>Review Requests</FeatureHeaderTitle>
          <NewRequestButton clusterId={clusterId} />
        </FeatureHeader>
        <RequestList />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={cfg.getAccessRequestRoute()}
            />
            <Flex mr={4} alignItems="baseline">
              <Text mr={3}>Request</Text>
              <Text typography="body1">{requestId}</Text>
            </Flex>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <RequestView />
    </FeatureBox>
  );
}
