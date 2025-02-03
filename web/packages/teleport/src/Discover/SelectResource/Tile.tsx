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

import { type ComponentPropsWithoutRef } from 'react';
import styled from 'styled-components';

import { Box, Flex, Link, Text } from 'design';
import { NewTab } from 'design/Icon';

import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import {
  PermissionsErrorMessage,
  ResourceKind,
} from 'teleport/Discover/Shared';

import { getResourcePretitle } from '.';
import { DiscoverIcon } from './icons';
import { type ResourceSpec } from './types';

export function Tile({
  resourceSpec,
  onChangeShowApp,
  onSelectResource,
}: {
  resourceSpec: ResourceSpec;
  onChangeShowApp(b: boolean): void;
  onSelectResource(r: ResourceSpec): void;
}) {
  const title = resourceSpec.name;
  const pretitle = getResourcePretitle(resourceSpec);
  const select = () => {
    if (!resourceSpec.hasAccess) {
      return;
    }

    onChangeShowApp(true);
    onSelectResource(resourceSpec);
  };

  let resourceCardProps: ComponentPropsWithoutRef<'button' | typeof Link>;

  if (resourceSpec.kind === ResourceKind.Application && resourceSpec.isDialog) {
    resourceCardProps = {
      onClick: select,
      onKeyUp: (e: KeyboardEvent) => e.key === 'Enter' && select(),
      role: 'button',
    };
  } else if (resourceSpec.unguidedLink) {
    resourceCardProps = {
      as: Link,
      href: resourceSpec.hasAccess ? resourceSpec.unguidedLink : null,
      target: '_blank',
      style: { textDecoration: 'none' },
      role: 'link',
    };
  } else {
    resourceCardProps = {
      onClick: () => resourceSpec.hasAccess && onSelectResource(resourceSpec),
      onKeyUp: (e: KeyboardEvent) => {
        if (e.key === 'Enter' && resourceSpec.hasAccess) {
          onSelectResource(resourceSpec);
        }
      },
      role: 'button',
    };
  }

  // There can be three types of click behavior with the resource cards:
  //  1) If the resource has no interactive UI flow ("unguided"),
  //     clicking on the card will take a user to our docs page
  //     on a new tab.
  //  2) If the resource is guided, we start the "flow" by
  //     taking user to the next step.
  //  3) If the resource is kind 'Application', it will render the legacy
  //     popup modal where it shows user to add app manually or automatically.
  return (
    <ResourceCard
      data-testid={resourceSpec.kind}
      hasAccess={resourceSpec.hasAccess}
      aria-label={`${pretitle} ${title}`}
      {...resourceCardProps}
    >
      {!resourceSpec.unguidedLink && resourceSpec.hasAccess && (
        <BadgeGuided>Guided</BadgeGuided>
      )}
      {!resourceSpec.hasAccess && (
        <ToolTipNoPermBadge>
          <PermissionsErrorMessage resource={resourceSpec} />
        </ToolTipNoPermBadge>
      )}
      <Flex px={2} alignItems="center" height="48px">
        <Flex mr={3} justifyContent="center" width="24px">
          <DiscoverIcon name={resourceSpec.icon} />
        </Flex>
        <Box>
          {pretitle && (
            <Text typography="body3" color="text.slightlyMuted">
              {pretitle}
            </Text>
          )}
          {resourceSpec.unguidedLink ? (
            <Text bold color="text.main">
              {title}
            </Text>
          ) : (
            <Text bold>{title}</Text>
          )}
        </Box>
      </Flex>

      {resourceSpec.unguidedLink && resourceSpec.hasAccess ? (
        <NewTabInCorner color="text.muted" size={18} />
      ) : null}
    </ResourceCard>
  );
}

const NewTabInCorner = styled(NewTab)`
  position: absolute;
  top: ${props => props.theme.space[3]}px;
  right: ${props => props.theme.space[3]}px;
  transition: color 0.3s;
`;

const ResourceCard = styled.button<{ hasAccess?: boolean }>`
  position: relative;
  text-align: left;
  background: ${props => props.theme.colors.spotBackground[0]};
  transition: all 0.3s;

  border: none;
  border-radius: 8px;
  padding: 12px;
  color: ${props => props.theme.colors.text.main};
  line-height: inherit;
  font-size: inherit;
  font-family: inherit;
  cursor: pointer;

  opacity: ${props => (props.hasAccess ? '1' : '0.45')};

  &:focus-visible {
    outline: none;
    box-shadow: 0 0 0 3px ${props => props.theme.colors.brand};
  }

  &:hover,
  &:focus-visible {
    background: ${props => props.theme.colors.spotBackground[1]};

    ${NewTabInCorner} {
      color: ${props => props.theme.colors.text.slightlyMuted};
    }
  }
`;

const BadgeGuided = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 0px 6px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
  line-height: 24px;
`;
