import React from 'react';
import { Link } from 'react-router-dom';

import Flex from 'design/Flex';
import Image from 'design/Image';

import Text from 'design/Text';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';

import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';
import cfg from 'teleport/config';

export function Finish() {
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
        External Audit Storage Successfully Added
      </Text>
      <Text mb={3}>Audit data will now be stored in your infrastructure.</Text>
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
