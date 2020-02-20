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
import Icon from '../Icon';
import { space, borderRadius } from 'design/system';

export const StyledTable = styled.table`
  background: ${props => props.theme.colors.primary.light};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  border-collapse: collapse;
  border-spacing: 0;
  border-radius: 8px;
  font-size: 12px;
  width: 100%;

  & > thead > tr > th,
  & > tbody > tr > th,
  & > tfoot > tr > th,
  & > thead > tr > td,
  & > tbody > tr > td,
  & > tfoot > tr > td {
    padding: 16px;
    vertical-align: middle;
  }

  & > thead > tr > th {
    background: ${props => props.theme.colors.primary.main};
    color: rgba(255, 255, 255, 0.56);
    cursor: pointer;
    font-size: 10px;
    font-weight: 600;
    padding: 4px 16px;
    text-align: left;
    text-transform: uppercase;

    ${Icon} {
      font-weight: bold;
      margin-left: 8px;
    }
  }

  ${space}
  ${borderRadius}
`;

export const StyledEmptyIndicator = styled.div`
  background: ${props => props.theme.colors.primary.main};
  border-radius: 4px;
  box-sizing: border-box;
  margin: 48px auto;
  max-width: 720px;
  padding: 48px 32px;
  text-align: center;

  a {
    color: ${props => props.theme.colors.link};
  }
`;
