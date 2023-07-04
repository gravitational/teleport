/**
 * Copyright 2023 Gravitational, Inc.
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
import {Box, Flex} from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Route, Switch } from 'teleport/components/Router';

import { IntegrationTiles } from './IntegrationTiles';
import {
  MachineIDIntegrationSection,
  NoCodeIntegrationDescription
} from './common';
import { getRoutesToEnrollIntegrations } from './IntegrationRoute';

export function IntegrationEnroll() {
  return (
    <FeatureBox>
      <Switch>
        {getRoutesToEnrollIntegrations()}
        <Route
          path={cfg.routes.integrationEnroll}
          component={IntegrationPicker}
        />
      </Switch>
    </FeatureBox>
  );
}

export function IntegrationPicker() {
  return (
    <>
      <FeatureHeader>
        <FeatureHeaderTitle>Select Integration Type</FeatureHeaderTitle>
      </FeatureHeader>
      <Flex flexDirection="column" gap={4}>
        <Flex flexDirection="column">
          <NoCodeIntegrationDescription />
          <IntegrationTiles />
        </Flex>
        <Flex flexDirection="column">
          <MachineIDIntegrationSection />
        </Flex>
      </Flex>
    </>
  );
}
