import React from 'react';
import { generatePath } from 'react-router';
import { Flex, Text, Image, ButtonPrimary, ButtonSecondary } from 'design';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';

import { useDiscover } from 'teleport/Discover/useDiscover';

export function Finished() {
  const { exitFlow, agentMeta } = useDiscover();
  let resourceText;
  if (agentMeta?.resourceName) {
    resourceText = `SAML Application [${agentMeta.resourceName}] has been successfully added to
        this Teleport Cluster.`;
  }

  return (
    <Flex
      width="600px"
      flexDirection="column"
      alignItems="center"
      css={`
        margin: 0 auto;
        text-align: center;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <Text mt={3} mb={2} typography="h4" bold>
        SAML Application Successfully Added
      </Text>
      <Text mb={3}>
        {resourceText} You can now use Teleport as an identity provider to log
        into it.
      </Text>
      <Flex>
        <ButtonPrimary
          width="270px"
          size="large"
          onClick={() =>
            history.push(
              generatePath(cfg.routes.apps, { clusterId: cfg.proxyCluster }),
              true
            )
          }
          mr={3}
        >
          Browse Applications
        </ButtonPrimary>
        <ButtonSecondary width="270px" size="large" onClick={() => exitFlow()}>
          Add Another Resource
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
