import React from 'react';
import { Link } from 'react-router-dom';

import Flex from 'design/Flex';
import Image from 'design/Image';

import Text from 'design/Text';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';

import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';
import cfg from 'teleport/config';

import { useGitHubFlow } from './useGitHubFlow';

export function Finish() {
  const { botConfig } = useGitHubFlow();

  return (
    <Flex
      width="600px"
      flexDirection="column"
      alignItems="center"
      mt={5}
      css={`
        margin-right: auto;
        margin-left: auto;
        text-align: center;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <Text mt={3} mb={2} typography="h4" bold>
        Your Machine User is Added to Teleport
      </Text>
      <Text mb={3}>
        Machine User {botConfig.botName} has been successfully added to this
        Teleport Cluster. You can see bot-{botConfig.botName} in the Teleport
        Users page and you can always find the sample GitHub Actions workflow
        again from the machine user's options.
      </Text>
      <Flex>
        <ButtonPrimary mr="4" as={Link} to={cfg.routes.users} size="large">
          View Machine Users
        </ButtonPrimary>
        <ButtonSecondary
          as={Link}
          to={cfg.getIntegrationEnrollRoute(null)}
          size="large"
        >
          Add Another Integration
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
