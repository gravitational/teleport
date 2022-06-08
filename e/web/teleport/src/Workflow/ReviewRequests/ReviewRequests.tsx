import React from 'react';
import { Link } from 'react-router-dom';
import { useParams } from 'react-router';
import { Text, Flex } from 'design';
import { ArrowBack } from 'design/Icon';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'e-teleport/config';
import RequestList from './RequestList';
import RequestView from './RequestView';

export default function Workflow() {
  const { requestId } = useParams<{ requestId?: string }>();

  if (!requestId) {
    return (
      <FeatureBox>
        <FeatureHeader>
          <FeatureHeaderTitle>Review Requests</FeatureHeaderTitle>
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
              fontSize={8}
              to={cfg.getAccessRequestRoute()}
              style={{ textDecoration: 'none' }}
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
