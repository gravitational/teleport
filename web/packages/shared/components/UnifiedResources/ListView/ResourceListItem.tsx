/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, Label, Text } from 'design';
import { StyledCheckbox } from 'design/Checkbox';
import { Tags } from 'design/Icon';

import { ResourceIcon } from 'design/ResourceIcon';

import { HoverTooltip } from 'shared/components/ToolTip';

import { ResourceItemProps } from '../types';
import { PinButton, CopyButton } from '../shared';

export function ResourceListItem({
  name,
  primaryIconName,
  SecondaryIcon,
  onLabelClick,
  description,
  addr,
  type,
  ActionButton,
  labels,
  pinningSupport,
  pinned,
  pinResource,
  selectResource,
  selected,
}: ResourceItemProps) {
  const [showLabels, setShowLabels] = useState(false);
  const [hovered, setHovered] = useState(false);

  return (
    <RowContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <RowInnerContainer alignItems="start" pinned={pinned} selected={selected}>
        {/* checkbox */}
        <HoverTooltip
          css={`
            grid-area: checkbox;
          `}
          tipContent={selected ? 'Deselect' : 'Select'}
        >
          <StyledCheckbox checked={selected} onChange={selectResource} />
        </HoverTooltip>

        {/* pin button */}
        {/* We wrap it in a box in order to center it properly in its grid. */}
        <Box
          css={`
            grid-area: pin;
            place-self: center center;
          `}
        >
          <PinButton
            setPinned={pinResource}
            pinned={pinned}
            pinningSupport={pinningSupport}
            hovered={hovered}
          />
        </Box>

        {/* icon */}
        <ResourceIcon
          name={primaryIconName}
          width="36px"
          height="36px"
          css={`
            grid-area: icon;
            place-self: center center;
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
            {hovered && <CopyButton name={name} />}
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
          `}
        >
          <ResTypeIconBox>
            <SecondaryIcon size={18} />
          </ResTypeIconBox>
          {type && (
            <Box ml={1} title={type}>
              <Text fontSize="14px" fontWeight={300} color="text.slightlyMuted">
                {type}
              </Text>
            </Box>
          )}
        </Flex>

        {/* address */}
        <Box
          css={`
            grid-area: address;
            overflow: hidden;
            text-overflow: ellipsis;
          `}
          ml={2}
          title={addr}
        >
          <Text fontSize="14px" fontWeight={300} color="text.muted">
            {addr}
          </Text>
        </Box>

        {/* show labels button */}
        {labels.length > 0 && (
          <HoverTooltip
            tipContent={showLabels ? 'Hide Labels' : 'Show Labels'}
            css={`
              grid-area: labels-btn;
            `}
          >
            <ShowLabelsButton
              size={1}
              onClick={() => setShowLabels(!showLabels)}
              className={showLabels ? 'active' : ''}
            >
              <Tags size={18} color={showLabels ? 'text.main' : 'text.muted'} />
            </ShowLabelsButton>
          </HoverTooltip>
        )}

        {/* action button */}
        <Box
          css={`
            grid-area: button;
          `}
        >
          {ActionButton}
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
              const { name, value } = label;
              const labelText = `${name}: ${value}`;
              // We can use the index i as the key since it will always be unique to this label.
              return (
                <Label
                  key={i}
                  title={labelText}
                  onClick={() => onLabelClick?.(label)}
                  kind="secondary"
                  data-is-label=""
                  mr={2}
                  css={`
                    cursor: pointer;
                    height: 20px;
                    line-height: 19px;
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

const RowContainer = styled(Box)`
  transition: all 150ms;
  position: relative;

  :hover {
    background-color: ${props => props.theme.colors.levels.surface};

    // We use a pseudo element for the shadow with position: absolute in order to prevent
    // the shadow from increasing the size of the layout and causing scrollbar flicker.
    :after {
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

const RowInnerContainer = styled(Flex)`
  display: grid;
  grid-template-columns: 22px 24px 36px 2fr 1fr 1fr 32px 90px;
  column-gap: ${props => props.theme.space[3]}px;
  grid-template-rows: 56px min-content;
  grid-template-areas:
    'checkbox pin icon name type address labels-btn button'
    '. . labels labels labels labels labels labels';
  align-items: center;
  height: 100%;
  min-width: 100%;
  padding-right: ${props => props.theme.space[3]}px;
  padding-left: ${props => props.theme.space[3]}px;

  background-color: ${props => getBackgroundColor(props)};

  border-bottom: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};

  :hover {
    // Make the border invisible instead of removing it, this is to prevent things from shifting due to the size change.
    border-bottom: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
  }
`;

const getBackgroundColor = props => {
  if (props.selected) {
    return props.theme.colors.interactive.tonal.primary[2];
  }
  if (props.pinned) {
    return props.theme.colors.interactive.tonal.primary[0];
  }
  return 'transparent';
};

const Name = styled(Text)`
  height: 20px;
  white-space: nowrap;
  line-height: 20px;
  max-width: 100%;
  font-size: 14px;
  font-weight: 300;
`;

const Description = styled(Text)`
  max-height: 20px;
  white-space: nowrap;
  font-size: 12px;
  color: ${props => props.theme.colors.text.muted};
`;

const ShowLabelsButton = styled(ButtonIcon)`
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
