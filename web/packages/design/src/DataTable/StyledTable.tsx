/*
Copyright 2021 Gravitational, Inc.

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

import { space, borderRadius } from 'design/system';
import { darken } from 'design/theme/utils/colorManipulator';

import Icon from '../Icon';

export const StyledTable = styled.table(
  props => `
  background: ${props.theme.colors.levels.surface};
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
  border-collapse: collapse;
  border-spacing: 0;
  font-size: 12px;
  width: 100%;

  & > thead > tr > th,
  & > tbody > tr > th,
  & > tfoot > tr > th,
  & > thead > tr > td,
  & > tbody > tr > td,
  & > tfoot > tr > td {
    padding: 8px 8px;
    vertical-align: middle;

    &:first-child {
      padding-left: 24px;
    }
    &:last-child {
      padding-right: 24px;
    }
  }

  & > tbody > tr > td {
    vertical-align: middle;
  }

  & > thead > tr > th {
    background: ${props.theme.colors.levels.sunkenSecondary};
    color: ${props.theme.colors.text.contrast};
    cursor: pointer;
    font-size: 10px;
    font-weight: 400;
    padding-bottom: 0;
    padding-top: 0;
    text-align: left;
    opacity: 0.75;
    text-transform: uppercase;
    white-space: nowrap;

    ${Icon} {
      font-weight: bold;
      font-size: 8px;
      margin-left: 8px;
    }
  }

  & > tbody > tr > td {
    color: rgba(255, 255, 255, 0.87);
    line-height: 16px;
  }

  tbody tr {
    border-bottom: 1px solid ${props.theme.colors.levels.surfaceSecondary};
  }

  tbody tr:hover {
    background-color: ${darken(props.theme.colors.levels.elevated, 0.14)};
  }

  `,
  space,
  borderRadius
);

export const StyledPanel = styled.nav`
  padding: 16px 24px;
  display: flex;
  height: 24px;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  background: ${props => props.theme.colors.levels.surface};
  ${borderRadius}
`;

export const StyledEmptyIndicator = styled.div(
  props => `
  background: ${props.theme.colors.levels.surfaceSecondary};
  border-radius: 4px;
  box-sizing: border-box;
  margin: 48px auto;
  max-width: 720px;
  padding: 48px 32px;
  text-align: center;

  a {
    color: ${props.theme.colors.link};
  }
`
);
