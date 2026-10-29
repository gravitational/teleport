/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import styled, { css } from 'styled-components';

import Box from 'design/Box';
import { CheckboxInput } from 'design/Checkbox';
import Flex from 'design/Flex';
import { Contract, Warning } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import Text from 'design/Text';
import { Theme } from 'design/theme';
import { HoverTooltip } from 'design/Tooltip';

import {
  BackgroundColorProps,
  getStatusBackgroundColor,
  StatusBackgroundColorProps,
} from '../shared/getBackgroundColor';
import { PinButton } from '../shared/PinButton';
import { ResourceCardBody } from './ResourceCardBody';
import { CardLayoutProps } from './types';

export function ScopedCardLayout({
  viewItem,
  setHovered,
  showingStatusInfo,
  visibleInputFields,
  showAllLabels,
  shouldDisplayStatusWarning,
  pinned,
  selected,
  innerContainer,
  onMouseLeave,
  selectResource,
  pinResource,
  pinningSupport,
  hovered,
  showResourceSelectedIcon,
  labelsInnerContainer,
  showMoreLabelsButton,
  onMoreLabelsClick,
  numMoreLabels,
  resourceLabelConfig,
  onLabelClick,
  onShowStatusInfo,
}: CardLayoutProps) {
  const { scope, primaryIconName, requiresRequest } = viewItem;

  return (
    <CardContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      showingStatusInfo={showingStatusInfo}
      showHoverState={visibleInputFields.hoverState}
    >
      <CardOuterContainer
        showAllLabels={showAllLabels}
        shouldDisplayWarning={shouldDisplayStatusWarning}
        showingStatusInfo={showingStatusInfo}
        showHoverState={visibleInputFields.hoverState}
        pinned={pinned}
        selected={selected}
      >
        <Flex ml={2} gap={1} alignItems="center">
          {scope && <Contract size="small" />}
          <Text
            // Make sure that this element keeps height of 1 line of text
            // whether there's a scope or not.
            css={`
              height: 1lh;
            `}
            typography="body4"
          >
            {scope}
          </Text>
        </Flex>
        <Flex height="100%">
          <Box
            flex="1"
            css={`
              position: relative;
            `}
            minWidth="0"
          >
            <CardInnerContainer
              showAllLabels={showAllLabels}
              showHoverState={visibleInputFields.hoverState}
              ref={innerContainer}
              p={3}
              alignItems="center"
              onMouseLeave={onMouseLeave}
              pinned={pinned}
              requiresRequest={requiresRequest}
              selected={selected}
              showingStatusInfo={showingStatusInfo}
              shouldDisplayWarning={shouldDisplayStatusWarning}
            >
              <Flex
                flexDirection="column"
                justifyContent="space-between"
                alignSelf="start"
                // Height needs to be fixed so that the pin button doesn't jump
                // around when labels are expanded.
                height="74px"
              >
                {visibleInputFields.checkbox && (
                  <Box alignSelf="start">
                    <CheckboxInput
                      checked={selected}
                      onChange={selectResource}
                    />
                  </Box>
                )}
                {visibleInputFields.pin && (
                  <Box alignSelf="end">
                    <PinButton
                      setPinned={pinResource}
                      pinned={pinned}
                      pinningSupport={pinningSupport}
                      hovered={hovered || selected}
                      scoped={!!scope}
                    />
                  </Box>
                )}
              </Flex>
              <ResourceIcon
                name={primaryIconName}
                width="45px"
                height="45px"
                alignSelf="center"
                ml={2}
                css={`
                  opacity: ${requiresRequest ? 0.5 : 1};
                `}
              />
              <ResourceCardBody
                viewItem={viewItem}
                visibleInputFields={visibleInputFields}
                hovered={hovered}
                showResourceSelectedIcon={showResourceSelectedIcon}
                showAllLabels={showAllLabels}
                labelsInnerContainer={labelsInnerContainer}
                shouldDisplayStatusWarning={shouldDisplayStatusWarning}
                showMoreLabelsButton={showMoreLabelsButton}
                onMoreLabelsClick={onMoreLabelsClick}
                numMoreLabels={numMoreLabels}
                resourceLabelConfig={resourceLabelConfig}
                onLabelClick={onLabelClick}
              />
            </CardInnerContainer>
          </Box>
          {shouldDisplayStatusWarning && (
            <HoverTooltip tipContent={'Show Connection Issue'} placement="left">
              <Flex
                alignSelf="stretch"
                ml="4px"
                mr="2px"
                onClick={onShowStatusInfo}
                css={`
                  cursor: pointer;
                `}
              >
                <Warning size="small" color="black" />
              </Flex>
            </HoverTooltip>
          )}
        </Flex>
      </CardOuterContainer>
    </CardContainer>
  );
}

const CardContainer = styled.li<{
  showingStatusInfo: boolean;
  showHoverState: boolean;
}>`
  height: 126px;
  list-style: none;
  position: relative;
`;

