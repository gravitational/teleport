import React from 'react';
import { Link as InternalLink } from 'react-router-dom';

import {
  ButtonPrimary,
  ButtonBorder,
  Flex,
  Image,
  Link as ExternalLink,
  H1,
} from 'design';

import pluginsWheel from 'design/assets/images/icons/plugins.svg';

import { P } from 'design/Text/Text';

import cfg from 'e-teleport/config';

export function IntegrationsSplash() {
  return (
    <Flex flexDirection="column" gap="4" alignItems="center">
      <Image maxHeight="400px" src={pluginsWheel} />
      <Flex flexDirection="column" gap="2" maxWidth="540px">
        <H1 textAlign="center">Enroll your first integration to Teleport</H1>
        <P textAlign="center">
          Teleport integrations can connect your cluster to external apps for
          tasks such as alerting cluster administrators when team members make
          access requests.
        </P>
      </Flex>
      <Flex justifyContent="center" gap="2">
        <InternalLink to={cfg.oss.getIntegrationEnrollRoute()}>
          <ButtonPrimary width="240px">Enroll New Integration</ButtonPrimary>
        </InternalLink>
        <ExternalLink
          href="https://goteleport.com/docs/access-controls/access-request-plugins/"
          target="_blank"
        >
          <ButtonBorder width="240px">View documentation</ButtonBorder>
        </ExternalLink>
      </Flex>
    </Flex>
  );
}
