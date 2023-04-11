import React from 'react';
import { Link as InternalLink } from 'react-router-dom';
import { Box, Flex, Link as ExternalLink, Text } from 'design';
import * as Icons from 'design/Icon';
import {
  IntegrationTile,
  IntegrationTiles,
  NoCodeIntegrationDescription,
} from 'teleport/IntegrationEnroll';

import cfg from 'e-teleport/config';

import { PluginType, pluginTypes as allPluginTypes } from '../data';

import { PluginIcon } from '../PluginIcon';

export function IntegrationPick(props: Props) {
  const {
    availableTypes,
    existingTypes,
    hasPluginAccess,
    hasIntegrationAccess,
  } = props;

  // hosted plugins are adapted for hosting,
  // and which the auth server is set up for.
  let hosted = [];
  let selfHosted = [];
  if (hasPluginAccess) {
    hosted = allPluginTypes.filter(
      p => p.hosted && availableTypes.includes(p.type)
    );
    selfHosted = allPluginTypes.filter(p => !p.hosted);
  }

  // At least one resource will be enabled, so this screen will never be empty.
  // For plugins, at least the selfHosted should render, if no hosted plugins.
  return (
    <Flex flexDirection="column" gap={4}>
      <Flex flexDirection="column">
        <NoCodeIntegrationDescription />
        <Flex mb={2} gap={3}>
          {hosted.map(p => (
            <PluginTile
              pluginExists={existingTypes.includes(p.type)}
              key={p.type}
              type={p}
            />
          ))}
          {hasIntegrationAccess && <IntegrationTiles />}
        </Flex>
      </Flex>

      {selfHosted.length > 0 && (
        <Flex flexDirection="column">
          <Text fontWeight="bold" typography="h4">
            Self-Hosted Plugins
          </Text>
          <Text mb={3} typography="body1">
            There is a wide variety of plugins that you can source from the
            Teleport GitHub repository. See below for a sampling, or check out
            the base repo at{' '}
            <ExternalLink
              href="https://github.com/gravitational/teleport-plugins"
              target="_blank"
            >
              https://github.com/gravitational/teleport-plugins
            </ExternalLink>
            . Self-hosted plugins will not show up in your integration list, and
            must be managed outside of the Teleport UI.
          </Text>
          <Flex mb={2} gap={3}>
            {selfHosted.map(p => (
              <PluginTile key={p.type} type={p} />
            ))}
          </Flex>
        </Flex>
      )}
    </Flex>
  );
}

type Props = {
  availableTypes: string[];
  existingTypes: string[];
  hasPluginAccess: boolean;
  hasIntegrationAccess: boolean;
};

function PluginTile({
  pluginExists = false,
  type,
}: {
  pluginExists?: boolean;
  type: PluginType;
}) {
  const isClickable = !pluginExists && type.hosted;
  return (
    <IntegrationTile
      $exists={pluginExists}
      as={isClickable ? InternalLink : ExternalLink}
      to={isClickable ? cfg.oss.getIntegrationEnrollRoute(type.type) : null}
      href={isClickable ? null : type.url}
      target={isClickable ? null : '_blank'}
    >
      <PluginIcon my={3} type={type.type} />
      <Box mb={2} css={{ position: 'relative' }}>
        <Text>
          {type.name}
          {pluginExists && (
            <>
              {' '}
              <Icons.Check
                ml={1}
                color="success"
                css={`
                  position: absolute;
                  display: inline-flex;
                  align-items: center;
                  top: 0;
                  bottom: 0;
                `}
              />
            </>
          )}
        </Text>
      </Box>
    </IntegrationTile>
  );
}
