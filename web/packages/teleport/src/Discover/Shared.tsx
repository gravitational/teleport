/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Text, ButtonSecondary } from 'design';
import cfg from 'teleport/config';
import history from 'teleport/services/history';

export const Header: React.FC = ({ children }) => (
  <Text mb={4} typography="h4" bold>
    {children}
  </Text>
);

// CancelButton is clicked when user wants to cancel connecting resource
// which at the moment just means to go back to dashboard.
// Later implementation, this could mean go back to main discover
// menu (TBD).
export const CancelButton: React.FC = () => (
  <ButtonSecondary
    mr={3}
    mt={3}
    width="165px"
    onClick={() => history.push(cfg.routes.root, true)}
  >
    Go To Dashboard
  </ButtonSecondary>
);
