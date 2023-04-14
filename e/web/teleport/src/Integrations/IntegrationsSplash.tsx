import React from 'react';
import { Link as InternalLink } from 'react-router-dom';

import {
  ButtonPrimary,
  ButtonOutlined,
  Flex,
  Image,
  Link as ExternalLink,
  Text,
} from 'design';

import pluginsWheel from 'design/assets/images/icons/plugins.svg';

import cfg from 'e-teleport/config';

export function IntegrationsSplash() {
  return (
    <Flex flexDirection="column" gap="4" alignItems="center">
      <Image maxHeight="400px" src={pluginsWheel} />
      <Flex flexDirection="column" gap="2" maxWidth="540px">
        <Text typography="h3" fontWeight="bold" textAlign="center">
          Enroll your first integration to Teleport
        </Text>
        <Text typography="body1" textAlign="center">
          Teleport integrations can connect your cluster to external apps for
          tasks such as alerting cluster administrators when team members make
          access requests.
        </Text>
      </Flex>
      <Flex justifyContent="center" gap="2">
        <InternalLink to={cfg.oss.getIntegrationEnrollRoute()}>
          <ButtonPrimary width="240px">Enroll new integration</ButtonPrimary>
        </InternalLink>
        <ExternalLink
          href="https://goteleport.com/docs/access-controls/access-request-plugins/"
          target="_blank"
        >
          <ButtonOutlined width="240px">View documentation</ButtonOutlined>
        </ExternalLink>
      </Flex>
    </Flex>
  );
}
