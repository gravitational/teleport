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

import { decomposeColor, emphasize } from 'design/theme/utils/colorManipulator';

import Icon from '../Icon';

export const StyledTable = styled.table(
  props => `
  background: ${props.theme.colors.levels.surface};
  border-collapse: collapse;
  border-spacing: 0;
  border-style: hidden;
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
    background: ${props.theme.colors.spotBackground[0]};
    color: ${props.theme.colors.text.main};
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
    color: ${props.theme.colors.text.main};
    line-height: 16px;
  }

  tbody tr {
    border-bottom: 1px solid ${getSolidRowBorderColor(props.theme)};
  }

  tbody tr:hover {
    background-color: ${props.theme.colors.spotBackground[0]};
  }

  `,
  space,
  borderRadius
);

// When `border-collapse: collapse` is set on a table element, Safari incorrectly renders row border with alpha channel.
// It looks like the collapsed border was rendered twice, that is, opacity 0.07 looks like opacity 0.14 (this is more visible
// on the dark theme).
// Sometimes, there is also an artifact visible after hovering the rows - some of them have correct border color, some not.
// WebKit issue https://bugs.webkit.org/show_bug.cgi?id=35456.
//
// `getSolidRowBorderColor` is a workaround. Instead of setting a color with an alpha channel to the border and letting
// the browser mix it with the background color, we calculate the final (non-alpha) color in the JS code.
// The final color is created by lightening or darkening the table background color by the value of the alpha channel of theme.colors.spotBackground[0].
function getSolidRowBorderColor(theme) {
  const alpha = decomposeColor(theme.colors.spotBackground[0]).values[3] || 0;
  return emphasize(theme.colors.levels.surface, alpha);
}

export const StyledPanel = styled.nav<{ showTopBorder: boolean }>`
  padding: 16px 24px;
  display: flex;
  height: 24px;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  background: ${props => props.theme.colors.levels.surface};
  ${borderRadius}
  border-top: ${props =>
    props.showTopBorder
      ? '1px solid ' + props.theme.colors.spotBackground[0]
      : undefined};
`;

export const StyledTableWrapper = styled.div`
  box-shadow: ${props => props.theme.boxShadow[0]};
  overflow: hidden;
  ${borderRadius}
`;
