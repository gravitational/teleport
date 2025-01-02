import { Link } from 'react-router-dom';

import { Box, ButtonPrimary, ButtonSecondary, Flex, H2, Image } from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';

import cfg from 'e-teleport/config';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';

import { OAuthPluginRegistered } from './PluginEnroll';

export function PluginEnrollSuccess(props: State) {
  const { oauthSuccessData, plugin, installedPluginName } = props;

  let primaryButtonUrl = cfg.oss.routes.integrations;
  let primaryButtonText = 'Go to Integration List';

  if (props.primaryButtonUrl) {
    primaryButtonUrl = props.primaryButtonUrl;
  } else if (plugin.type === 'okta') {
    primaryButtonUrl = cfg.oss.getIntegrationStatusRoute(
      plugin.type,
      installedPluginName
    );
  }

  if (props.primaryButtonText) {
    primaryButtonText = props.primaryButtonText;
  } else if (plugin.type === 'okta') {
    primaryButtonText = 'Go to Okta Status Page';
  }

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
        <Link to={primaryButtonUrl}>
          <ButtonPrimary>{primaryButtonText}</ButtonPrimary>
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
  installedPluginName: string;
  oauthSuccessData?: OAuthPluginRegistered;
  primaryButtonText?: string | null;
  primaryButtonUrl?: string | null;
};
