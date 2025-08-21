/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { ReactNode } from 'react';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import {
  Box,
  Link as ExternalLink,
  Flex,
  H2,
  Label,
  ResourceIcon,
  Text,
} from 'design';
import { NewTab } from 'design/Icon';
import * as Icons from 'design/Icon';
import { ResourceIconName } from 'design/ResourceIcon/resourceIconSpecs';

import {
  integrationTagOptions,
  type IntegrationTag,
} from './IntegrationTiles/integrations';

export const IntegrationTile = styled(Flex)<{
  disabled?: boolean;
  $exists?: boolean;
}>`
  color: inherit;
  text-decoration: none;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  border-radius: ${({ theme }) => theme.radii[2]}px;
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[3]}px;
  height: 170px;
  width: 170px;
  background-color: ${({ theme }) => theme.colors.levels.sunken};
  text-align: center;
  cursor: ${({ disabled, $exists }) =>
    disabled || $exists ? 'not-allowed' : 'pointer'};
  transition: background-color 200ms ease;

  ${props => {
    if (props.$exists) {
      return;
    }

    return `
    opacity: ${props.disabled ? '0.45' : '1'};
    &:hover,
    &:focus-visible {
      background-color: ${props.theme.colors.levels.surface};
    }
    `;
  }};
`;

const NewTabIcon = styled(NewTab)`
  transition: color 0.3s;
`;

type IntegrationLink = {
  url: string;
  external?: boolean;
  onClick?: () => void;
};

export function Tile({
  title,
  description,
  hasAccess,
  icon,
  link,
  tags = [],
  enrolled,
  renderBadge,
  'data-testid': dataTestID,
}: {
  title: string;
  description?: ReactNode;
  hasAccess: boolean;
  enrolled?: boolean;
  icon: ResourceIconName;
  link?: IntegrationLink;
  tags?: IntegrationTag[];
  renderBadge?: () => ReactNode;
  'data-testid'?: string;
}) {
  const nameForTag = (tag: IntegrationTag) => {
    const option = integrationTagOptions.find(option => option.value === tag);
    return option ? option.label : null;
  };

  let tileProps = {};
  if (link && hasAccess) {
    if (link.external) {
      tileProps = {
        as: ExternalLink,
        href: link.url,
        target: '_blank',
        onClick: link.onClick,
        style: { textDecoration: 'none' },
        role: 'link',
      };
    } else {
      tileProps = {
        as: InternalLink,
        to: link.url,
      };
    }
  }

  if (dataTestID) {
    tileProps['data-testid'] = dataTestID;
  }

  return (
    <IntegrationCard
      tabIndex={0}
      title={title}
      data-testid={dataTestID}
      disabled={!hasAccess}
      {...tileProps}
    >
      {renderBadge?.() ??
        (link && !link.external ? (
          <BadgeGuided>Guided</BadgeGuided>
        ) : (
          <BadgeSelfHosted>
            Self-Hosted
            <NewTabIcon size={14} ml={1} />
          </BadgeSelfHosted>
        ))}
      <Flex flexDirection="row" width={'100%'}>
        <Flex alignItems="flex-start" ml={3} mr={2} mt={3} gap={3}>
          <ResourceIcon name={icon} />
          <Flex flexDirection="column" gap={1} maxWidth={340}>
            <StyledText>{title}</StyledText>
            {description && (
              <Text typography="body2" color="text.slightlyMuted">
                {description}
              </Text>
            )}
            <Flex gap={1}>
              {tags.map(tag => {
                return (
                  <StyledLabel
                    key={tag}
                    title={nameForTag(tag)}
                    kind="secondary"
                    data-is-label=""
                  >
                    {nameForTag(tag)}
                  </StyledLabel>
                );
              })}
            </Flex>
          </Flex>
        </Flex>
      </Flex>
      {enrolled && (
        <StyledIconCheck
          data-testid="integration-checkmark"
          color="success.main"
          size="large"
        />
      )}
    </IntegrationCard>
  );
}

const StyledLabel = styled(Label)`
  height: 20px;
  margin: 1px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  line-height: 18px;
`;

const IntegrationCard = styled.div<{ disabled?: boolean }>`
  align-items: flex-start;
  display: inline-flex;
  margin: 0;
  appearance: auto;
  text-align: left;
  position: relative;
  text-decoration: none;

  height: 132px;
  width: 100%;
  border-radius: ${props => props.theme.radii[3]}px;
  box-shadow: inset 0 0 0 2px ${props => props.theme.colors.interactive.tonal.neutral[0]};
  background-color: transparent;
  transition: background-color 0.3s ease;
  
  color: ${props => props.theme.colors.text.main};
  line-height: inherit;
  font-size: inherit;
  font-family: inherit;

  cursor: ${({ disabled }) => (disabled ? 'not-allowed' : 'pointer')};
  
  opacity: ${props => (props.disabled ? '0.45' : '1')};
 
  &:hover {
    background-color: ${props =>
      props.disabled
        ? 'transparent'
        : props.theme.colors.interactive.tonal.neutral[0]};
    box-shadow: inset 0 0 0 2px transparent;
  }

  &:focus-visible {
    outline: none;
    box-shadow: 0 0 0 3px ${props => props.theme.colors.brand};
  }

  &:hover,
  &:focus-visible {
    ${NewTabIcon} {
    color: ${props => props.theme.colors.text.slightlyMuted};
    }    
`;

/**
 * IntegrationIcon wraps ResourceIcon with css required for integration
 * and plugin tiles.
 */
export const IntegrationIcon = styled(ResourceIcon).withConfig({
  shouldForwardProp: prop => prop !== 'size',
})<{ size?: number }>`
  margin: 0 auto;
  height: 100%;
  min-width: 0;
  max-width: ${props => props.width || 72}px;
`;

const StyledText = styled(Text)`
  white-space: nowrap;
  font-size: 18px;
  font-weight: 500;
`;

const BadgeSelfHosted = styled.div`
  position: absolute;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  color: ${props => props.theme.colors.text.slightlyMuted};
  padding: 2px 8px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
  line-height: 20px;
`;

const BadgeGuided = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 2px 8px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
  line-height: 18px;
`;

const StyledIconCheck = styled(Icons.Check)`
  position: absolute;
  bottom: ${({ theme }) => theme.space[2]}px;
  right: ${({ theme }) => theme.space[2]}px;
`;

export const NoCodeIntegrationDescription = () => (
  <Box mb={3}>
    <H2 mb={1}>No-Code Integrations</H2>
    <Text>
      Set up Teleport to post notifications to messaging apps, discover and
      import resources from cloud providers and other services.
    </Text>
  </Box>
);
