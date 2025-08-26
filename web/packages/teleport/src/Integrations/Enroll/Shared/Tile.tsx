/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Link as ExternalLink, Flex, Label, Text } from 'design';
import { NewTab } from 'design/Icon';
import * as Icons from 'design/Icon';
import { ResourceIconName } from 'design/ResourceIcon/resourceIconSpecs';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

import { IntegrationIcon } from '../common';
import { type IntegrationTileSpec } from '../IntegrationTiles/integrations';
import {
  GenericNoPermBadge,
  renderExternalAuditStorageBadge,
} from '../IntegrationTiles/IntegrationTiles';
import { integrationTagOptions, type IntegrationTag } from './common';

type IntegrationLink = {
  url: string;
  external?: boolean;
  onClick?: () => void;
};

export function IntegrationTileWithSpec({
  spec,
  hasIntegrationAccess = true,
  hasExternalAuditStorage = true,
}: {
  spec: IntegrationTileSpec;
  hasIntegrationAccess?: boolean;
  hasExternalAuditStorage?: boolean;
}) {
  const link = hasExternalAuditStorage
    ? { external: false, url: cfg.getIntegrationEnrollRoute(spec.kind) }
    : null;

  let renderBadge = null;
  const dataTestID = `tile-${spec.kind}`;

  if (spec.kind === IntegrationKind.ExternalAuditStorage) {
    const externalAuditStorageEnabled =
      cfg.entitlements.ExternalAuditStorage.enabled;

    hasIntegrationAccess =
      hasIntegrationAccess &&
      hasExternalAuditStorage &&
      externalAuditStorageEnabled;

    renderBadge = () => {
      return renderExternalAuditStorageBadge(
        hasExternalAuditStorage,
        externalAuditStorageEnabled
      );
    };
  } else if (!hasIntegrationAccess) {
    renderBadge = () => {
      return <GenericNoPermBadge noAccess={!hasIntegrationAccess} />;
    };
  }

  return (
    <Tile
      title={spec.name}
      hasAccess={hasIntegrationAccess}
      link={link}
      tags={spec.tags}
      icon={spec.icon}
      description={spec.description}
      renderBadge={renderBadge}
      data-testid={dataTestID}
    />
  );
}

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
          <Flex width={72}>
            <IntegrationIcon name={icon} size={72} />
          </Flex>
          <Flex flexDirection="column" gap={1} minWidth={0}>
            <StyledText>{title}</StyledText>
            {description && (
              <Text typography="body2" color="text.slightlyMuted" fontSize={12}>
                {description}
              </Text>
            )}
            <Flex gap={1} flexWrap="wrap">
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

const NewTabIcon = styled(NewTab)`
  transition: color 0.3s;
`;

const StyledText = styled(Text)`
  white-space: nowrap;
  font-size: 16px;
  font-weight: 500;
`;

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
