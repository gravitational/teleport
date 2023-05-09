/**
Copyright 2023 Gravitational, Inc.

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

export const PickerContainer = styled.div`
  display: flex;
  flex-direction: column;
  position: absolute;
  box-sizing: border-box;
  z-index: 1000;
  font-size: 12px;
  color: ${props => props.theme.colors.text.main};
  background: ${props => props.theme.colors.levels.elevated};
  box-shadow: ${props => props.theme.boxShadow[1]};
  border-radius: ${props => props.theme.radii[2]}px;
  // we have to use a border of the same width as in SearchBar when it is closed to keep
  // the layout from shifting when switching between open and closed state
  border: 1px solid ${props => props.theme.colors.levels.elevated};
  text-shadow: none;
  // Prevents inner items from covering the border on rounded corners.
  overflow: hidden;

  // Account for border.
  width: calc(100% + 2px);
  margin-top: -1px;
`;
