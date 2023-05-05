/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import styled from 'styled-components';
import { Flex, Text } from 'design';
import defaultTheme from 'design/theme';

/**
 * Header
 */
const FeatureHeader = styled(Flex)`
  flex-shrink: 0;
  border-bottom: 1px solid
    ${props => props.theme.colors.levels.surfaceSecondary};
  height: 56px;
  margin-left: -40px;
  margin-right: -40px;
  padding-left: 40px;
  padding-right: 40px;
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
  theme: defaultTheme,
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
        : 'rgba(255, 255, 255, .06)'};
  }

  &.active {
    color: ${props => props.theme.colors.light};
  }

  &.active:after {
    background-color: ${props => props.theme.colors.brandAccent};
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
