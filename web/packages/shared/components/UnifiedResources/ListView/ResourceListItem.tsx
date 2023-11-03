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

import React, { useState, useLayoutEffect, useRef } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, Label, Text } from 'design';
import { StyledCheckbox } from 'design/Checkbox';
import { Tags } from 'design/Icon';

import { ResourceIcon } from 'design/ResourceIcon';

import { HoverTooltip } from '../UnifiedResources';

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
  const [isNameOverflowed, setIsNameOverflowed] = useState(false);
  const [isDescOverflowed, setIsDescOverflowed] = useState(false);
  const [showLabels, setShowLabels] = useState(false);

  const [hovered, setHovered] = useState(false);

  const innerContainer = useRef<Element | null>(null);
  const nameText = useRef<HTMLDivElement | null>(null);
  const descText = useRef<HTMLDivElement | null>(null);

  useLayoutEffect(() => {
    const observer = new ResizeObserver(() => {
      // This check will let us know if the name or description text has overflowed. We do this
      // to conditionally render a tooltip for only overflowed names and descriptions.
      if (
        nameText.current?.scrollWidth >
        nameText.current?.parentElement.offsetWidth
      ) {
        setIsNameOverflowed(true);
      } else {
        setIsNameOverflowed(false);
      }
      if (
        descText.current?.scrollWidth >
        descText.current?.parentElement.offsetWidth
      ) {
        setIsDescOverflowed(true);
      } else {
        setIsDescOverflowed(false);
      }
    });

    observer.observe(innerContainer.current);
    return () => {
      observer.disconnect();
    };
  });

  return (
    <RowContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <RowInnerContainer
        ref={innerContainer}
        alignItems="start"
        pinned={pinned}
        selected={selected}
      >
        {/* checkbox */}
        <Box
          css={`
            grid-area: checkbox;
            place-self: center;
          `}
        >
          <HoverTooltip tipContent={<>{selected ? 'Deselect' : 'Select'}</>}>
            <StyledCheckbox checked={selected} onChange={selectResource} />
          </HoverTooltip>
        </Box>

        {/* pin button */}
        <Box
          css={`
            grid-area: pin;
            place-self: center;
          `}
        >
          <PinButton
            setPinned={pinResource}
            pinned={pinned}
            pinningSupport={pinningSupport}
            hovered={hovered}
            css={`
              display: flex;
            `}
          />
        </Box>

        {/* icon */}
        <ResourceIcon
          name={primaryIconName}
          width="36px"
          height="36px"
          css={`
            grid-area: icon;
            place-self: center;
          `}
        />

        {/* name */}
        <Box
          css={`
            grid-area: name;
            display: flex;
          `}
        >
          <Flex flexDirection="column">
            <Flex>
              {isNameOverflowed ? (
                <HoverTooltip tipContent={<>{name}</>}>
                  <Text
                    ref={nameText}
                    typography="h5"
                    fontWeight={300}
                    css={`
                      max-width: 13vw;
                      text-overflow: ellipsis;
                      white-space: nowrap;
                    `}
                  >
                    {name}
                  </Text>
                </HoverTooltip>
              ) : (
                <Text
                  ref={nameText}
                  typography="h5"
                  fontWeight={300}
                  css={`
                    max-width: 13vw;
                    text-overflow: ellipsis;
                    white-space: nowrap;
                  `}
                >
                  {name}
                </Text>
              )}
              {hovered && <CopyButton name={name} />}
            </Flex>
            {description && (
              <>
                {isDescOverflowed ? (
                  <HoverTooltip tipContent={<>{description}</>}>
                    <Text
                      ref={descText}
                      typography="subtitle1"
                      color="text.muted"
                      css={`
                        max-width: 15vw;
                        text-overflow: ellipsis;
                        white-space: nowrap;
                      `}
                    >
                      {description}
                    </Text>
                  </HoverTooltip>
                ) : (
                  <Text
                    ref={descText}
                    typography="subtitle1"
                    color="text.muted"
                    css={`
                      max-width: 15vw;
                      text-overflow: ellipsis;
                      white-space: nowrap;
                    `}
                  >
                    {description}
                  </Text>
                )}
              </>
            )}
          </Flex>
        </Box>

        {/* type */}
        <Flex flexDirection="row" alignItems="center">
          <ResTypeIconBox>
            <SecondaryIcon size={18} />
          </ResTypeIconBox>
          {type && (
            <Box ml={1} title={type}>
              <Text typography="h2" fontSize={16} color="text.slightlyMuted">
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
        >
          {addr && (
            <Box ml={2} title={addr}>
              <Text typography="body1" color="text.muted">
                {addr}
              </Text>
            </Box>
          )}
        </Box>

        {/* show labels button */}
        <ShowLabelsBtnContainer>
          <ButtonIcon
            size={1}
            onClick={() => setShowLabels(!showLabels)}
            className={showLabels ? 'active' : ''}
          >
            <Tags size={18} color={showLabels ? 'text.main' : 'text.muted'} />
          </ButtonIcon>
        </ShowLabelsBtnContainer>

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
            ml={2}
          >
            {labels.map((label, i) => {
              const { name, value } = label;
              const labelText = `${name}: ${value}`;
              return (
                <Label
                  key={JSON.stringify([name, value, i])}
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
  width: 100%;
  transition: all 150ms;

  :hover {
    background-color: ${props => props.theme.colors.levels.surface};
    box-shadow: ${props => props.theme.boxShadow[3]};
  }
`;

const RowInnerContainer = styled(Flex)`
  display: grid;
  grid-template-columns: 42px 36px 56px 16vw 16vw auto 64px 90px;
  grid-template-rows: 56px min-content;
  grid-template-areas:
    'checkbox pin icon name type address labels-btn button'
    '. . labels labels labels labels labels labels';
  align-items: center;
  height: 100%;
  min-width: 100%;
  padding-right: 16px;
  padding-top: 8px;
  padding-bottom: 8px;

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

const ShowLabelsBtnContainer = styled(Box)`
  grid-area: labels-btn;
  place-self: center end;
  margin-right: 32px;

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
