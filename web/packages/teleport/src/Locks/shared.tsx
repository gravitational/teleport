/*
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

import Table from 'design/DataTable';
import { Spinner } from 'design/Icon';

export const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
    padding: 8px;
  }
  border-radius: 8px;
  overflow: hidden;
` as typeof Table;

export const StyledSpinner = styled(Spinner)`
  cursor: default;
  padding: 8px 0;
  animation: anim-rotate 2s infinite linear;
  @keyframes anim-rotate {
    0% {
      transform: rotate(0);
    }
    100% {
      transform: rotate(360deg);
    }
  }
`;
