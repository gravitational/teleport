/*
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

import styled from 'styled-components';
import { border } from 'styled-system';
import { Flex, Text } from 'design';

/**
 * Header
 */
const FeatureHeader = styled(Flex)`
  flex-shrink: 0;
  // border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
  height: 56px;
  margin-left: -40px;
  margin-right: -40px;
  padding-left: 40px;
  padding-right: 40px;
  ${border}
`;

FeatureHeader.defaultProps = {
  alignItems: 'center',
  mb: 4,
};

/**
 * Header Title
 */
const FeatureHeaderTitle = styled(Text)`
  white-space: nowrap;
`;

FeatureHeaderTitle.defaultProps = {
  ...Text.defaultProps,
  typography: 'h3',
};

/**
 * Feature Box (container)
 */
const FeatureBox = styled(Flex)`
  width: 100%;
  height: 100%;
  flex-direction: column;
  /*
    This hack adds space to the bottom.
    Directly assigning padding-bottom does not work as flex container ignores this child padding.
    Directly assigning margin-bottom impacts the scrollbar area by pushing it up as well.
    It works in all major browsers.
  */
  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;

FeatureBox.defaultProps = {
  px: 6,
};

/**
 * Layout
 */
const AppVerticalSplit = styled.div`
  position: absolute;
  width: 100%;
  height: 100%;
  display: flex;
`;

const AppHorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
`;

const TabItem = styled.button`
  color: ${props => props.theme.colors.text.slightlyMuted};
  cursor: pointer;
  display: inline-flex;
  font-size: 14px;
  padding: 12px 40px;
  position: relative;
  text-decoration: none;
  font-weight: 500;

  &:hover {
    background: ${props =>
      props.active
        ? props.theme.colors.levels.surface
        : props.theme.colors.spotBackground[0]};
  }

  &.active {
    color: ${props => props.theme.colors.text.main};
  }

  &.active:after {
    background-color: ${props => props.theme.colors.brand};
    content: '';
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 4px;
  }
`;

export {
  AppHorizontalSplit,
  AppVerticalSplit,
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
  TabItem,
};
