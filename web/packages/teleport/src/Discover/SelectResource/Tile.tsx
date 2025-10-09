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

import { useState, type ComponentPropsWithoutRef } from 'react';
import styled from 'styled-components';

import { Box, Flex, Link, Text } from 'design';
import { NewTab } from 'design/Icon';
import { Theme } from 'design/theme';
import { PinningSupport } from 'shared/components/UnifiedResources';
import { PinButton } from 'shared/components/UnifiedResources/shared/PinButton';

import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import {
  PermissionsErrorMessage,
  ResourceKind,
} from 'teleport/Discover/Shared';

import { getResourcePretitle } from '.';
import { DiscoverIcon } from './icons';
import { SelectResourceSpec } from './resources';

export type Size = 'regular' | 'large';

export function Tile({
  resourceSpec,
  size = 'regular',
  isPinned = false,
  onChangeShowApp,
  onSelectResource,
  onChangePin,
  pinningSupport,
}: {
  /**
   * if true, renders a larger tile with larger icon to
   * help differentiate pinned tiles from regular tiles.
   */
  size?: Size;
  isPinned: boolean;
  resourceSpec: SelectResourceSpec;
  onChangeShowApp(showApp: boolean): void;
  onSelectResource(selectedResourceSpec: SelectResourceSpec): void;
  pinningSupport: PinningSupport;
  onChangePin(guideId: string): void;
}) {
  const [pinHovered, setPinHovered] = useState(false);

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

  const wantLargeTile = size === 'large';

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
      tabIndex={0}
      title={title}
      data-testid={
        wantLargeTile
          ? `large-tile-${resourceSpec.kind}-${title}`
          : `regular-tile-${resourceSpec.kind}-${title}`
      }
      aria-label={`${pretitle} ${title}`}
      onMouseEnter={() => setPinHovered(true)}
      onMouseLeave={() => setPinHovered(false)}
      {...resourceCardProps}
    >
      <InnerCard
        hasAccess={resourceSpec.hasAccess}
        wantLargeTile={wantLargeTile}
      >
        {!resourceSpec.unguidedLink && resourceSpec.hasAccess && (
          <BadgeGuided>Guided</BadgeGuided>
        )}
        {!resourceSpec.hasAccess && (
          <ToolTipNoPermBadge>
            <PermissionsErrorMessage resource={resourceSpec} />
          </ToolTipNoPermBadge>
        )}
        <Flex alignItems="end" justifyContent="space-between">
          <Flex
            px={2}
            alignItems="center"
            height="48px"
            css={{ display: wantLargeTile ? 'block' : 'flex' }}
            gap={3}
          >
            <DiscoverIcon
              name={resourceSpec.icon}
              size={wantLargeTile ? 'large' : 'small'}
            />
            <Box mt={wantLargeTile ? 2 : 0}>
              {resourceSpec.unguidedLink ? (
                <Flex alignItems="center" gap={2}>
                  <StyledText wantLargeTile={wantLargeTile}>{title}</StyledText>
                  {resourceSpec.hasAccess && (
                    <NewTabIcon color="text.muted" size="small" />
                  )}
                </Flex>
              ) : (
                <StyledText wantLargeTile={wantLargeTile}>{title}</StyledText>
              )}
              {pretitle && (
                <Text typography="body3" color="text.slightlyMuted">
                  {pretitle}
                </Text>
              )}
            </Box>
          </Flex>
          <Box mb={1}>
            <PinButton
              className="pin-resource"
              hovered={pinHovered}
              pinned={isPinned}
              setPinned={() => onChangePin(resourceSpec.id)}
              pinningSupport={pinningSupport}
            />
          </Box>
        </Flex>
      </InnerCard>
    </ResourceCard>
  );
}

const NewTabIcon = styled(NewTab)`
  transition: color 0.3s;
`;

/**
 * ResourceCard cannot be a button, even though it's used like a button
 * since "PinButton.tsx" is rendered as its children. Otherwise it causes
 * an error where "button cannot be nested within a button".
 */
const ResourceCard = styled.div`
  position: relative;

  border-radius: ${props => props.theme.radii[3]}px;
  transition: all 150ms;

  &:hover {
    background-color: ${props => props.theme.colors.levels.surface};

    // We use a pseudo element for the shadow with position: absolute in order
    // to prevent the shadow from increasing the size of the layout and causing
    // scrollbar flicker.
    &:after {
      box-shadow: ${props => props.theme.boxShadow[3]};
      border-radius: ${props => props.theme.radii[3]}px;
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      z-index: -1;
      width: 100%;
      height: 100%;
    }
  }
`;

const InnerCard = styled.div<{ hasAccess?: boolean; wantLargeTile?: boolean }>`
  align-items: flex-start;
  display: inline-block;
  box-sizing: border-box;
  margin: 0;
  appearance: auto;
  text-align: left;

  height: ${p => (p.wantLargeTile ? '154px' : 'auto')};

  width: 100%;
  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => props.theme.radii[3]}px;
  background-color: ${props => getBackgroundColor(props)};

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
    // Make the border invisible instead of removing it,
    // this is to prevent things from shifting due to the size change.
    border: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);

    ${NewTabIcon} {
      color: ${props => props.theme.colors.text.slightlyMuted};
    }
  }
`;

export const getBackgroundColor = (props: {
  pinned?: boolean;
  theme: Theme;
}) => {
  if (props.pinned) {
    return props.theme.colors.interactive.tonal.primary[1];
  }
  return 'transparent';
};

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
  line-height: 18px;
`;

const StyledText = styled(Text)<{ wantLargeTile?: boolean }>`
  white-space: ${p => (p.wantLargeTile ? 'nowrap' : 'normal')};
  width: ${p => (p.wantLargeTile ? '155px' : 'auto')};
  font-weight: bold;
`;
