import React from 'react';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Flex, Link as ExternalLink, Text } from 'design';
import * as Icons from 'design/Icon';

import cfg from 'e-teleport/config';

import { PluginType, pluginTypes as allPluginTypes } from '../data';

import { PluginIcon } from '../PluginIcon';

export function IntegrationPick(props: Props) {
  const { availableTypes, existingTypes } = props;

  // Plugins that are adapted for hosting,
  // and which the auth server is set up for.
  const hosted = allPluginTypes.filter(
    p => p.hosted && availableTypes.includes(p.type)
  );

  // Plugins that can only be self-hosted.
  const selfHosted = allPluginTypes.filter(p => !p.hosted);

  return (
    <Flex flexDirection="column" gap={4}>
      {hosted.length > 0 && (
        <Flex flexDirection="column">
          <Text fontWeight="bold" typography="h4">
            No-Code Integrations
          </Text>
          <Text mb={2} typography="body1">
            Hosted Integrations eliminate the setup work so you can quickly
            connect applications to Teleport for alerting and other useful
            functions. This list is short for now, but it will grow with time!
          </Text>
          <Flex my={2} gap={3}>
            {hosted.map(p => (
              <PluginTile
                disabled={existingTypes.includes(p.type)}
                key={p.type}
                type={p}
              />
            ))}
          </Flex>
        </Flex>
      )}

      {selfHosted.length > 0 && (
        <Flex flexDirection="column">
          <Text fontWeight="bold" typography="h4">
            Self-Hosted Plugins
          </Text>
          <Text mb={2} typography="body1">
            There is a wide variety of plugins that you can source from the
            Teleport GitHub repository. See below for a sampling, or check out
            the base repo at{' '}
            <ExternalLink href="https://github.com/gravitational/teleport-plugins">
              https://github.com/gravitational/teleport-plugins
            </ExternalLink>
            . Self-hosted plugins will not show up in your integration list, and
            must be managed outside of the Teleport UI.
          </Text>
          <Flex my={2} gap={3}>
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
};

function PluginTile({
  disabled = false,
  type,
}: {
  disabled?: boolean;
  type: PluginType;
}) {
  const tile = (
    <Tile disabled={disabled}>
      <PluginIcon my={3} type={type.type} />
      <Box mb={2} css={{ position: 'relative' }}>
        {/* Compensate for icon width to keep the text itself centered */}
        <Text>
          {type.name}
          {disabled && (
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
    </Tile>
  );

  if (disabled) {
    return tile;
  }

  if (type.hosted) {
    return (
      <TileInternalLink to={cfg.oss.getIntegrationEnrollRoute(type.type)}>
        {tile}
      </TileInternalLink>
    );
  } else {
    return <TileExternalLink href={type.url}>{tile}</TileExternalLink>;
  }
}

const Tile = styled(Flex)`
  flex-direction: column;
  align-items: center;
  border-radius: 4px;
  height: 144px;
  width: 144px;
  background-color: ${({ theme }) => theme.colors.buttons.secondary.default};

  ${({ theme, disabled }) =>
    !disabled &&
    `
  &:hover {
    background-color: ${theme.colors.buttons.secondary.hover};
  }
  `}
`;

const linkStyles = `
  color: inherit;
  text-decoration: none;
`;
const TileInternalLink = styled(InternalLink)`
  ${linkStyles}
`;

const TileExternalLink = styled(ExternalLink)`
  ${linkStyles}
`;
