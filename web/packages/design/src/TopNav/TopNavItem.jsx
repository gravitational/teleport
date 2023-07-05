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

import { space, width, maxWidth, height, maxHeight } from 'design/system';

/**
 * TopNavItem
 */
const TopNavItem = styled.button`
  align-items: center;
  background: none;
  border: none;
  color: ${props =>
    props.active ? props.theme.colors.light : 'rgba(255, 255, 255, .56)'};
  cursor: pointer;
  display: inline-flex;
  font-size: 11px;
  font-weight: 600;
  height: 100%;
  margin: 0;
  outline: none;
  padding: 0 16px;
  position: relative;
  text-decoration: none;

  &:hover {
    background: ${props =>
      props.active
        ? props.theme.colors.levels.surface
        : 'rgba(255, 255, 255, .06)'};
  }

  &.active {
    background: ${props => props.theme.colors.levels.surface};
    color: ${props => props.theme.colors.light};
  }

  &.active:after {
    background-color: ${props => props.theme.colors.brand.accent};
    content: '';
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 4px;
  }

  ${space}
  ${width}
  ${maxWidth}
  ${height}
  ${maxHeight}
`;

TopNavItem.displayName = 'TopNavItem';

export default TopNavItem;
