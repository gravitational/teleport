import React from 'react';
import { Link } from 'react-router-dom';
import { Box, ButtonPrimary, ButtonSecondary, Flex, H2, Image } from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';

import cfg from 'e-teleport/config';

import { CloudHostablePlugin } from './plugins';

import { OAuthPluginRegistered } from './PluginEnroll';

export function PluginEnrollSuccess(props: State) {
  const { oauthSuccessData, plugin } = props;

  return (
    <Flex flexDirection="column" alignItems="center" mt="6">
      <Image src={pamSuccess} maxWidth="120px" />
      <H2 my="2">{plugin.name} is integrated successfully</H2>
      <Box maxWidth="500px" textAlign="center">
        {plugin.cloudHostable && plugin.NextSteps && (
          <plugin.NextSteps successData={oauthSuccessData} />
        )}
      </Box>

      <Flex gap="2" my="3">
        <Link to={cfg.oss.routes.integrations}>
          <ButtonPrimary>Go to Integration List</ButtonPrimary>
        </Link>
        <Link to={cfg.oss.getIntegrationEnrollRoute(null)}>
          <ButtonSecondary>Add Another Integration</ButtonSecondary>
        </Link>
      </Flex>
    </Flex>
  );
}

type State = {
  plugin: CloudHostablePlugin;
  oauthSuccessData?: OAuthPluginRegistered;
};
