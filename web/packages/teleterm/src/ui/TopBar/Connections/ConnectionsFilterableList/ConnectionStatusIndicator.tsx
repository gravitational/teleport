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

import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';

export const ConnectionStatusIndicator: React.FC<Props> = props => {
  const { connected, ...styles } = props;
  return <StyledStatus $connected={connected} {...styles} />;
};

const StyledStatus = styled<Props>(Box)`
  width: 8px;
  height: 8px;
  border-radius: 50%;
  ${props => {
    const { $connected, theme } = props;
    const backgroundColor = $connected
      ? theme.colors.success.main
      : theme.colors.grey[300];
    return {
      backgroundColor,
    };
  }}
`;

type Props = {
  connected: boolean;
  [key: string]: any;
};
