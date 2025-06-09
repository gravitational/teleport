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

import { useEffect, useState } from 'react';
import styled, { css } from 'styled-components';

import { Box, ButtonIcon, Flex, Label, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { Tags, Warning } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';

import { makeLabelTag } from 'teleport/components/formatters';

import { CopyButton } from '../shared/CopyButton';
import {
  BackgroundColorProps,
  getBackgroundColor,
  getStatusBackgroundColor,
} from '../shared/getBackgroundColor';
import { PinButton } from '../shared/PinButton';
import { ResourceActionButtonWrapper } from '../shared/ResourceActionButton';
import { shouldWarnResourceStatus } from '../shared/StatusInfo';
import { ResourceItemProps } from '../types';

export function ResourceListItem({
  onLabelClick,
  pinningSupport,
  pinned,
  pinResource,
  selectResource,
  selected,
  expandAllLabels,
  onShowStatusInfo,
  showingStatusInfo,
  viewItem,
}: ResourceItemProps) {
  const {
    name,
    primaryIconName,
    SecondaryIcon,
    listViewProps,
    ActionButton,
    labels,
    requiresRequest = false,
    status,
  } = viewItem;
  const { description, resourceType, addr } = listViewProps;

  const [showLabels, setShowLabels] = useState(expandAllLabels);
  const [hovered, setHovered] = useState(false);

  // Update whether this item's labels are shown if the `expandAllLabels` preference is updated.
  useEffect(() => {
    setShowLabels(expandAllLabels);
  }, [expandAllLabels]);

  const showLabelsButton = labels.length > 0 && (hovered || showLabels);
  const shouldDisplayStatusWarning = shouldWarnResourceStatus(status);

  // Determines which column the resource type text should end at.
  // We do this because if there is no address, or the labels button
  // isn't showing, we want to let the resource type be able to extend
  // and use the free space that's left.
  const resourceTypeColumnEnd = () => {
    if (!addr) {
      if (!showLabelsButton) {
        return 'grid-column-end: labels-btn;';
      }
      return 'grid-column-end: address;';
    }
    return '';
  };

  return (
    <RowContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      shouldDisplayWarning={shouldDisplayStatusWarning}
      showingStatusInfo={showingStatusInfo}
    >
      <RowInnerContainer
        requiresRequest={requiresRequest}
        alignItems="start"
        pinned={pinned}
        selected={selected}
        shouldDisplayWarning={shouldDisplayStatusWarning}
        showingStatusInfo={showingStatusInfo}
      >
        {/* checkbox */}
        <HoverTooltip
          css={`
            grid-area: checkbox;
          `}
          tipContent={selected ? 'Deselect' : 'Select'}
        >
          <CheckboxInput checked={selected} onChange={selectResource} />
        </HoverTooltip>

        {/* pin button */}
        <PinButton
          setPinned={pinResource}
          pinned={pinned}
          pinningSupport={pinningSupport}
          hovered={hovered}
          css={`
            grid-area: pin;
            place-self: center center;
          `}
        />

        {/* icon */}
        <ResourceIcon
          name={primaryIconName}
          width="36px"
          height="36px"
          css={`
            grid-area: icon;
            place-self: center center;
            opacity: ${requiresRequest ? '0.5' : '1'};
          `}
        />

        {/* name and description */}
        <Flex
          css={`
            grid-area: name;
            justify-content: left;
            align-items: center;
            overflow: hidden;
          `}
        >
          <Flex
            flexDirection="column"
            css={`
              overflow: hidden;
            `}
          >
            <HoverTooltip tipContent={name} showOnlyOnOverflow>
              <Name>{name}</Name>
            </HoverTooltip>
            <HoverTooltip tipContent={description} showOnlyOnOverflow>
              <Description>{description}</Description>
            </HoverTooltip>
          </Flex>
          <Box
            css={`
              align-self: start;
            `}
          >
            {hovered && <CopyButton name={name} ml={1} />}
          </Box>
        </Flex>

        {/* type */}
        <Flex
          flexDirection="row"
          alignItems="center"
          css={`
            grid-area: type;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            // If there is no address and label button showing, let this column take up the extra space.
            ${resourceTypeColumnEnd()}
          `}
        >
          <ResTypeIconBox mr={1}>
            <SecondaryIcon size={18} />
          </ResTypeIconBox>
          <HoverTooltip
            tipContent={resourceType}
            css={`
              // Required for text-overflow: ellipsis to work. This is because a flex child won't shrink unless
              // its min-width is explicitly set.
              min-width: 0;
            `}
            showOnlyOnOverflow
          >
            <Text fontSize="14px" fontWeight={300} color="text.slightlyMuted">
              {resourceType}
            </Text>
          </HoverTooltip>
        </Flex>

        {/* address */}
        <HoverTooltip
          tipContent={addr}
          showOnlyOnOverflow
          css={`
            grid-area: address;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            // If the labels button isn't showing, let this column take up the extra space.
            ${!showLabelsButton ? 'grid-column-end: labels-btn;' : ''}
          `}
        >
          <Text fontSize="14px" fontWeight={300} color="text.muted">
            {addr}
          </Text>
        </HoverTooltip>

        {/* show labels button */}
        {showLabelsButton && (
          <HoverTooltip
            tipContent={showLabels ? 'Hide labels' : 'Show labels'}
            css={`
              grid-area: labels-btn;
            `}
          >
            <HoverIconButton
              size={1}
              onClick={() => setShowLabels(prevState => !prevState)}
              className={showLabels ? 'active' : ''}
            >
              <Tags size={18} color={showLabels ? 'text.main' : 'text.muted'} />
            </HoverIconButton>
          </HoverTooltip>
        )}

        {/* warning icon if status is unhealthy */}
        {shouldDisplayStatusWarning && (
          <HoverTooltip
            tipContent={'Show Connection Issue'}
            css={`
              grid-area: warning-icon;
              cursor: pointer;
            `}
          >
            <HoverIconButton size={1} onClick={onShowStatusInfo}>
              <Warning size={18} />
            </HoverIconButton>
          </HoverTooltip>
        )}

        {/* action button */}
        <Box
          css={`
            grid-area: button;
          `}
        >
          <ResourceActionButtonWrapper requiresRequest={requiresRequest}>
            {ActionButton}
          </ResourceActionButtonWrapper>
        </Box>

        {/* labels */}
        {showLabels && (
          <Box
            css={`
              grid-area: labels;
            `}
            ml={1}
            mb={2}
          >
            {labels.map((label, i) => {
              const labelText = makeLabelTag(label);
              // We can use the index i as the key since it will always be unique to this label.
              return (
                <Label
                  key={i}
                  title={labelText}
                  onClick={() => onLabelClick?.(label)}
                  kind="secondary"
                  mr={2}
                  css={`
                    cursor: pointer;
                    height: 20px;
                    line-height: 19px;
                    max-width: 100%;
                    overflow: hidden;
                    text-overflow: ellipsis;
                    white-space: nowrap;
                  `}
                >
                  {labelText}
                </Label>
              );
            })}
          </Box>
        )}
      </RowInnerContainer>
    </RowContainer>
  );
}

const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

const RowContainer = styled(Box)<{
  shouldDisplayWarning: boolean;
  showingStatusInfo: boolean;
}>`
  transition: all 150ms;
  position: relative;

  ${p =>
    p.shouldDisplayWarning &&
    css`
      background-color: ${getStatusBackgroundColor({
        showingStatusInfo: p.showingStatusInfo,
        theme: p.theme,
        action: '',
        viewType: 'list',
      })};
    `}

  &:hover {
    background-color: ${props => props.theme.colors.levels.surface};

    ${p =>
      p.shouldDisplayWarning &&
      css`
        background-color: ${getStatusBackgroundColor({
          showingStatusInfo: p.showingStatusInfo,
          theme: p.theme,
          action: 'hover',
          viewType: 'list',
        })};
      `}

    // We use a pseudo element for the shadow with position: absolute in order to prevent
    // the shadow from increasing the size of the layout and causing scrollbar flicker.
    &:after {
      box-shadow: ${props => props.theme.boxShadow[3]};
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

const RowInnerContainer = styled(Flex)<BackgroundColorProps>`
  display: grid;
  grid-template-columns: 22px 24px 36px 2fr 1fr 1fr 32px min-content;
  column-gap: ${props => props.theme.space[3]}px;
  grid-template-rows: 56px min-content;
  grid-template-areas:
    'checkbox pin icon name type address labels-btn warning-icon button'
    '. . labels labels labels labels labels labels labels';
  align-items: center;
  height: 100%;
  min-width: 100%;
  padding-right: ${props => props.theme.space[3]}px;
  padding-left: ${props => props.theme.space[3]}px;

  background-color: ${props => getBackgroundColor(props)};

  border-bottom: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};

  &:hover {
    // Make the border invisible instead of removing it, this is to prevent things from shifting due to the size change.
    border-bottom: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
  }
`;

const Name = styled(Text)`
  white-space: nowrap;
  line-height: 20px;
  font-weight: 300;
`;

const Description = styled(Text)`
  white-space: nowrap;
  font-size: 12px;
  color: ${props => props.theme.colors.text.muted};
`;

const HoverIconButton = styled(ButtonIcon)`
  .active {
    background: ${props => props.theme.colors.buttons.secondary.default};

    &:hover,
    &:focus {
      background: ${props => props.theme.colors.buttons.secondary.hover};
    }
    &:active {
      background: ${props => props.theme.colors.buttons.secondary.active};
    }
  }
`;