function getBorderColor({
  theme,
  pinned,
  selected,
  shouldDisplayWarning,
  showingStatusInfo,
}: {
  theme: Theme;
  pinned?: boolean;
  selected?: boolean;
  shouldDisplayWarning: boolean;
  showingStatusInfo: boolean;
}) {
  if (selected) {
    return theme.colors.interactive.tonal.primary[2];
  }
  if (showingStatusInfo) {
    return theme.colors.interactive.solid.alert.active;
  }
  if (shouldDisplayWarning) {
    return showingStatusInfo
      ? theme.colors.interactive.solid.alert.active
      : theme.colors.interactive.solid.alert.default;
  }
  return pinned
    ? theme.colors.interactive.tonal.primary[2]
    : theme.colors.interactive.tonal.neutral[0];
}

/**
 * The outer container's purpose is to reserve horizontal space on the resource
 * grid. It holds the inner container that normally holds a regular layout of
 * the card, and is fully contained inside the outer container.  Once the user
 * clicks the "more" button, the inner container "pops out" by changing its
 * position to absolute.
 *
 * The card height is fixed to allow the warning icon to stay in place when
 * user clicks on "showAllLabels". Without the fixed height, the container's
 * height shrinks when the inner container pops out, causing the icon to jump
 * up.
 */
const CardOuterContainer = styled(Box)<{
  showAllLabels?: boolean;
  shouldDisplayWarning: boolean;
  showingStatusInfo: boolean;
  showHoverState: boolean;
  pinned?: boolean;
  selected?: boolean;
}>`
  height: 126px;
  border-radius: ${({ theme }) => theme.radii[3]}px;
  transition: box-shadow 150ms;
  background-color: ${getBorderColor};
  background-clip: padding-box;
  border: ${({ theme }) => theme.borders[2]} ${getBorderColor};

  ${CardContainer}:hover & {
    ${({ showHoverState, showAllLabels, theme }) =>
      (showHoverState || showAllLabels) &&
      css`
        box-shadow: ${theme.boxShadow[3]};
      `};
    ${p =>
      p.shouldDisplayWarning &&
      p.showHoverState &&
      css`
        border-color: ${p.theme.colors.interactive.solid.alert.hover};
        background-color: ${p.theme.colors.interactive.solid.alert.hover};
      `}
  }
`;

function innerContainerBackground(props: BackgroundColorProps) {
  if (props.selected || props.pinned) {
    let topLayer = '';
    let bottomLayer = '';
    if (props.theme.type === 'light') {
      topLayer = props.selected
        ? props.theme.colors.interactive.tonal.primary[2]
        : props.theme.colors.interactive.tonal.primary[1];
      bottomLayer = props.theme.colors.levels.surface;
    } else {
      topLayer = props.selected
        ? props.theme.colors.interactive.tonal.primary[1]
        : props.theme.colors.interactive.tonal.primary[0];
      bottomLayer = props.theme.colors.levels.sunken;
    }
    return css`
      background:
        linear-gradient(0deg, ${topLayer} 0%, ${topLayer} 100%), ${bottomLayer};
    `;
  }
  return css`
    background: ${props.theme.colors.levels.sunken};
  `;
}

function innerContainerHoverBackground(props: BackgroundColorProps) {
  if (props.selected || props.pinned) {
    let topLayer = '';
    let bottomLayer = '';
    if (props.theme.type === 'light') {
      topLayer = props.selected
        ? props.theme.colors.interactive.tonal.primary[1]
        : props.theme.colors.interactive.tonal.primary[0];
      bottomLayer = props.theme.colors.levels.surface;
    } else {
      topLayer = props.selected
        ? props.theme.colors.interactive.tonal.primary[1]
        : props.theme.colors.interactive.tonal.primary[0];
      bottomLayer = props.theme.colors.levels.elevated;
    }
    return css`
      background:
        linear-gradient(0deg, ${topLayer} 0%, ${topLayer} 100%), ${bottomLayer};
    `;
  }
  return css`
    background: ${props.theme.colors.levels.elevated};
  `;
}

function innerContainerStatusBackground(props: StatusBackgroundColorProps) {
  const color = getStatusBackgroundColor(props);
  return css`
    background:
      linear-gradient(0deg, ${color} 0%, ${color} 100%),
      ${props.theme.colors.levels.sunken};
  `;
}

/**
 * The inner container that normally holds a regular layout of the card, and is
 * fully contained inside the outer container.  Once the user clicks the "more"
 * button, the inner container "pops out" by changing its position to absolute.
 */
const CardInnerContainer = styled(Flex)<
  BackgroundColorProps & { showHoverState: boolean; showAllLabels?: boolean }
>`
  border-radius: 6px;
  transition: background-color 150ms;
  ${innerContainerBackground};

  ${({ showAllLabels }) =>
    showAllLabels &&
    css`
      position: absolute;
      left: -2px;
      right: -2px;
      border: ${({ theme }) => theme.borders[2]} ${getBorderColor};
      border-top-width: 0;
      z-index: 1;
    `}

  &:hover {
    ${p =>
      p.shouldDisplayWarning &&
      p.showHoverState &&
      css`
        border-color: ${p.theme.colors.interactive.solid.alert.hover};
        ${innerContainerStatusBackground({
          showingStatusInfo: p.showingStatusInfo,
          theme: p.theme,
          action: 'hover',
          viewType: 'card',
        })};
      `}
  }

  ${({ showHoverState, showAllLabels, shouldDisplayWarning }) =>
    (showHoverState || showAllLabels) &&
    !shouldDisplayWarning &&
    css`
      ${CardContainer}:hover & {
        ${innerContainerHoverBackground};
      }
    `};
`;
